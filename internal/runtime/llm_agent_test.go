package runtime

import (
	"context"
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
