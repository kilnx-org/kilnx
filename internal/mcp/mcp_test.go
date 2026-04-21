package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// roundTrip writes requests to the server via a pipe and collects responses.
func roundTrip(t *testing.T, requests []map[string]any) []map[string]any {
	t.Helper()

	var input bytes.Buffer
	for _, req := range requests {
		b, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		input.Write(b)
		input.WriteByte('\n')
	}

	var output bytes.Buffer
	origStdin := stdin
	origStdout := stdout
	stdin = bufio.NewReader(&input)
	stdout = &output
	defer func() {
		stdin = origStdin
		stdout = origStdout
	}()

	Serve()

	var responses []map[string]any
	scanner := bufio.NewScanner(&output)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var resp map[string]any
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("unmarshal response %q: %v", line, err)
		}
		responses = append(responses, resp)
	}
	return responses
}

func initMsg(id int) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "1"},
		},
	}
}

func TestTransportFraming(t *testing.T) {
	resps := roundTrip(t, []map[string]any{initMsg(1)})
	if len(resps) != 1 {
		t.Fatalf("want 1 response, got %d", len(resps))
	}
	if resps[0]["jsonrpc"] != "2.0" {
		t.Errorf("want jsonrpc=2.0, got %v", resps[0]["jsonrpc"])
	}
	result, _ := resps[0]["result"].(map[string]any)
	if result["protocolVersion"] != protocolVersion {
		t.Errorf("want protocolVersion=%s, got %v", protocolVersion, result["protocolVersion"])
	}
}

func TestNotificationSkipped(t *testing.T) {
	// Notifications have no id — server must not respond.
	resps := roundTrip(t, []map[string]any{
		initMsg(1),
		{"jsonrpc": "2.0", "method": "notifications/initialized"},
	})
	if len(resps) != 1 {
		t.Errorf("want 1 response (notification skipped), got %d", len(resps))
	}
}

func TestResourcesList(t *testing.T) {
	resps := roundTrip(t, []map[string]any{
		initMsg(1),
		{"jsonrpc": "2.0", "id": 2, "method": "resources/list", "params": map[string]any{}},
	})
	if len(resps) != 2 {
		t.Fatalf("want 2 responses, got %d", len(resps))
	}
	result, _ := resps[1]["result"].(map[string]any)
	resources, _ := result["resources"].([]any)
	if len(resources) == 0 {
		t.Error("want at least one resource")
	}
}

func TestResourcesReadUnknownURI(t *testing.T) {
	resps := roundTrip(t, []map[string]any{
		initMsg(1),
		{"jsonrpc": "2.0", "id": 2, "method": "resources/read", "params": map[string]any{"uri": "kilnx://nonexistent"}},
	})
	if len(resps) != 2 {
		t.Fatalf("want 2 responses, got %d", len(resps))
	}
	if resps[1]["error"] == nil {
		t.Error("want error for unknown URI, got nil")
	}
}

func TestResourcesReadKnownURIs(t *testing.T) {
	uris := []string{"kilnx://quickref", "kilnx://keywords", "kilnx://grammar-summary", "kilnx://examples"}
	reqs := []map[string]any{initMsg(1)}
	for i, uri := range uris {
		reqs = append(reqs, map[string]any{
			"jsonrpc": "2.0",
			"id":      i + 2,
			"method":  "resources/read",
			"params":  map[string]any{"uri": uri},
		})
	}
	resps := roundTrip(t, reqs)
	if len(resps) != len(reqs) {
		t.Fatalf("want %d responses, got %d", len(reqs), len(resps))
	}
	for i, uri := range uris {
		resp := resps[i+1]
		if resp["error"] != nil {
			t.Errorf("URI %s returned error: %v", uri, resp["error"])
		}
		result, _ := resp["result"].(map[string]any)
		contents, _ := result["contents"].([]any)
		if len(contents) == 0 {
			t.Errorf("URI %s returned empty contents", uri)
		}
	}
}

func TestToolsList(t *testing.T) {
	resps := roundTrip(t, []map[string]any{
		initMsg(1),
		{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]any{}},
	})
	if len(resps) != 2 {
		t.Fatalf("want 2 responses, got %d", len(resps))
	}
	result, _ := resps[1]["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	if len(tools) == 0 {
		t.Error("want at least one tool")
	}
}

func TestToolsCallUnknown(t *testing.T) {
	resps := roundTrip(t, []map[string]any{
		initMsg(1),
		{"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": map[string]any{"name": "nonexistent", "arguments": map[string]any{}}},
	})
	if len(resps) != 2 {
		t.Fatalf("want 2 responses, got %d", len(resps))
	}
	result, _ := resps[1]["result"].(map[string]any)
	if result["isError"] != true {
		t.Error("want isError=true for unknown tool")
	}
}

func TestToolsCallKeywordInfo(t *testing.T) {
	resps := roundTrip(t, []map[string]any{
		initMsg(1),
		{"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": map[string]any{"name": "keyword_info", "arguments": map[string]any{"keyword": "page"}}},
	})
	if len(resps) != 2 {
		t.Fatalf("want 2 responses, got %d", len(resps))
	}
	result, _ := resps[1]["result"].(map[string]any)
	if result["isError"] == true {
		t.Error("want isError=false for known keyword")
	}
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatal("want non-empty content")
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "page") {
		t.Errorf("want 'page' in response, got: %s", text)
	}
}

func TestShutdown(t *testing.T) {
	resps := roundTrip(t, []map[string]any{
		initMsg(1),
		{"jsonrpc": "2.0", "id": 2, "method": "shutdown", "params": map[string]any{}},
	})
	if len(resps) != 2 {
		t.Fatalf("want 2 responses, got %d", len(resps))
	}
	if resps[1]["error"] != nil {
		t.Errorf("shutdown returned error: %v", resps[1]["error"])
	}
}

func TestBuildKeywordRefDeterministic(t *testing.T) {
	first := buildKeywordRef()
	for i := 0; i < 10; i++ {
		if got := buildKeywordRef(); got != first {
			t.Errorf("buildKeywordRef() is non-deterministic (iteration %d differs)", i+1)
		}
	}
}

func TestExtractEnvVars(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   []string
	}{
		{"empty", "", nil},
		{"no env", "config\n  port: 8080\n", nil},
		{"single", "config\n  secret: env SECRET required\n", []string{"SECRET"}},
		{"multiple", "config\n  database: env DB default \"sqlite://app.db\"\n  secret: env SECRET required\n", []string{"DB", "SECRET"}},
		{"dedup", "config\n  secret: env SECRET required\n  secret: env SECRET required\n", []string{"SECRET"}},
		{"comment ignored", "# env COMMENT_VAR\nconfig\n  secret: env REAL required\n", []string{"REAL"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractEnvVars(tc.source)
			if len(got) != len(tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
				return
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("got[%d]=%s, want %s", i, got[i], tc.want[i])
				}
			}
		})
	}
}
