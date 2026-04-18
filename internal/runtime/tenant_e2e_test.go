package runtime

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/lexer"
	"github.com/kilnx-org/kilnx/internal/parser"
)

// startTestServerWithHandles is like startTestServer but also returns the
// DB and Server so tests can seed rows and create sessions directly.
func startTestServerWithHandles(t *testing.T, src string) (baseURL string, db *database.DB, srv *Server, cleanup func()) {
	t.Helper()
	source := lexer.StripComments(src)
	tokens := lexer.Tokenize(source)
	app, err := parser.Parse(tokens, source)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err = database.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.MigrateInternal(); err != nil {
		db.Close()
		t.Fatalf("migrate internal: %v", err)
	}
	if len(app.Models) > 0 {
		if _, err := db.Migrate(app.Models); err != nil {
			db.Close()
			t.Fatalf("migrate: %v", err)
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		db.Close()
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	srv = NewServer(app, db, port)
	go srv.Start()

	baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/healthz")
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	return baseURL, db, srv, func() { db.Close() }
}

// getAsUser performs a GET on `path` with a session cookie for the user row
// returned by the given SQL (must return exactly one row).
func getAsUser(t *testing.T, srv *Server, baseURL, path string, user database.Row) (int, string) {
	t.Helper()
	sessionID := srv.sessions.Create(user, "email")
	signed := srv.sessions.signSessionID(sessionID)

	req, err := http.NewRequest("GET", baseURL+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: "_kilnx_session", Value: signed})

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("get %s: %v", path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// TestTenantIsolation_PageSelect seeds two orgs, logs in as each, and asserts
// that the listing page only shows the logged-in user's org quotes.
func TestTenantIsolation_PageSelect(t *testing.T) {
	src := `
config
  secret: "test-secret-32-bytes-min-len-padding"

model org
  name: text required unique

model user
  tenant: org
  email: email unique
  password: password required
  role: option [admin, member] default member

model quote
  tenant: org
  number: text required

auth
  table: user
  identity: email
  password: password
  login: /auth/login
  after login: /quotes

page /quotes requires auth
  query quotes: SELECT number FROM quote ORDER BY number
  html
    <ul>
      {{each quotes}}<li class="q">{number}</li>{{end}}
    </ul>
`
	baseURL, db, srv, cleanup := startTestServerWithHandles(t, src)
	defer cleanup()

	if err := db.ExecWithParams(`INSERT INTO org (name) VALUES ('acme')`, nil); err != nil {
		t.Fatalf("seed org1: %v", err)
	}
	if err := db.ExecWithParams(`INSERT INTO org (name) VALUES ('beta')`, nil); err != nil {
		t.Fatalf("seed org2: %v", err)
	}
	seed := func(email, orgName string) database.Row {
		rows, err := db.QueryRowsWithParams(`SELECT id FROM org WHERE name = :n`, map[string]string{"n": orgName})
		if err != nil || len(rows) == 0 {
			t.Fatalf("resolve org %s: %v", orgName, err)
		}
		orgID := rows[0]["id"]
		if err := db.ExecWithParams(
			`INSERT INTO user (email, password, role, org_id) VALUES (:email, 'x', 'member', :org)`,
			map[string]string{"email": email, "org": orgID},
		); err != nil {
			t.Fatalf("seed user %s: %v", email, err)
		}
		if err := db.ExecWithParams(
			`INSERT INTO quote (number, org_id) VALUES (:n, :o)`,
			map[string]string{"n": "Q-" + orgName, "o": orgID},
		); err != nil {
			t.Fatalf("seed quote %s: %v", orgName, err)
		}
		ur, err := db.QueryRowsWithParams(`SELECT id, email, role, org_id FROM user WHERE email = :e`, map[string]string{"e": email})
		if err != nil || len(ur) == 0 {
			t.Fatalf("resolve user row %s: %v", email, err)
		}
		return ur[0]
	}
	alice := seed("alice@acme.test", "acme")
	bob := seed("bob@beta.test", "beta")

	status, body := getAsUser(t, srv, baseURL, "/quotes", alice)
	if status != http.StatusOK {
		t.Fatalf("alice GET /quotes: status %d body=%s", status, body)
	}
	if !strings.Contains(body, "Q-acme") {
		t.Errorf("alice should see Q-acme, got:\n%s", body)
	}
	if strings.Contains(body, "Q-beta") {
		t.Errorf("TENANT LEAK: alice saw Q-beta:\n%s", body)
	}

	status, body = getAsUser(t, srv, baseURL, "/quotes", bob)
	if status != http.StatusOK {
		t.Fatalf("bob GET /quotes: status %d body=%s", status, body)
	}
	if !strings.Contains(body, "Q-beta") {
		t.Errorf("bob should see Q-beta, got:\n%s", body)
	}
	if strings.Contains(body, "Q-acme") {
		t.Errorf("TENANT LEAK: bob saw Q-acme:\n%s", body)
	}
}
