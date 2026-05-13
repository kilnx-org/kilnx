package runtime

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// fakeClaudeBin writes a shell stub at <dir>/claude that copies the
// fixture jsonl file to stdout. It returns the directory holding the
// stub, which callers wire into KILNX_CLAUDE_BIN. We deliberately do
// NOT override PATH because exec.LookPath would still cache the real
// binary; the runtime consults KILNX_CLAUDE_BIN directly.
func fakeClaudeBin(t *testing.T, fixture string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake claude bin uses /bin/sh; skipping on windows")
	}
	dir := t.TempDir()
	stub := filepath.Join(dir, "claude")
	abs, err := filepath.Abs(fixture)
	if err != nil {
		t.Fatalf("abs fixture: %v", err)
	}
	script := "#!/bin/sh\nexec cat " + abs + "\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	return stub
}

func TestExecuteLLMAgent_HappyPath(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("KILNX_CLAUDE_BIN", fakeClaudeBin(t, "testdata/agent_streams/happy.jsonl"))

	app := &parser.App{Config: &parser.AppConfig{WorkspaceRoot: t.TempDir()}}
	node := parser.Node{
		Type:           parser.NodeLLM,
		Name:           "task",
		LLMMode:        "agent",
		LLMModel:       "claude-sonnet-4-6",
		LLMAgentBudget: 0.10,
		LLMAgentTools:  []string{"read", "write"},
	}

	res, err := executeLLMAgent(context.Background(), node, app, map[string]string{"prompt": "say hi"}, nil)
	if err != nil {
		t.Fatalf("agent run: %v", err)
	}
	if res.Text != "Hello world" {
		t.Errorf("text = %q, want %q", res.Text, "Hello world")
	}
	if res.SessionID != "11111111-2222-4333-8444-555555555555" {
		t.Errorf("session id = %q", res.SessionID)
	}
	if res.CostUSD != 0.0023 {
		t.Errorf("cost = %v", res.CostUSD)
	}
	if res.DurationMS != 1234 {
		t.Errorf("duration = %v", res.DurationMS)
	}
	if res.StopReason != "end_turn" {
		t.Errorf("stop reason = %q", res.StopReason)
	}
}

func TestExecuteLLMAgent_BudgetHit(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("KILNX_CLAUDE_BIN", fakeClaudeBin(t, "testdata/agent_streams/budget_hit.jsonl"))

	app := &parser.App{Config: &parser.AppConfig{WorkspaceRoot: t.TempDir()}}
	node := parser.Node{
		Type:           parser.NodeLLM,
		Name:           "task",
		LLMMode:        "agent",
		LLMAgentBudget: 0.0001,
	}
	res, err := executeLLMAgent(context.Background(), node, app, nil, nil)
	if err != nil {
		t.Fatalf("agent run: %v", err)
	}
	if res.StopReason != "exceeded_max_budget" {
		t.Errorf("stop reason = %q, want exceeded_max_budget", res.StopReason)
	}
	if !strings.Contains(res.Text, "Partial") {
		t.Errorf("text = %q, want to contain Partial", res.Text)
	}
}

func TestExecuteLLMAgent_MissingAPIKey(t *testing.T) {
	os.Unsetenv("ANTHROPIC_API_KEY")
	app := &parser.App{Config: &parser.AppConfig{WorkspaceRoot: t.TempDir()}}
	node := parser.Node{Type: parser.NodeLLM, LLMMode: "agent", LLMAgentBudget: 0.1}
	_, err := executeLLMAgent(context.Background(), node, app, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Fatalf("expected api-key error, got %v", err)
	}
}

func TestExecuteLLMAgent_BadResumeUUID(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("KILNX_CLAUDE_BIN", fakeClaudeBin(t, "testdata/agent_streams/happy.jsonl"))

	app := &parser.App{Config: &parser.AppConfig{WorkspaceRoot: t.TempDir()}}
	node := parser.Node{
		Type:           parser.NodeLLM,
		LLMMode:        "agent",
		LLMAgentBudget: 0.1,
		LLMAgentResume: "not-a-uuid",
	}
	_, err := executeLLMAgent(context.Background(), node, app, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "valid UUID") {
		t.Fatalf("expected uuid error, got %v", err)
	}
}

func TestExecuteLLMAgent_CwdEscape(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("KILNX_CLAUDE_BIN", fakeClaudeBin(t, "testdata/agent_streams/happy.jsonl"))

	root := t.TempDir()
	app := &parser.App{Config: &parser.AppConfig{WorkspaceRoot: root}}
	node := parser.Node{
		Type:           parser.NodeLLM,
		LLMMode:        "agent",
		LLMAgentBudget: 0.1,
		LLMAgentCwd:    "/etc",
	}
	_, err := executeLLMAgent(context.Background(), node, app, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "escapes workspace-root") {
		t.Fatalf("expected escape error, got %v", err)
	}
}

func TestExecuteLLMAgent_StreamingHyperstream(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("KILNX_CLAUDE_BIN", fakeClaudeBin(t, "testdata/agent_streams/streaming.jsonl"))

	app := &parser.App{Config: &parser.AppConfig{WorkspaceRoot: t.TempDir()}}
	node := parser.Node{
		Type:             parser.NodeLLM,
		Name:             "task",
		LLMMode:          "agent",
		LLMAgentBudget:   0.1,
		LLMStreamTarget:  "#out",
		LLMStreamSwap:    "append",
	}
	rec := httptest.NewRecorder()
	res, err := executeLLMAgent(context.Background(), node, app, map[string]string{"prompt": "hi"}, rec)
	if err != nil {
		t.Fatalf("agent run: %v", err)
	}
	if res.Text != "chunk-one chunk-two" {
		t.Errorf("text = %q", res.Text)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("content-type = %q", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<hs-partial") {
		t.Errorf("body missing hs-partial envelope: %q", body)
	}
	if !strings.Contains(body, "chunk-") {
		t.Errorf("body missing streamed text: %q", body)
	}
	if !strings.Contains(body, `final="true"`) {
		t.Errorf("body missing final envelope: %q", body)
	}
}

func TestExecuteLLMAgent_MaxTurnsExceeded(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("KILNX_CLAUDE_BIN", fakeClaudeBin(t, "testdata/agent_streams/max_turns.jsonl"))

	app := &parser.App{Config: &parser.AppConfig{WorkspaceRoot: t.TempDir()}}
	node := parser.Node{
		Type:             parser.NodeLLM,
		LLMMode:          "agent",
		LLMAgentBudget:   0.1,
		LLMAgentMaxTurns: 2,
	}
	_, err := executeLLMAgent(context.Background(), node, app, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "max turns") {
		t.Fatalf("expected max turns error, got %v", err)
	}
}

func TestExecuteLLMAgent_ShowToolsChannel(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("KILNX_CLAUDE_BIN", fakeClaudeBin(t, "testdata/agent_streams/tools.jsonl"))

	app := &parser.App{Config: &parser.AppConfig{WorkspaceRoot: t.TempDir()}}
	node := parser.Node{
		Type:              parser.NodeLLM,
		Name:              "task",
		LLMMode:           "agent",
		LLMAgentBudget:    0.1,
		LLMStreamTarget:   "#out",
		LLMAgentShowTools: true,
	}
	rec := httptest.NewRecorder()
	_, err := executeLLMAgent(context.Background(), node, app, nil, rec)
	if err != nil {
		t.Fatalf("agent run: %v", err)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "tool_use:Read") {
		t.Errorf("body missing tool_use payload: %q", body)
	}
	if !strings.Contains(body, "tool_result") {
		t.Errorf("body missing tool_result payload: %q", body)
	}
}

func TestWriteMCPConfig_StdioHappy(t *testing.T) {
	app := &parser.App{
		MCPServers: []parser.MCPServer{{
			Name:    "filesystem",
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp/x"},
			Env:     map[string]string{"FOO": "bar"},
		}},
	}
	node := parser.Node{LLMAgentMCP: []string{"filesystem"}}
	path, cleanup, err := writeMCPConfig(node, app)
	if err != nil {
		t.Fatalf("writeMCPConfig: %v", err)
	}
	defer cleanup()
	if path == "" {
		t.Fatal("empty path")
	}
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mcp config: %v", err)
	}
	var doc struct {
		MCPServers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(buf, &doc); err != nil {
		t.Fatalf("unmarshal: %v (raw=%s)", err, string(buf))
	}
	srv, ok := doc.MCPServers["filesystem"]
	if !ok {
		t.Fatalf("filesystem missing in %s", string(buf))
	}
	if srv.Command != "npx" {
		t.Errorf("command = %q", srv.Command)
	}
	if len(srv.Args) != 3 || srv.Args[0] != "-y" {
		t.Errorf("args = %v", srv.Args)
	}
	if srv.Env["FOO"] != "bar" {
		t.Errorf("env = %v", srv.Env)
	}
}

func TestWriteMCPConfig_HTTPTransport(t *testing.T) {
	app := &parser.App{
		MCPServers: []parser.MCPServer{{
			Name:      "remote",
			URL:       "https://mcp.example.com/sse",
			Transport: "sse",
		}},
	}
	node := parser.Node{LLMAgentMCP: []string{"remote"}}
	path, cleanup, err := writeMCPConfig(node, app)
	if err != nil {
		t.Fatalf("writeMCPConfig: %v", err)
	}
	defer cleanup()
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s := string(buf)
	if !strings.Contains(s, `"url":"https://mcp.example.com/sse"`) {
		t.Errorf("missing url: %s", s)
	}
	if !strings.Contains(s, `"type":"sse"`) {
		t.Errorf("missing type: %s", s)
	}
}

func TestResolveAgentCwd_DeclaredInsideRoot(t *testing.T) {
	root := t.TempDir()
	app := &parser.App{Config: &parser.AppConfig{WorkspaceRoot: root}}
	target := filepath.Join(root, "project-x")
	node := parser.Node{LLMAgentCwd: target}
	got, cleanup, err := resolveAgentCwd(node, app, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	defer cleanup()
	realTarget, _ := filepath.EvalSymlinks(target)
	if got != realTarget {
		t.Errorf("got %q want %q", got, realTarget)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("declared cwd not created: %v", err)
	}
}

func TestExecuteLLMAgent_MCPUnknownServer(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("KILNX_CLAUDE_BIN", fakeClaudeBin(t, "testdata/agent_streams/happy.jsonl"))

	app := &parser.App{Config: &parser.AppConfig{WorkspaceRoot: t.TempDir()}}
	node := parser.Node{
		Type:           parser.NodeLLM,
		LLMMode:        "agent",
		LLMAgentBudget: 0.1,
		LLMAgentMCP:    []string{"ghost"},
	}
	_, err := executeLLMAgent(context.Background(), node, app, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "not declared") {
		t.Fatalf("expected mcp not-declared error, got %v", err)
	}
}
