package runtime

import (
	"net/http/httptest"
	"net/url"
	"os"
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
