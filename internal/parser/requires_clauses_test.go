package parser

import (
	"testing"

	"github.com/kilnx-org/kilnx/internal/lexer"
)

func mustParse(t *testing.T, src string) *App {
	t.Helper()
	tokens := lexer.Tokenize(src)
	app, err := Parse(tokens, src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return app
}

const minConfig = `config
  database: "sqlite://test.db"
  port: 8080
  secret: "sec"

`

func TestSplitClauseText(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"auth", []string{"auth"}},
		{"admin", []string{"admin"}},
		{"auth, admin", []string{"auth", "admin"}},
		{":current_user.plan in ['cad','full']", []string{":current_user.plan in ['cad','full']"}},
		{"auth, :current_user.plan in ['cad','full']", []string{"auth", ":current_user.plan in ['cad','full']"}},
		{"admin, :current_user.active == 'true'", []string{"admin", ":current_user.active == 'true'"}},
	}
	for _, tt := range tests {
		got := splitClauseText(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitClauseText(%q): got %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitClauseText(%q)[%d]: got %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestParseOneClause(t *testing.T) {
	tests := []struct {
		input    string
		wantKind RequiresClauseKind
		wantVal  string
	}{
		{"auth", RequiresClauseAuth, ""},
		{"", RequiresClauseAuth, ""},
		{"superuser", RequiresClauseSuperuser, ""},
		{"admin", RequiresClauseRole, "admin"},
		{"editor", RequiresClauseRole, "editor"},
		{":current_user.plan in ['cad']", RequiresClauseExpr, "current_user.plan in ['cad']"},
		{":role == 'admin'", RequiresClauseExpr, "role == 'admin'"},
	}
	for _, tt := range tests {
		got := parseOneClause(tt.input)
		if got.Kind != tt.wantKind || got.Value != tt.wantVal {
			t.Errorf("parseOneClause(%q): got {%v,%q}, want {%v,%q}", tt.input, got.Kind, got.Value, tt.wantKind, tt.wantVal)
		}
	}
}

func TestRequiresClauseEnd(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{" admin", " admin"},
		{" auth method POST", " auth "},
		{" admin layout main", " admin "},
		{" :current_user.plan in ['cad','full'] method POST", " :current_user.plan in ['cad','full'] "},
		{" auth, :plan in ['a','b']", " auth, :plan in ['a','b']"},
	}
	for _, tt := range tests {
		got := requiresClauseEnd(tt.input)
		if got != tt.want {
			t.Errorf("requiresClauseEnd(%q): got %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseRequiresClauses_page(t *testing.T) {
	tests := []struct {
		src         string
		wantClauses []RequiresClause
	}{
		{
			"page /test requires auth\n  html\n    hi\n",
			[]RequiresClause{{Kind: RequiresClauseAuth}},
		},
		{
			"page /test requires admin\n  html\n    hi\n",
			[]RequiresClause{{Kind: RequiresClauseRole, Value: "admin"}},
		},
		{
			"page /test requires superuser\n  html\n    hi\n",
			[]RequiresClause{{Kind: RequiresClauseSuperuser}},
		},
		{
			"page /test requires auth, :current_user.plan in ['cad','full']\n  html\n    hi\n",
			[]RequiresClause{
				{Kind: RequiresClauseAuth},
				{Kind: RequiresClauseExpr, Value: "current_user.plan in ['cad','full']"},
			},
		},
		{
			"page /test requires admin, :current_user.active == 'true'\n  html\n    hi\n",
			[]RequiresClause{
				{Kind: RequiresClauseRole, Value: "admin"},
				{Kind: RequiresClauseExpr, Value: "current_user.active == 'true'"},
			},
		},
	}

	for _, tt := range tests {
		app := mustParse(t, minConfig+tt.src)
		if len(app.Pages) == 0 {
			t.Errorf("no pages parsed for %q", tt.src)
			continue
		}
		got := app.Pages[0].RequiresClauses
		if len(got) != len(tt.wantClauses) {
			t.Errorf("RequiresClauses for %q: got %v, want %v", tt.src, got, tt.wantClauses)
			continue
		}
		for i, c := range got {
			if c.Kind != tt.wantClauses[i].Kind || c.Value != tt.wantClauses[i].Value {
				t.Errorf("RequiresClauses[%d] for %q: got {%v,%q}, want {%v,%q}",
					i, tt.src, c.Kind, c.Value, tt.wantClauses[i].Kind, tt.wantClauses[i].Value)
			}
		}
	}
}

func TestParseAuthSuperuser(t *testing.T) {
	src := minConfig + `model user
  email: email
  password: password

auth
  table: user
  identity: email
  password: password
  login: /login
  superuser: ops@example.com
`
	app := mustParse(t, src)
	if app.Auth == nil {
		t.Fatal("auth is nil")
	}
	if app.Auth.Superuser != "ops@example.com" {
		t.Errorf("Superuser: got %q, want %q", app.Auth.Superuser, "ops@example.com")
	}
}

func TestRequiresModifierBoundary(t *testing.T) {
	src := minConfig + `action /submit requires auth method POST
  query: INSERT INTO x (a) VALUES (:a)
  redirect /
`
	app := mustParse(t, src)
	if len(app.Actions) == 0 {
		t.Fatal("no actions")
	}
	a := app.Actions[0]
	if a.Method != "POST" {
		t.Errorf("method: got %q, want POST", a.Method)
	}
	if len(a.RequiresClauses) != 1 || a.RequiresClauses[0].Kind != RequiresClauseAuth {
		t.Errorf("RequiresClauses: got %v", a.RequiresClauses)
	}
}

func TestRequiresStream(t *testing.T) {
	src := minConfig + `stream /feed requires admin
  query: SELECT id FROM x
  every 5s
`
	app := mustParse(t, src)
	if len(app.Streams) == 0 {
		t.Fatal("no streams")
	}
	s := app.Streams[0]
	if len(s.RequiresClauses) != 1 || s.RequiresClauses[0].Kind != RequiresClauseRole || s.RequiresClauses[0].Value != "admin" {
		t.Errorf("Stream.RequiresClauses: got %v", s.RequiresClauses)
	}
}

func TestRequiresSocket(t *testing.T) {
	src := minConfig + `socket /ws requires auth
  on connect
    query: SELECT 1
`
	app := mustParse(t, src)
	if len(app.Sockets) == 0 {
		t.Fatal("no sockets")
	}
	sk := app.Sockets[0]
	if len(sk.RequiresClauses) != 1 || sk.RequiresClauses[0].Kind != RequiresClauseAuth {
		t.Errorf("Socket.RequiresClauses: got %v", sk.RequiresClauses)
	}
}
