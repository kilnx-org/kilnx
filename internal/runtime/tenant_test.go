package runtime

import (
	"errors"
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

func withTenantParam() map[string]string {
	return map[string]string{"current_user.org_id": "42"}
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
	got, err := RewriteTenantSQL("SELECT id, number FROM quote", BuildTenantMap(buildApp(t)), withTenantParam())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "SELECT id, number FROM quote WHERE quote.org_id = :current_user.org_id"
	if got != want {
		t.Errorf("got:\n  %q\nwant:\n  %q", got, want)
	}
}

func TestRewriteTenantSQL_AppendsAndToExistingWhere(t *testing.T) {
	got, err := RewriteTenantSQL("SELECT id FROM quote WHERE status = 'sent'", BuildTenantMap(buildApp(t)), withTenantParam())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "WHERE quote.org_id = :current_user.org_id AND status = 'sent'") {
		t.Errorf("filter not inserted before existing WHERE predicates: %q", got)
	}
}

func TestRewriteTenantSQL_InsertsBeforeOrderBy(t *testing.T) {
	got, err := RewriteTenantSQL("SELECT id FROM quote ORDER BY created DESC", BuildTenantMap(buildApp(t)), withTenantParam())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "WHERE quote.org_id = :current_user.org_id ") {
		t.Errorf("filter should land before ORDER BY: %q", got)
	}
	if !strings.Contains(got, "ORDER BY created DESC") {
		t.Errorf("ORDER BY lost: %q", got)
	}
}

func TestRewriteTenantSQL_UsesAlias(t *testing.T) {
	got, err := RewriteTenantSQL("SELECT q.id FROM quote q WHERE q.status = 'draft'", BuildTenantMap(buildApp(t)), withTenantParam())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "q.org_id = :current_user.org_id") {
		t.Errorf("alias not used: %q", got)
	}
}

func TestRewriteTenantSQL_LeavesNonTenantTableAlone(t *testing.T) {
	in := "SELECT sku, name FROM material"
	got, err := RewriteTenantSQL(in, BuildTenantMap(buildApp(t)), withTenantParam())
	if err != nil || got != in {
		t.Errorf("non-tenant table should pass through, got: %q (err %v)", got, err)
	}
}

func TestRewriteTenantSQL_EmptyTenantMapIsNoOp(t *testing.T) {
	in := "SELECT id FROM quote"
	got, err := RewriteTenantSQL(in, TenantMap{}, nil)
	if err != nil || got != in {
		t.Errorf("empty map should be a no-op, got %q err=%v", got, err)
	}
}

// -------- fail-closed shape guards --------

func TestRewriteTenantSQL_RejectsJoin(t *testing.T) {
	_, err := RewriteTenantSQL("SELECT q.id FROM quote q JOIN material m ON m.id = q.material_id",
		BuildTenantMap(buildApp(t)), withTenantParam())
	if !errors.Is(err, ErrUnsafeTenantShape) {
		t.Fatalf("expected ErrUnsafeTenantShape for JOIN, got %v", err)
	}
}

func TestRewriteTenantSQL_RejectsSubqueryInFrom(t *testing.T) {
	_, err := RewriteTenantSQL("SELECT * FROM (SELECT id FROM quote) sub",
		BuildTenantMap(buildApp(t)), withTenantParam())
	if !errors.Is(err, ErrUnsafeTenantShape) {
		t.Fatalf("expected ErrUnsafeTenantShape for FROM subquery, got %v", err)
	}
}

func TestRewriteTenantSQL_RejectsCTE(t *testing.T) {
	_, err := RewriteTenantSQL("WITH t AS (SELECT id FROM quote) SELECT * FROM t",
		BuildTenantMap(buildApp(t)), withTenantParam())
	if !errors.Is(err, ErrUnsafeTenantShape) {
		t.Fatalf("expected ErrUnsafeTenantShape for CTE, got %v", err)
	}
}

func TestRewriteTenantSQL_RejectsUnion(t *testing.T) {
	_, err := RewriteTenantSQL("SELECT id FROM quote UNION SELECT id FROM customer",
		BuildTenantMap(buildApp(t)), withTenantParam())
	if !errors.Is(err, ErrUnsafeTenantShape) {
		t.Fatalf("expected ErrUnsafeTenantShape for UNION, got %v", err)
	}
}

func TestRewriteTenantSQL_RejectsSchemaQualifiedTable(t *testing.T) {
	_, err := RewriteTenantSQL("SELECT id FROM public.quote",
		BuildTenantMap(buildApp(t)), withTenantParam())
	if !errors.Is(err, ErrUnsafeTenantShape) {
		t.Fatalf("expected ErrUnsafeTenantShape for schema-qualified table, got %v", err)
	}
}

func TestRewriteTenantSQL_RejectsBlockComment(t *testing.T) {
	_, err := RewriteTenantSQL("SELECT id FROM quote /* where */",
		BuildTenantMap(buildApp(t)), withTenantParam())
	if !errors.Is(err, ErrUnsafeTenantShape) {
		t.Fatalf("expected ErrUnsafeTenantShape with comments, got %v", err)
	}
}

func TestRewriteTenantSQL_RejectsLineComment(t *testing.T) {
	_, err := RewriteTenantSQL("SELECT id FROM quote -- WHERE id = 1",
		BuildTenantMap(buildApp(t)), withTenantParam())
	if !errors.Is(err, ErrUnsafeTenantShape) {
		t.Fatalf("expected ErrUnsafeTenantShape with comments, got %v", err)
	}
}

func TestRewriteTenantSQL_RejectsEmbeddedSemicolon(t *testing.T) {
	_, err := RewriteTenantSQL("SELECT id FROM quote; DROP TABLE quote",
		BuildTenantMap(buildApp(t)), withTenantParam())
	if !errors.Is(err, ErrUnsafeTenantShape) {
		t.Fatalf("expected ErrUnsafeTenantShape for multi-statement, got %v", err)
	}
}

// -------- missing-param guard --------

func TestRewriteTenantSQL_FailsWhenTenantParamMissing(t *testing.T) {
	_, err := RewriteTenantSQL("SELECT id FROM quote", BuildTenantMap(buildApp(t)), map[string]string{})
	if !errors.Is(err, ErrMissingTenantParam) {
		t.Fatalf("expected ErrMissingTenantParam when session has no org_id, got %v", err)
	}
}

// -------- mutation guards --------

func TestRewriteTenantSQL_MutationWithExplicitTenantPasses(t *testing.T) {
	in := "INSERT INTO quote (org_id, number) VALUES (:current_user.org_id, :n)"
	got, err := RewriteTenantSQL(in, BuildTenantMap(buildApp(t)), withTenantParam())
	if err != nil {
		t.Fatalf("explicit-mutation should pass, got %v", err)
	}
	if got != in {
		t.Errorf("mutation should not be rewritten, got %q", got)
	}
}

func TestRewriteTenantSQL_MutationWithoutTenantRejected(t *testing.T) {
	cases := []string{
		"UPDATE quote SET status = 'sent' WHERE id = :id",
		"DELETE FROM quote WHERE id = :id",
		"INSERT INTO quote (number) VALUES (:n)",
	}
	for _, in := range cases {
		_, err := RewriteTenantSQL(in, BuildTenantMap(buildApp(t)), withTenantParam())
		if !errors.Is(err, ErrMutationNotScoped) {
			t.Errorf("mutation %q should be rejected, got %v", in, err)
		}
	}
}

func TestRewriteTenantSQL_MutationOnNonTenantTablePasses(t *testing.T) {
	in := "DELETE FROM material WHERE id = :id"
	got, err := RewriteTenantSQL(in, BuildTenantMap(buildApp(t)), withTenantParam())
	if err != nil {
		t.Fatalf("non-tenant mutation should pass, got %v", err)
	}
	if got != in {
		t.Errorf("non-tenant mutation should be unchanged, got %q", got)
	}
}

// -------- sensitive current_user field redaction --------

func TestIsSensitiveField(t *testing.T) {
	cases := map[string]bool{
		"password":                true,
		"Password":                true,
		"password_hash":           true,
		"reset_token":             true,
		"email_verification_token":true,
		"api_key":                 true,
		"apikey":                  true,
		"secret":                  true,
		"salt":                    true,
		"mfa_secret":              true,
		"email":                   false,
		"name":                    false,
		"org_id":                  false,
		"role":                    false,
		"locale":                  false,
	}
	for name, want := range cases {
		if got := isSensitiveField(name, ""); got != want {
			t.Errorf("isSensitiveField(%q) = %v, want %v", name, got, want)
		}
	}
	// explicit auth.password column overrides even if named oddly
	if !isSensitiveField("pw_hash", "pw_hash") {
		t.Error("explicit password column should be treated as sensitive")
	}
}
