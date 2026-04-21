package runtime

import (
	"testing"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

func makeSession(identity, role string, data database.Row) *Session {
	if data == nil {
		data = database.Row{}
	}
	data["role"] = role
	return &Session{
		UserID:   "1",
		Identity: identity,
		Role:     role,
		Data:     data,
	}
}

func makeServer(superuser string) *Server {
	app := &parser.App{}
	s := &Server{app: app, superuserIdentity: superuser}
	return s
}

func TestEvalAuthExpr_inList(t *testing.T) {
	sess := makeSession("user@example.com", "viewer", database.Row{"plan": "cad"})
	tests := []struct {
		expr string
		want bool
	}{
		{"current_user.plan in ['cad','full']", true},
		{"current_user.plan in ['full','enterprise']", false},
		{"current_user.role in ['admin','editor']", false},
		{"current_user.role in ['viewer','editor']", true},
	}
	for _, tt := range tests {
		got := evalAuthExpr(tt.expr, sess)
		if got != tt.want {
			t.Errorf("evalAuthExpr(%q): got %v, want %v", tt.expr, got, tt.want)
		}
	}
}

func TestEvalAuthExpr_comparison(t *testing.T) {
	sess := makeSession("u@e.com", "admin", database.Row{"level": "5", "active": "true"})
	tests := []struct {
		expr string
		want bool
	}{
		{"current_user.level == '5'", true},
		{"current_user.level != '3'", true},
		{"current_user.level > '3'", true},
		{"current_user.level < '3'", false},
		{"current_user.active == 'true'", true},
		{"current_user.active == 'false'", false},
	}
	for _, tt := range tests {
		got := evalAuthExpr(tt.expr, sess)
		if got != tt.want {
			t.Errorf("evalAuthExpr(%q): got %v, want %v", tt.expr, got, tt.want)
		}
	}
}

func TestEvalAuthExpr_andOr(t *testing.T) {
	sess := makeSession("u@e.com", "editor", database.Row{"plan": "full", "active": "true"})
	tests := []struct {
		expr string
		want bool
	}{
		{"current_user.plan in ['full'] and current_user.active == 'true'", true},
		{"current_user.plan in ['full'] and current_user.active == 'false'", false},
		{"current_user.plan in ['basic'] or current_user.active == 'true'", true},
		{"current_user.plan in ['basic'] or current_user.active == 'false'", false},
	}
	for _, tt := range tests {
		got := evalAuthExpr(tt.expr, sess)
		if got != tt.want {
			t.Errorf("evalAuthExpr(%q): got %v, want %v", tt.expr, got, tt.want)
		}
	}
}

func TestEvalRequiresClauses_auth(t *testing.T) {
	s := makeServer("")
	sess := makeSession("u@e.com", "viewer", nil)
	clauses := []parser.RequiresClause{{Kind: parser.RequiresClauseAuth}}
	if !s.evalRequiresClauses(clauses, sess) {
		t.Error("auth clause should pass for logged-in user")
	}
}

func TestEvalRequiresClauses_role(t *testing.T) {
	s := makeServer("")
	admin := makeSession("a@e.com", "admin", nil)
	viewer := makeSession("v@e.com", "viewer", nil)

	clauses := []parser.RequiresClause{{Kind: parser.RequiresClauseRole, Value: "admin"}}
	if !s.evalRequiresClauses(clauses, admin) {
		t.Error("admin clause should pass for admin user")
	}
	if s.evalRequiresClauses(clauses, viewer) {
		t.Error("admin clause should fail for viewer")
	}
}

func TestEvalRequiresClauses_expr(t *testing.T) {
	s := makeServer("")
	sess := makeSession("u@e.com", "viewer", database.Row{"plan": "cad"})
	clauses := []parser.RequiresClause{
		{Kind: parser.RequiresClauseAuth},
		{Kind: parser.RequiresClauseExpr, Value: "current_user.plan in ['cad','full']"},
	}
	if !s.evalRequiresClauses(clauses, sess) {
		t.Error("should pass: user has plan=cad")
	}
	sess2 := makeSession("u@e.com", "viewer", database.Row{"plan": "basic"})
	if s.evalRequiresClauses(clauses, sess2) {
		t.Error("should fail: user has plan=basic")
	}
}

func TestEvalRequiresClauses_superuser_clause(t *testing.T) {
	s := makeServer("ops@example.com")
	ops := makeSession("ops@example.com", "viewer", nil)
	regular := makeSession("user@example.com", "admin", nil)

	clauses := []parser.RequiresClause{{Kind: parser.RequiresClauseSuperuser}}
	if !s.evalRequiresClauses(clauses, ops) {
		t.Error("superuser clause should pass for superuser identity")
	}
	if s.evalRequiresClauses(clauses, regular) {
		t.Error("superuser clause should fail for non-superuser")
	}
}

func TestEvalRequiresClauses_superuserBypass(t *testing.T) {
	s := makeServer("ops@example.com")
	ops := makeSession("ops@example.com", "viewer", nil)

	// superuser bypasses even an admin-only clause
	clauses := []parser.RequiresClause{{Kind: parser.RequiresClauseRole, Value: "admin"}}
	if !s.evalRequiresClauses(clauses, ops) {
		t.Error("superuser identity should bypass role check")
	}
}

func TestParseInList(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"'cad','full'", []string{"cad", "full"}},
		{"\"cad\", \"full\"", []string{"cad", "full"}},
		{"admin,editor,viewer", []string{"admin", "editor", "viewer"}},
		{"'a'", []string{"a"}},
	}
	for _, tt := range tests {
		got := parseInList(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseInList(%q): got %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseInList(%q)[%d]: got %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}
