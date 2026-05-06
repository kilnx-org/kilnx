package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

var fetchClient = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

var jsonNumberRe = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)

// executeFetch performs an HTTP request and returns the response rows plus
// the HTTP status code. A non-nil error indicates a transport-level failure
// (DNS, connection, timeout, body read) and should propagate to the caller
// so the surrounding action can roll back. HTTP 4xx/5xx are NOT treated as
// errors: the response body is parsed and the status code is returned so
// the caller may bind `<name>.status_code` / `<name>.ok` and let the user
// branch with `on`.
func executeFetch(node parser.Node, params map[string]string) ([]database.Row, int, error) {
	fetchURL := node.FetchURL
	for k, v := range params {
		fetchURL = strings.ReplaceAll(fetchURL, ":"+k, url.QueryEscape(v))
	}

	wantJSON := bodyShouldBeJSON(node.FetchHeaders)
	bodyReader, contentType, err := buildRequestBody(node, params, wantJSON)
	if err != nil {
		return nil, 0, fmt.Errorf("fetch %s: %w", redactURL(fetchURL), err)
	}

	req, err := http.NewRequest(node.FetchMethod, fetchURL, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("fetch %s: %w", redactURL(fetchURL), err)
	}

	req.Header.Set("User-Agent", "Kilnx/1.0")
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	for k, v := range node.FetchHeaders {
		if strings.HasPrefix(v, "env:") {
			envVar := strings.TrimPrefix(v, "env:")
			v = os.Getenv(envVar)
			if v == "" {
				continue
			}
		}
		for pk, pv := range params {
			v = strings.ReplaceAll(v, ":"+pk, pv)
		}
		req.Header.Set(k, v)
	}

	resp, err := fetchClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("fetch %s %s: %w", node.FetchMethod, redactURL(fetchURL), err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("fetch read body: %w", err)
	}

	fmt.Printf("  fetch %s %s -> %d (%d bytes)\n", node.FetchMethod, redactURL(fetchURL), resp.StatusCode, len(body))

	rows, err := parseJSONResponse(body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return rows, resp.StatusCode, nil
}

// bodyShouldBeJSON returns true if any user-supplied header has Content-Type
// set to a JSON media type (case-insensitive).
func bodyShouldBeJSON(headers map[string]string) bool {
	for k, v := range headers {
		if strings.EqualFold(k, "Content-Type") && strings.Contains(strings.ToLower(v), "application/json") {
			return true
		}
	}
	return false
}

// buildRequestBody resolves :param references in body values and encodes the
// result either as application/json (when wantJSON) or
// application/x-www-form-urlencoded. Returns an empty body for GET requests.
func buildRequestBody(node parser.Node, params map[string]string, wantJSON bool) (io.Reader, string, error) {
	if len(node.FetchBody) == 0 || node.FetchMethod == "GET" {
		return nil, "", nil
	}

	resolved := make(map[string]string, len(node.FetchBody))
	for k, v := range node.FetchBody {
		if strings.HasPrefix(v, ":") {
			paramName := strings.TrimPrefix(v, ":")
			if got, ok := params[paramName]; ok {
				v = got
			}
		}
		resolved[k] = v
	}

	if wantJSON {
		obj := make(map[string]any, len(resolved))
		for k, v := range resolved {
			obj[k] = jsonCoerce(v)
		}
		buf, err := json.Marshal(obj)
		if err != nil {
			return nil, "", fmt.Errorf("encode json body: %w", err)
		}
		return bytes.NewReader(buf), "application/json", nil
	}

	form := url.Values{}
	for k, v := range resolved {
		form.Set(k, v)
	}
	return strings.NewReader(form.Encode()), "application/x-www-form-urlencoded", nil
}

// jsonCoerce tries to emit numbers/bools/null as native JSON types when the
// value is unambiguous. Anything else stays a string. This lets users pass
// `body amount: :total` to APIs (Stripe etc.) that require typed numbers.
func jsonCoerce(v string) any {
	switch v {
	case "true":
		return true
	case "false":
		return false
	case "null":
		return nil
	}
	if jsonNumberRe.MatchString(v) {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return v
}

// redactURL strips the query string so secrets passed via :param substitution
// do not end up in stdout/log lines.
func redactURL(raw string) string {
	if i := strings.Index(raw, "?"); i >= 0 {
		return raw[:i] + "?<redacted>"
	}
	return raw
}

// bindFetchResult populates params with `<name>.<key>` entries from the first
// row of the response, plus `<name>.status_code` and `<name>.ok` so users can
// branch with `on <name>.ok` / `on <name>.status_code`.
func bindFetchResult(params map[string]string, name string, rows []database.Row, status int) {
	if params == nil {
		return
	}
	if len(rows) > 0 {
		for k, v := range rows[0] {
			params[name+"."+k] = v
		}
	}
	params[name+".status_code"] = strconv.Itoa(status)
	params[name+".ok"] = boolStr(status >= 200 && status < 300)
}

// annotateFetchRows attaches status_code/ok to the first row so the result is
// usable through renderContext.queries (page render path).
func annotateFetchRows(rows []database.Row, status int) []database.Row {
	if len(rows) == 0 {
		rows = []database.Row{{}}
	}
	rows[0]["status_code"] = strconv.Itoa(status)
	rows[0]["ok"] = boolStr(status >= 200 && status < 300)
	return rows
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// parseJSONResponse converts JSON into database.Row slices for template use.
// Supports: object (single row), array of objects (multiple rows), or wraps primitives.
func parseJSONResponse(body []byte) ([]database.Row, error) {
	body = []byte(strings.TrimSpace(string(body)))
	if len(body) == 0 {
		return nil, nil
	}

	var arr []map[string]interface{}
	if err := json.Unmarshal(body, &arr); err == nil {
		var rows []database.Row
		for _, obj := range arr {
			rows = append(rows, flattenJSON(obj, ""))
		}
		return rows, nil
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err == nil {
		return []database.Row{flattenJSON(obj, "")}, nil
	}

	return []database.Row{{"_body": string(body)}}, nil
}

// flattenJSON converts a nested JSON object into a flat map[string]string.
// Nested keys are joined with dots: {"user": {"name": "Alice"}} -> {"user.name": "Alice"}
func flattenJSON(obj map[string]interface{}, prefix string) database.Row {
	row := make(database.Row)
	for k, v := range obj {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]interface{}:
			for fk, fv := range flattenJSON(val, key) {
				row[fk] = fv
			}
		case []interface{}:
			row[key+"._count"] = fmt.Sprintf("%d", len(val))
			for i, item := range val {
				if i >= 10 {
					break
				}
				if m, ok := item.(map[string]interface{}); ok {
					for fk, fv := range flattenJSON(m, fmt.Sprintf("%s.%d", key, i)) {
						row[fk] = fv
					}
				} else {
					row[fmt.Sprintf("%s.%d", key, i)] = fmt.Sprintf("%v", item)
				}
			}
		case nil:
			row[key] = ""
		case float64:
			if val == float64(int64(val)) {
				row[key] = fmt.Sprintf("%d", int64(val))
			} else {
				row[key] = fmt.Sprintf("%g", val)
			}
		case bool:
			if val {
				row[key] = "true"
			} else {
				row[key] = "false"
			}
		default:
			row[key] = fmt.Sprintf("%v", val)
		}
	}
	return row
}
