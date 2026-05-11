package parser

import "testing"

func TestLLMAgent_OnProgressBlock(t *testing.T) {
	src := `job run-review
  llm review
    model: claude-sonnet-4-6
    agent
      cwd: /tmp/x
      max-budget-usd: 1.0
      on progress
        query: UPDATE review_job SET progress = :progress WHERE id = :job_id
`
	app := parse(t, src)
	if len(app.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(app.Jobs))
	}
	body := app.Jobs[0].Body
	if len(body) != 1 || body[0].Type != NodeLLM {
		t.Fatalf("expected 1 NodeLLM in job body, got %+v", body)
	}
	n := body[0]
	if n.LLMMode != "agent" {
		t.Fatalf("mode: want agent, got %q", n.LLMMode)
	}
	if len(n.LLMAgentProgressBody) != 1 {
		t.Fatalf("expected 1 node in progress body, got %d", len(n.LLMAgentProgressBody))
	}
	if n.LLMAgentProgressBody[0].Type != NodeQuery {
		t.Errorf("progress body node type: want NodeQuery, got %d", n.LLMAgentProgressBody[0].Type)
	}
	if n.LLMAgentBudget != 1.0 {
		t.Errorf("budget: want 1.0, got %v", n.LLMAgentBudget)
	}
}
