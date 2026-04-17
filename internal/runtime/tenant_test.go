package runtime

import (
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/parser"
)

func buildApp(t *testing.T) *parser.App {
	t.Helper()
	return &parser.App{
		Models: []parser.Model{
			{Name: "org"},
			{Name: "quote", Tenant: "org"},
			{Name: "material"}, // not tenant-scoped
			{Name: "customer", Tenant: "org"},
		},
	}
}

func TestBuildTenantMap(t *testing.T) {
	m := BuildTenantMap(buildApp(t))
	if m["quote"] != "org" {
		t.Errorf("expected quote->org, got %v", m)
	}
	if m["customer"] != "org" {
		t.Errorf("expected customer->org, got %v", m)
	}
	if _, ok := m["material"]; ok {
		t.Error("material should not be tenant-scoped")
	}
}

func TestRewriteTenantSQL_AppendsWhereWhenNone(t *testing.T) {
	m := BuildTenantMap(buildApp(t))
	in := "SELECT id, number FROM quote"
	got := RewriteTenantSQL(in, m)
	want := "SELECT id, number FROM quote WHERE quote.org_id = :current_user.org_id"
	if got != want {
		t.Errorf("got:\n  %q\nwant:\n  %q", got, want)
	}
}

func TestRewriteTenantSQL_AppendsAndToExistingWhere(t *testing.T) {
	m := BuildTenantMap(buildApp(t))
	in := "SELECT id FROM quote WHERE status = 'sent'"
	got := RewriteTenantSQL(in, m)
	if !strings.Contains(got, "WHERE quote.org_id = :current_user.org_id AND status = 'sent'") {
		t.Errorf("filter not inserted before existing WHERE predicates: %q", got)
	}
}

func TestRewriteTenantSQL_InsertsBeforeOrderBy(t *testing.T) {
	m := BuildTenantMap(buildApp(t))
	in := "SELECT id FROM quote ORDER BY created DESC"
	got := RewriteTenantSQL(in, m)
	if !strings.Contains(got, "WHERE quote.org_id = :current_user.org_id ") {
		t.Errorf("filter should land before ORDER BY: %q", got)
	}
	if !strings.Contains(got, "ORDER BY created DESC") {
		t.Errorf("ORDER BY lost: %q", got)
	}
}

func TestRewriteTenantSQL_UsesAlias(t *testing.T) {
	m := BuildTenantMap(buildApp(t))
	in := "SELECT q.id, q.number FROM quote q WHERE q.status = 'draft'"
	got := RewriteTenantSQL(in, m)
	if !strings.Contains(got, "q.org_id = :current_user.org_id") {
		t.Errorf("alias not used: %q", got)
	}
}

func TestRewriteTenantSQL_LeavesNonTenantTableAlone(t *testing.T) {
	m := BuildTenantMap(buildApp(t))
	in := "SELECT sku, name FROM material"
	got := RewriteTenantSQL(in, m)
	if got != in {
		t.Errorf("non-tenant table should pass through, got: %q", got)
	}
}

func TestRewriteTenantSQL_LeavesMutationsAlone(t *testing.T) {
	m := BuildTenantMap(buildApp(t))
	cases := []string{
		"UPDATE quote SET status = 'sent' WHERE id = :id",
		"DELETE FROM quote WHERE id = :id",
		"INSERT INTO quote (number, title) VALUES (:n, :t)",
	}
	for _, in := range cases {
		if got := RewriteTenantSQL(in, m); got != in {
			t.Errorf("mutation should not be rewritten at runtime layer: %q -> %q", in, got)
		}
	}
}

func TestRewriteTenantSQL_EmptyTenantMapIsNoOp(t *testing.T) {
	in := "SELECT id FROM quote"
	if got := RewriteTenantSQL(in, TenantMap{}); got != in {
		t.Errorf("expected no-op, got %q", got)
	}
}
