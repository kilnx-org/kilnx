package parser

import "testing"

func TestLLMResponseMode(t *testing.T) {
	src := `action /chat method POST
  llm reply
    model: claude-sonnet-4-6
    system: You are helpful
    temperature: 0.7
    max-tokens: 2048
    response
      history: SELECT papel, conteudo FROM mensagem WHERE conversa_id = :id ORDER BY criada
      stream: #chat-msgs
      stream-swap: append
`
	app := parse(t, src)
	if len(app.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(app.Actions))
	}
	a := app.Actions[0]
	if len(a.Body) != 1 {
		t.Fatalf("expected 1 body node, got %d", len(a.Body))
	}
	n := a.Body[0]
	if n.Type != NodeLLM {
		t.Fatalf("expected NodeLLM, got %d", n.Type)
	}
	if n.Name != "reply" {
		t.Errorf("name: want %q, got %q", "reply", n.Name)
	}
	if n.LLMModel != "claude-sonnet-4-6" {
		t.Errorf("model: want %q, got %q", "claude-sonnet-4-6", n.LLMModel)
	}
	if n.LLMSystem != "You are helpful" {
		t.Errorf("system: want %q, got %q", "You are helpful", n.LLMSystem)
	}
	if n.LLMTemperature != 0.7 {
		t.Errorf("temperature: want 0.7, got %v", n.LLMTemperature)
	}
	if n.LLMMaxTokens != 2048 {
		t.Errorf("max-tokens: want 2048, got %d", n.LLMMaxTokens)
	}
	if n.LLMMode != "response" {
		t.Errorf("mode: want %q, got %q", "response", n.LLMMode)
	}
	if n.LLMHistorySQL == "" {
		t.Errorf("history SQL not captured")
	}
	if n.LLMStreamTarget != "#chat-msgs" {
		t.Errorf("stream target: want %q, got %q", "#chat-msgs", n.LLMStreamTarget)
	}
	if n.LLMStreamSwap != "append" {
		t.Errorf("stream-swap: want %q, got %q", "append", n.LLMStreamSwap)
	}
}

func TestLLMAgentMode(t *testing.T) {
	src := `job process-doc
  llm task
    model: claude-sonnet-4-6
    system: Code assistant
    agent
      cwd: /workspaces/jobs/:doc_id
      tools: read, write, bash
      max-turns: 10
      max-budget-usd: 0.50
      permission-mode: plan
      mcp: stripe, github
      pool: 4
      pool-idle-ttl: 5m
`
	app := parse(t, src)
	if len(app.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(app.Jobs))
	}
	body := app.Jobs[0].Body
	if len(body) != 1 {
		t.Fatalf("expected 1 body node, got %d", len(body))
	}
	n := body[0]
	if n.Type != NodeLLM {
		t.Fatalf("expected NodeLLM, got %d", n.Type)
	}
	if n.Name != "task" {
		t.Errorf("name: want %q, got %q", "task", n.Name)
	}
	if n.LLMMode != "agent" {
		t.Errorf("mode: want %q, got %q", "agent", n.LLMMode)
	}
	if n.LLMAgentCwd != "/workspaces/jobs/:doc_id" {
		t.Errorf("cwd: want %q, got %q", "/workspaces/jobs/:doc_id", n.LLMAgentCwd)
	}
	wantTools := []string{"read", "write", "bash"}
	if len(n.LLMAgentTools) != len(wantTools) {
		t.Fatalf("tools len: want %d, got %d", len(wantTools), len(n.LLMAgentTools))
	}
	for i, w := range wantTools {
		if n.LLMAgentTools[i] != w {
			t.Errorf("tools[%d]: want %q, got %q", i, w, n.LLMAgentTools[i])
		}
	}
	if n.LLMAgentMaxTurns != 10 {
		t.Errorf("max-turns: want 10, got %d", n.LLMAgentMaxTurns)
	}
	if n.LLMAgentBudget != 0.50 {
		t.Errorf("budget: want 0.50, got %v", n.LLMAgentBudget)
	}
	if n.LLMAgentPermissionMode != "plan" {
		t.Errorf("permission-mode: want %q, got %q", "plan", n.LLMAgentPermissionMode)
	}
	wantMCP := []string{"stripe", "github"}
	if len(n.LLMAgentMCP) != len(wantMCP) {
		t.Fatalf("mcp len: want %d, got %d", len(wantMCP), len(n.LLMAgentMCP))
	}
	if n.LLMAgentPool != 4 {
		t.Errorf("pool: want 4, got %d", n.LLMAgentPool)
	}
	if n.LLMAgentPoolIdleTTL != "5m" {
		t.Errorf("pool-idle-ttl: want %q, got %q", "5m", n.LLMAgentPoolIdleTTL)
	}
}

func TestLLMOldShapeRejected(t *testing.T) {
	src := `action /chat method POST
  llm reply: claude-sonnet-4-6
    history: SELECT 1
    system: You are helpful
`
	app, err := parseAllowErrors(t, src)
	if err != nil {
		// Either error or empty model/mode is acceptable as a v0.2 rejection.
		return
	}
	if len(app.Actions) == 0 {
		return
	}
	body := app.Actions[0].Body
	for _, n := range body {
		if n.Type != NodeLLM {
			continue
		}
		// Old shape inline model after colon should NOT populate LLMModel
		// (only the child `model:` line should).
		if n.LLMModel == "claude-sonnet-4-6" {
			t.Errorf("v0.2 parser must not accept inline model after colon; got LLMModel=%q", n.LLMModel)
		}
		// No mode declared means response/agent missing — analyzer would catch.
		if n.LLMMode != "" {
			t.Errorf("expected empty LLMMode when no response/agent block; got %q", n.LLMMode)
		}
	}
}
