package runtime

import (
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

// TestActionAttributeEndToEndCSRF guards Issue 1 (BLOCKER): a button with
// action="/foo" must end up with a CSRF token its POST can satisfy. Render
// the page, pull the token out of hx-vals, then post it back through
// handleAction. The request must NOT be rejected with 403.
func TestActionAttributeEndToEndCSRF(t *testing.T) {
	db, err := database.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.Conn().Exec("CREATE TABLE task (id INTEGER PRIMARY KEY, title TEXT)")

	action := parser.Page{
		Path:   "/tasks/create",
		Method: "POST",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: "INSERT INTO task (title) VALUES ('hello')"},
		},
	}
	app := &parser.App{
		Actions: []parser.Page{action},
	}

	s := &Server{app: app, db: db}
	s.sessions = NewSessionStore("test-secret")
	s.sessions.SetDB(db)
	s.logger = NewLogger(nil)
	s.i18n = NewI18n(nil, "en", false)

	// Render a page snippet through renderHTML so the {csrf} placeholder is
	// substituted just like during a real request.
	ctx := &renderContext{
		actions:  app.Actions,
		i18n:     s.i18n,
		queries:  map[string][]database.Row{},
		paginate: map[string]PaginateInfo{},
	}
	rendered := renderHTML(`<button action="/tasks/create">Create</button>`, ctx)

	if !strings.Contains(rendered, `hx-post="/tasks/create"`) {
		t.Fatalf("expected hx-post in rendered output: %s", rendered)
	}
	// {csrf} must have been substituted (no literal placeholder left).
	if strings.Contains(rendered, "{csrf}") {
		t.Fatalf("csrf placeholder not substituted: %s", rendered)
	}

	// Pull the CSRF token out of the rendered hx-vals JSON.
	tokenRe := regexp.MustCompile(`hx-vals='\{"_csrf":"([^"]+)"\}'`)
	m := tokenRe.FindStringSubmatch(rendered)
	if len(m) < 2 {
		t.Fatalf("could not find csrf token in rendered output: %s", rendered)
	}
	csrf := m[1]

	// POST it back through handleAction. The handler reads _csrf from
	// formData; htmx in the browser would translate hx-vals into form
	// fields, so we mirror that here.
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/tasks/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleAction(rec, req, action, s.getApp())

	if rec.Code == 403 {
		t.Fatalf("CSRF rejected: action= flow does not propagate a valid token (got 403, body=%q)", rec.Body.String())
	}
}
