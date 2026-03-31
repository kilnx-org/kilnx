package runtime

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
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

// executeFetch performs an HTTP request and returns the result as query rows.
// The response JSON is flattened into a map[string]string for template interpolation.
func executeFetch(node parser.Node, params map[string]string) ([]database.Row, error) {
	// Resolve URL params (:param -> value)
	fetchURL := node.FetchURL
	for k, v := range params {
		fetchURL = strings.ReplaceAll(fetchURL, ":"+k, url.QueryEscape(v))
	}

	// Build request body for POST/PUT/PATCH
	var bodyReader io.Reader
	if len(node.FetchBody) > 0 && node.FetchMethod != "GET" {
		bodyParams := url.Values{}
		for k, v := range node.FetchBody {
			// Resolve :param references
			if strings.HasPrefix(v, ":") {
				paramName := strings.TrimPrefix(v, ":")
				if resolved, ok := params[paramName]; ok {
					v = resolved
				}
			}
			bodyParams.Set(k, v)
		}
		bodyReader = strings.NewReader(bodyParams.Encode())
	}

	req, err := http.NewRequest(node.FetchMethod, fetchURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", fetchURL, err)
	}

	req.Header.Set("User-Agent", "Kilnx/1.0")
	req.Header.Set("Accept", "application/json")
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	// Apply custom headers
	for k, v := range node.FetchHeaders {
		if strings.HasPrefix(v, "env:") {
			envVar := strings.TrimPrefix(v, "env:")
			v = os.Getenv(envVar)
			if v == "" {
				continue
			}
		}
		// Resolve :param references in header values
		for pk, pv := range params {
			v = strings.ReplaceAll(v, ":"+pk, pv)
		}
		req.Header.Set(k, v)
	}

	resp, err := fetchClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s %s: %w", node.FetchMethod, fetchURL, err)
	}
	defer resp.Body.Close()

	// Read response (max 2MB)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("fetch read body: %w", err)
	}

	fmt.Printf("  fetch %s %s -> %d (%d bytes)\n", node.FetchMethod, fetchURL, resp.StatusCode, len(body))

	// Parse JSON response into rows
	return parseJSONResponse(body)
}

// parseJSONResponse converts JSON into database.Row slices for template use.
// Supports: object (single row), array of objects (multiple rows), or wraps primitives.
func parseJSONResponse(body []byte) ([]database.Row, error) {
	body = []byte(strings.TrimSpace(string(body)))
	if len(body) == 0 {
		return nil, nil
	}

	// Try array of objects first
	var arr []map[string]interface{}
	if err := json.Unmarshal(body, &arr); err == nil {
		var rows []database.Row
		for _, obj := range arr {
			rows = append(rows, flattenJSON(obj, ""))
		}
		return rows, nil
	}

	// Try single object
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err == nil {
		return []database.Row{flattenJSON(obj, "")}, nil
	}

	// Fallback: wrap raw response
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
			// Store array length
			row[key+"._count"] = fmt.Sprintf("%d", len(val))
			// Store first few items as indexed keys
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
