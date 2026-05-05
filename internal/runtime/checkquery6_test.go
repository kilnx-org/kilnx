package runtime

import (
	"fmt"
	"testing"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
	"net/http/httptest"
	"net/url"
	"strings"
)

func TestCheckQuery6(t *testing.T) {
	db, err := database.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	db.Conn().Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)")
	db.Conn().Exec("INSERT INTO users (id, email) VALUES (1, 'queried@example.com')")

	app := &parser.App{
		Auth: &parser.AuthConfig{
			Table: "users", Identity: "email", Password: "password_hash",
			LoginPath: "/login", RegisterPath: "/register",
			ForgotPath: "/forgot-password", ResetPath: "/reset-password",
		},
	}
	s := &Server{app: app, db: db}
	s.sessions = NewSessionStore("test-secret")
	s.sessions.SetDB(db)
	s.logger = NewLogger(nil)

	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}, "id": {"1"}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeSendEmail, EmailTo: ":email", EmailSubject: "Hello", Props: map[string]string{
				"to_query": `SELECT email FROM users WHERE id = :id`,
				"body":     "Welcome!",
			}},
			{Type: parser.NodeRedirect, Value: "/done"},
		},
	}

	fmt.Printf("before handleAction: db=%p s.db=%p\n", db, s.db)
	rows, err := db.QueryRowsWithParams("SELECT email FROM users WHERE id = :id", map[string]string{"id": "1"})
	fmt.Printf("direct query: err=%v rows=%v\n", err, rows)

	s.handleAction(rec, req, action, s.getApp())
	fmt.Printf("code=%d\n", rec.Code)
}
