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

// TestLLMDispatch_AgentModePlaceholder guards against silent fallthrough into
// the response runtime (Messages API) when the parser hands us an `llm` node
// whose discriminator is `agent`. P3 will wire the real agent runtime; until
// then, dispatch must short-circuit to a placeholder and never touch llmClient.
//
// Defense in depth: parser currently rejects malformed shapes, but a future
// refactor or hand-built node must not regress into hitting Anthropic.
func TestLLMDispatch_AgentModePlaceholder(t *testing.T) {
	os.Unsetenv("ANTHROPIC_API_KEY")

	action := parser.Page{
		Path:   "/agent-task",
		Method: "POST",
		Body: []parser.Node{
			{
				Type:        parser.NodeLLM,
				Name:        "reply",
				LLMMode:     "agent",
				LLMAgentCwd: "/tmp/jobs/:id",
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
	req := httptest.NewRequest("POST", "/agent-task", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	s.handleAction(rec, req, action, s.getApp())

	if rec.Code == 403 {
		t.Fatalf("CSRF rejected: %s", rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Modo agent ainda não implementado.") {
		t.Errorf("expected agent placeholder, got body=%q", body)
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
