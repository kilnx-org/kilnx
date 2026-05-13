package runtime

import (
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

// TestLLMDispatch_AgentMode_MissingAPIKey guards against silent fallthrough
// into the response runtime when the parser hands us an `llm` node whose
// discriminator is `agent`. With ANTHROPIC_API_KEY unset, the agent runtime
// must error out cleanly (not touch llmClient, not panic). The varName binds
// resolve to empty / error strings so downstream HTML interpolation
// degrades gracefully.
func TestLLMDispatch_AgentMode_MissingAPIKey(t *testing.T) {
	os.Unsetenv("ANTHROPIC_API_KEY")

	action := parser.Page{
		Path:   "/agent-task",
		Method: "POST",
		Body: []parser.Node{
			{
				Type:    parser.NodeLLM,
				Name:    "reply",
				LLMMode: "agent",
			},
			{Type: parser.NodeHTML, HTMLContent: `<p>[{reply}]</p>`},
		},
	}
	app := &parser.App{
		Actions: []parser.Page{action},
		Config:  &parser.AppConfig{WorkspaceRoot: t.TempDir()},
	}

	db, err := database.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	s := &Server{app: app, db: db}
	s.sessions = NewSessionStore("test-secret")
	s.sessions.SetDB(db)
	s.logger = NewLogger(nil)
	s.i18n = NewI18n(nil, "en", false)

	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/agent-task", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	s.handleAction(rec, req, action, s.getApp())

	if rec.Code == 403 {
		t.Fatalf("CSRF rejected: %s", rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "[]") {
		t.Errorf("expected empty agent reply on missing API key, got body=%q", body)
	}
}

// TestLLMDispatch_AgentMode_HappyBindings exercises the agent dispatch arm
// end to end with a fake claude binary, asserting all five binds
// (:reply, :reply.session_id, :reply.cost_usd, :reply.duration_ms,
// :reply.stop_reason) land on formData and surface through interpolation.
func TestLLMDispatch_AgentMode_HappyBindings(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	stubDir := t.TempDir()
	stub := stubDir + "/claude"
	fixture, _ := filepath.Abs("testdata/agent_streams/happy.jsonl")
	script := "#!/bin/sh\nexec cat " + fixture + "\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	t.Setenv("KILNX_CLAUDE_BIN", stub)

	action := parser.Page{
		Path:   "/agent-task",
		Method: "POST",
		Body: []parser.Node{
			{
				Type:           parser.NodeLLM,
				Name:           "reply",
				LLMMode:        "agent",
				LLMAgentBudget: 0.1,
			},
			{
				Type: parser.NodeHTML,
				HTMLContent: `<p>text=[{_form.reply}]</p>` +
					`<p>sid=[{_form.reply.session_id}]</p>` +
					`<p>cost=[{_form.reply.cost_usd}]</p>` +
					`<p>dur=[{_form.reply.duration_ms}]</p>` +
					`<p>stop=[{_form.reply.stop_reason}]</p>`,
			},
		},
	}
	app := &parser.App{
		Actions: []parser.Page{action},
		Config:  &parser.AppConfig{WorkspaceRoot: t.TempDir()},
	}
	db, err := database.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	s := &Server{app: app, db: db}
	s.sessions = NewSessionStore("test-secret")
	s.sessions.SetDB(db)
	s.logger = NewLogger(nil)
	s.i18n = NewI18n(nil, "en", false)

	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/agent-task", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	s.handleAction(rec, req, action, s.getApp())

	if rec.Code == 403 {
		t.Fatalf("CSRF rejected: %s", rec.Body.String())
	}
	body := rec.Body.String()
	checks := []string{
		"text=[Hello world]",
		"sid=[11111111-2222-4333-8444-555555555555]",
		"cost=[0.002300]",
		"dur=[1234]",
		"stop=[end_turn]",
	}
	for _, want := range checks {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q\nbody=%s", want, body)
		}
	}
}

// TestLLMDispatch_UnknownModePlaceholder asserts the default arm rejects nodes
// with empty or unknown LLMMode rather than silently calling executeLLM.
func TestLLMDispatch_UnknownModePlaceholder(t *testing.T) {
	os.Unsetenv("ANTHROPIC_API_KEY")

	action := parser.Page{
		Path:   "/bad-llm",
		Method: "POST",
		Body: []parser.Node{
			{
				Type: parser.NodeLLM,
				Name: "reply",
				// LLMMode intentionally empty - simulates a malformed node
			},
			{Type: parser.NodeHTML, HTMLContent: `<p>{reply}</p>`},
		},
	}
	app := &parser.App{Actions: []parser.Page{action}}

	db, err := database.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	s := &Server{app: app, db: db}
	s.sessions = NewSessionStore("test-secret")
	s.sessions.SetDB(db)
	s.logger = NewLogger(nil)
	s.i18n = NewI18n(nil, "en", false)

	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/bad-llm", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	s.handleAction(rec, req, action, s.getApp())

	if rec.Code == 403 {
		t.Fatalf("CSRF rejected: %s", rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Configuração inválida do bloco llm.") {
		t.Errorf("expected unknown-mode placeholder, got body=%q", body)
	}
}
