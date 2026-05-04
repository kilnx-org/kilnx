package runtime

import (
	"testing"

	"github.com/kilnx-org/kilnx/internal/parser"
)

func TestBuildPermissionMap(t *testing.T) {
	app := &parser.App{
		Permissions: []parser.Permission{
			{
				Role:  "admin",
				Rules: []string{"all"},
			},
			{
				Role:  "editor",
				Rules: []string{"read post", "write post where author = current_user"},
			},
			{
				Role:  "viewer",
				Rules: []string{"read post where status = published"},
			},
		},
	}

	pm := BuildPermissionMap(app)

	if !pm.CanAccess("admin", "post") {
		t.Error("admin should have blanket access")
	}
	if !pm.CanRead("editor", "post") {
		t.Error("editor should be able to read post")
	}
	if !pm.CanWrite("editor", "post") {
		t.Error("editor should be able to write post")
	}
	if !pm.CanRead("viewer", "post") {
		t.Error("viewer should be able to read post")
	}
	if pm.CanWrite("viewer", "post") {
		t.Error("viewer should NOT be able to write post")
	}
	if pm.CanRead("viewer", "comment") {
		t.Error("viewer should NOT be able to read comment")
	}
}

func TestParsePermissionRule(t *testing.T) {
	tests := []struct {
		raw      string
		want     *PermissionRule
		wantNil  bool
	}{
		{"all", &PermissionRule{Action: "all", Resource: "", Condition: ""}, false},
		{"read post", &PermissionRule{Action: "read", Resource: "post", Condition: ""}, false},
		{"write post where author = current_user", &PermissionRule{Action: "write", Resource: "post", Condition: "author = current_user"}, false},
		{"read post where status = published", &PermissionRule{Action: "read", Resource: "post", Condition: "status = published"}, false},
		{"invalid", nil, true},
	}

	for _, tc := range tests {
		got := parsePermissionRule(tc.raw)
		if tc.wantNil {
			if got != nil {
				t.Errorf("parsePermissionRule(%q) = %+v, want nil", tc.raw, got)
			}
			continue
		}
		if got == nil {
			t.Errorf("parsePermissionRule(%q) = nil, want %+v", tc.raw, tc.want)
			continue
		}
		if got.Action != tc.want.Action || got.Resource != tc.want.Resource || got.Condition != tc.want.Condition {
			t.Errorf("parsePermissionRule(%q) = %+v, want %+v", tc.raw, got, tc.want)
		}
	}
}

func TestRewritePermissionSQL_ReadWithCondition(t *testing.T) {
	app := &parser.App{
		Permissions: []parser.Permission{
			{Role: "viewer", Rules: []string{"read post where status = published"}},
		},
	}
	pm := BuildPermissionMap(app)
	params := map[string]string{"current_user.id": "42"}

	in := "SELECT id, title FROM post"
	got, err := RewritePermissionSQL(in, pm, "viewer", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "WHERE post.status = published"
	if !contains(got, want) {
		t.Errorf("expected %q to contain %q, got:\n%s", got, want, got)
	}
}

func TestRewritePermissionSQL_ReadWithCurrentUser(t *testing.T) {
	app := &parser.App{
		Permissions: []parser.Permission{
			{Role: "editor", Rules: []string{"read post where author = current_user"}},
		},
	}
	pm := BuildPermissionMap(app)
	params := map[string]string{"current_user.id": "7"}

	in := "SELECT id, title FROM post"
	got, err := RewritePermissionSQL(in, pm, "editor", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "WHERE post.author = :current_user.id"
	if !contains(got, want) {
		t.Errorf("expected %q to contain %q, got:\n%s", got, want, got)
	}
}

func TestRewritePermissionSQL_NoConditionNoRewrite(t *testing.T) {
	app := &parser.App{
		Permissions: []parser.Permission{
			{Role: "editor", Rules: []string{"read post"}},
		},
	}
	pm := BuildPermissionMap(app)
	params := map[string]string{}

	in := "SELECT id FROM post"
	got, err := RewritePermissionSQL(in, pm, "editor", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != in {
		t.Errorf("expected no rewrite, got:\n%s", got)
	}
}

func TestRewritePermissionSQL_RoleWithoutRules(t *testing.T) {
	app := &parser.App{
		Permissions: []parser.Permission{
			{Role: "editor", Rules: []string{"read post where status = published"}},
		},
	}
	pm := BuildPermissionMap(app)
	params := map[string]string{}

	in := "SELECT id FROM post"
	got, err := RewritePermissionSQL(in, pm, "admin", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != in {
		t.Errorf("expected no rewrite for unrelated role, got:\n%s", got)
	}
}

func TestRewritePermissionSQL_MutationNotRewritten(t *testing.T) {
	app := &parser.App{
		Permissions: []parser.Permission{
			{Role: "editor", Rules: []string{"write post where author = current_user"}},
		},
	}
	pm := BuildPermissionMap(app)
	params := map[string]string{"current_user.id": "7"}

	in := "INSERT INTO post (title) VALUES ('hello')"
	got, err := RewritePermissionSQL(in, pm, "editor", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != in {
		t.Errorf("mutations should not be rewritten, got:\n%s", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestPermissionMapConditionForWrite(t *testing.T) {
	app := &parser.App{
		Permissions: []parser.Permission{
			{Role: "editor", Rules: []string{"write post where author = current_user"}},
			{Role: "admin", Rules: []string{"write post"}},
		},
	}
	pm := BuildPermissionMap(app)

	if got := pm.ConditionForWrite("editor", "post"); got != "author = current_user" {
		t.Errorf("editor write condition = %q, want %q", got, "author = current_user")
	}
	if got := pm.ConditionForWrite("admin", "post"); got != "" {
		t.Errorf("admin write condition = %q, want empty", got)
	}
	if got := pm.ConditionForWrite("viewer", "post"); got != "" {
		t.Errorf("viewer write condition = %q, want empty", got)
	}
}

func TestPermissionMapCanAccessHierarchy(t *testing.T) {
	app := &parser.App{
		Permissions: []parser.Permission{
			{Role: "viewer", Rules: []string{"read post"}},
		},
	}
	pm := BuildPermissionMap(app)

	// viewer has explicit rule
	if !pm.CanAccess("viewer", "post") {
		t.Error("viewer should access post")
	}
	// editor has no explicit rule but is higher in hierarchy
	if !pm.CanAccess("editor", "post") {
		t.Error("editor should access post via hierarchy")
	}
	// admin is highest
	if !pm.CanAccess("admin", "post") {
		t.Error("admin should access post via hierarchy")
	}
	// unknown role without rule should not access
	if pm.CanAccess("guest", "post") {
		t.Error("guest should NOT access post")
	}
}

func TestRewritePermissionSQL_AppendsAndToExistingWhere(t *testing.T) {
	app := &parser.App{
		Permissions: []parser.Permission{
			{Role: "viewer", Rules: []string{"read post where status = published"}},
		},
	}
	pm := BuildPermissionMap(app)
	params := map[string]string{}

	in := "SELECT id FROM post WHERE category = 'news'"
	got, err := RewritePermissionSQL(in, pm, "viewer", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "WHERE post.status = published AND category = 'news'"
	if !contains(got, want) {
		t.Errorf("expected %q to contain %q, got:\n%s", got, want, got)
	}
}

func TestRewritePermissionSQL_InsertsBeforeOrderBy(t *testing.T) {
	app := &parser.App{
		Permissions: []parser.Permission{
			{Role: "viewer", Rules: []string{"read post where status = published"}},
		},
	}
	pm := BuildPermissionMap(app)
	params := map[string]string{}

	in := "SELECT id FROM post ORDER BY created DESC"
	got, err := RewritePermissionSQL(in, pm, "viewer", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "WHERE post.status = published ORDER BY"
	if !contains(got, want) {
		t.Errorf("expected %q to contain %q, got:\n%s", got, want, got)
	}
}

func TestRewritePermissionSQL_UsesAlias(t *testing.T) {
	app := &parser.App{
		Permissions: []parser.Permission{
			{Role: "viewer", Rules: []string{"read post where status = published"}},
		},
	}
	pm := BuildPermissionMap(app)
	params := map[string]string{}

	in := "SELECT p.id FROM post p"
	got, err := RewritePermissionSQL(in, pm, "viewer", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "WHERE p.status = published"
	if !contains(got, want) {
		t.Errorf("expected %q to contain %q, got:\n%s", got, want, got)
	}
}


func TestBuildPermissionMap_NilApp(t *testing.T) {
	pm := BuildPermissionMap(nil)
	if len(pm) != 0 {
		t.Error("nil app should return empty permission map")
	}
}

func TestRewritePermissionSQL_NoPermissions(t *testing.T) {
	sql := `SELECT * FROM posts`
	pm := PermissionMap{}
	result, err := RewritePermissionSQL(sql, pm, "viewer", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != sql {
		t.Errorf("expected unchanged SQL, got %q", result)
	}
}

func TestRewritePermissionSQL_EmptyRole(t *testing.T) {
	sql := `SELECT * FROM posts`
	pm := BuildPermissionMap(&parser.App{
		Permissions: []parser.Permission{
			{Role: "viewer", Rules: []string{"read post where status = published"}},
		},
	})
	result, err := RewritePermissionSQL(sql, pm, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != sql {
		t.Errorf("expected unchanged SQL for empty role, got %q", result)
	}
}

func TestRewritePermissionSQL_SQLComment(t *testing.T) {
	sql := `SELECT * FROM posts /* comment */`
	pm := BuildPermissionMap(&parser.App{
		Permissions: []parser.Permission{
			{Role: "viewer", Rules: []string{"read post where status = published"}},
		},
	})
	_, err := RewritePermissionSQL(sql, pm, "viewer", nil)
	if err == nil {
		t.Error("expected error for SQL with comments")
	}
}

func TestRewritePermissionSQL_NonSelect(t *testing.T) {
	sql := `INSERT INTO posts (title) VALUES ('test')`
	pm := BuildPermissionMap(&parser.App{
		Permissions: []parser.Permission{
			{Role: "viewer", Rules: []string{"read post where status = published"}},
		},
	})
	result, err := RewritePermissionSQL(sql, pm, "viewer", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != sql {
		t.Errorf("expected unchanged SQL for non-SELECT, got %q", result)
	}
}

func TestRewritePermissionSQL_NoTableMatch(t *testing.T) {
	sql := `SELECT 1`
	pm := BuildPermissionMap(&parser.App{
		Permissions: []parser.Permission{
			{Role: "viewer", Rules: []string{"read post where status = published"}},
		},
	})
	result, err := RewritePermissionSQL(sql, pm, "viewer", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != sql {
		t.Errorf("expected unchanged SQL when no table match, got %q", result)
	}
}

func TestRewritePermissionSQL_UnsafeShape(t *testing.T) {
	sql := `SELECT * FROM posts UNION SELECT * FROM comments`
	pm := BuildPermissionMap(&parser.App{
		Permissions: []parser.Permission{
			{Role: "viewer", Rules: []string{"read post where status = published"}},
		},
	})
	result, err := RewritePermissionSQL(sql, pm, "viewer", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != sql {
		t.Errorf("expected unchanged SQL for unsafe shape, got %q", result)
	}
}

func TestRewritePermissionSQL_InvalidCondition(t *testing.T) {
	sql := `SELECT * FROM post`
	pm := BuildPermissionMap(&parser.App{
		Permissions: []parser.Permission{
			{Role: "viewer", Rules: []string{"read post where invalid-condition"}},
		},
	})
	_, err := RewritePermissionSQL(sql, pm, "viewer", nil)
	if err == nil {
		t.Error("expected error for invalid condition")
	}
}

func TestRewritePermissionSQL_EmptyFilter(t *testing.T) {
	sql := `SELECT * FROM posts`
	pm := BuildPermissionMap(&parser.App{
		Permissions: []parser.Permission{
			{Role: "viewer", Rules: []string{"read post"}},
		},
	})
	result, err := RewritePermissionSQL(sql, pm, "viewer", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != sql {
		t.Errorf("expected unchanged SQL when no condition, got %q", result)
	}
}


func TestRewritePermissionSQL_UnresolvedPlaceholder(t *testing.T) {
	sql := `SELECT * FROM post`
	pm := BuildPermissionMap(&parser.App{
		Permissions: []parser.Permission{
			{Role: "viewer", Rules: []string{"read post where author_id = :current_user.id"}},
		},
	})
	result, err := RewritePermissionSQL(sql, pm, "viewer", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != sql {
		t.Errorf("expected unchanged SQL when placeholder unresolved, got %q", result)
	}
}
