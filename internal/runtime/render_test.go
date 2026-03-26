package runtime

import (
	"html"
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/database"
)

func newTestContext() *renderContext {
	return &renderContext{
		queries:     make(map[string][]database.Row),
		paginate:    make(map[string]PaginateInfo),
		queryParams: make(map[string]string),
	}
}

func TestRenderHTML_Interpolation(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"name": "Alice", "email": "alice@test.com"}}

	result := renderHTML("<p>{user.name}</p>", ctx)
	if !strings.Contains(result, "Alice") {
		t.Errorf("expected Alice, got %s", result)
	}
}

func TestRenderHTML_InterpolationEscaped(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"name": "<script>alert(1)</script>"}}

	result := renderHTML("<p>{user.name}</p>", ctx)
	if strings.Contains(result, "<script>") {
		t.Errorf("XSS: script tag not escaped in output: %s", result)
	}
	if !strings.Contains(result, html.EscapeString("<script>alert(1)</script>")) {
		t.Errorf("expected escaped script tag, got %s", result)
	}
}

func TestRenderHTML_Count(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["users"] = []database.Row{{"name": "A"}, {"name": "B"}, {"name": "C"}}

	result := renderHTML("{users.count} users", ctx)
	if !strings.Contains(result, "3 users") {
		t.Errorf("expected '3 users', got %s", result)
	}
}

func TestRenderHTML_CSRF(t *testing.T) {
	ctx := newTestContext()
	result := renderHTML(`<input name="_csrf" value="{csrf}">`, ctx)

	if strings.Contains(result, "{csrf}") {
		t.Errorf("csrf token not replaced: %s", result)
	}
	// Token should be a 64-char hex string
	start := strings.Index(result, `value="`) + 7
	end := strings.Index(result[start:], `"`) + start
	token := result[start:end]
	if len(token) != 64 {
		t.Errorf("csrf token should be 64 chars, got %d: %s", len(token), token)
	}
	// Validate the token works
	if !validateCSRFToken(token) {
		t.Errorf("generated csrf token failed validation")
	}
}

func TestRenderHTML_CSRFNotGeneratedWhenAbsent(t *testing.T) {
	ctx := newTestContext()
	result := renderHTML("<p>No form here</p>", ctx)
	if result != "<p>No form here</p>" {
		t.Errorf("unexpected change: %s", result)
	}
}

func TestRenderHTML_EachBasic(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["users"] = []database.Row{
		{"name": "Alice", "email": "a@test.com"},
		{"name": "Bob", "email": "b@test.com"},
	}

	result := renderHTML(`{{each users}}<li>{name} - {email}</li>{{end}}`, ctx)
	if !strings.Contains(result, "Alice - a@test.com") {
		t.Errorf("expected Alice row, got %s", result)
	}
	if !strings.Contains(result, "Bob - b@test.com") {
		t.Errorf("expected Bob row, got %s", result)
	}
}

func TestRenderHTML_EachEmpty(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["users"] = []database.Row{}

	result := renderHTML(`{{each users}}<li>{name}</li>{{end}}`, ctx)
	if result != "" {
		t.Errorf("expected empty string for empty query, got %s", result)
	}
}

func TestRenderHTML_EachElse(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["users"] = []database.Row{}

	result := renderHTML(`{{each users}}<li>{name}</li>{{else}}<p>No users</p>{{end}}`, ctx)
	if !strings.Contains(result, "No users") {
		t.Errorf("expected else block, got %s", result)
	}
}

func TestRenderHTML_EachElseNotShownWhenData(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["users"] = []database.Row{{"name": "Alice"}}

	result := renderHTML(`{{each users}}<li>{name}</li>{{else}}<p>No users</p>{{end}}`, ctx)
	if strings.Contains(result, "No users") {
		t.Errorf("else block should not show when data exists: %s", result)
	}
	if !strings.Contains(result, "Alice") {
		t.Errorf("expected Alice, got %s", result)
	}
}

func TestRenderHTML_EachQueryNotFound(t *testing.T) {
	ctx := newTestContext()

	result := renderHTML(`{{each missing}}<li>{name}</li>{{else}}<p>Empty</p>{{end}}`, ctx)
	if !strings.Contains(result, "Empty") {
		t.Errorf("expected else block for missing query, got %s", result)
	}
}

func TestRenderHTML_EachXSSEscaping(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["users"] = []database.Row{{"name": `<img src=x onerror=alert(1)>`}}

	result := renderHTML(`{{each users}}<td>{name}</td>{{end}}`, ctx)
	if strings.Contains(result, "<img") {
		t.Errorf("XSS: img tag not escaped inside each: %s", result)
	}
}

func TestRenderHTML_EachCrossQueryAccess(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["users"] = []database.Row{{"name": "Alice"}}
	ctx.queries["stats"] = []database.Row{{"total": "42"}}

	result := renderHTML(`{{each users}}<li>{name} of {stats.total}</li>{{end}}`, ctx)
	if !strings.Contains(result, "Alice of 42") {
		t.Errorf("expected cross-query access, got %s", result)
	}
}

func TestRenderHTML_MultipleEachBlocks(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["contacts"] = []database.Row{{"name": "Alice"}}
	ctx.queries["companies"] = []database.Row{{"name": "Acme"}}

	result := renderHTML(`{{each contacts}}<li>{name}</li>{{end}} | {{each companies}}<li>{name}</li>{{end}}`, ctx)
	if !strings.Contains(result, "Alice") || !strings.Contains(result, "Acme") {
		t.Errorf("expected both blocks rendered, got %s", result)
	}
}

func TestRenderHTML_IfEquals(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["deal"] = []database.Row{{"stage": "closed_won"}}

	result := renderHTML(`{{if deal.stage == "closed_won"}}<span>Won!</span>{{end}}`, ctx)
	if !strings.Contains(result, "Won!") {
		t.Errorf("expected Won!, got %s", result)
	}
}

func TestRenderHTML_IfNotEquals(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["deal"] = []database.Row{{"stage": "lead"}}

	result := renderHTML(`{{if deal.stage == "closed_won"}}<span>Won!</span>{{end}}`, ctx)
	if strings.Contains(result, "Won!") {
		t.Errorf("should not show Won! for lead stage: %s", result)
	}
}

func TestRenderHTML_IfElse(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["deal"] = []database.Row{{"stage": "lead"}}

	result := renderHTML(`{{if deal.stage == "closed_won"}}<span>Won!</span>{{else}}<span>Open</span>{{end}}`, ctx)
	if !strings.Contains(result, "Open") {
		t.Errorf("expected else block, got %s", result)
	}
}

func TestRenderHTML_IfCountGreaterThan(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["users"] = []database.Row{{"name": "A"}, {"name": "B"}}

	result := renderHTML(`{{if users.count > 0}}<p>Has users</p>{{else}}<p>No users</p>{{end}}`, ctx)
	if !strings.Contains(result, "Has users") {
		t.Errorf("expected 'Has users', got %s", result)
	}
}

func TestRenderHTML_IfCountZero(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["users"] = []database.Row{}

	result := renderHTML(`{{if users.count > 0}}<p>Has users</p>{{else}}<p>No users</p>{{end}}`, ctx)
	if !strings.Contains(result, "No users") {
		t.Errorf("expected 'No users', got %s", result)
	}
}

func TestRenderHTML_IfInsideEach(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["deals"] = []database.Row{
		{"title": "Won deal", "stage": "closed_won"},
		{"title": "Lost deal", "stage": "closed_lost"},
	}

	result := renderHTML(`{{each deals}}{{if stage == "closed_won"}}<span class="green">{title}</span>{{else}}<span class="red">{title}</span>{{end}}{{end}}`, ctx)
	if !strings.Contains(result, `class="green">Won deal`) {
		t.Errorf("expected green Won deal, got %s", result)
	}
	if !strings.Contains(result, `class="red">Lost deal`) {
		t.Errorf("expected red Lost deal, got %s", result)
	}
}

func TestRenderHTML_RawFilter(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["post"] = []database.Row{{"content": "<b>Bold text</b>"}}

	result := renderHTML(`<div>{post.content | raw}</div>`, ctx)
	if !strings.Contains(result, "<b>Bold text</b>") {
		t.Errorf("raw filter should not escape HTML, got %s", result)
	}
}

func TestRenderHTML_RawFilterVsNormal(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["post"] = []database.Row{{"content": "<b>Bold</b>", "title": "<b>Title</b>"}}

	result := renderHTML(`{post.content | raw} | {post.title}`, ctx)
	if !strings.Contains(result, "<b>Bold</b>") {
		t.Errorf("raw should contain unescaped HTML: %s", result)
	}
	if !strings.Contains(result, "&lt;b&gt;Title&lt;/b&gt;") {
		t.Errorf("normal should contain escaped HTML: %s", result)
	}
}

func TestRenderHTML_Pagination(t *testing.T) {
	ctx := newTestContext()
	ctx.paginate["users"] = PaginateInfo{
		Page:    2,
		PerPage: 10,
		Total:   55,
		HasPrev: true,
		HasNext: true,
	}

	result := renderHTML(`Page {paginate.users.page} of {paginate.users.total_pages}`, ctx)
	if !strings.Contains(result, "Page 2 of 6") {
		t.Errorf("expected 'Page 2 of 6', got %s", result)
	}
}

func TestRenderHTML_PaginationHasNext(t *testing.T) {
	ctx := newTestContext()
	ctx.paginate["items"] = PaginateInfo{Page: 1, PerPage: 10, Total: 25, HasPrev: false, HasNext: true}

	result := renderHTML(`{{if paginate.items.has_next == "true"}}<a href="?page={paginate.items.next}">Next</a>{{end}}`, ctx)
	if !strings.Contains(result, `href="?page=2"`) {
		t.Errorf("expected next link with page=2, got %s", result)
	}
}

func TestRenderHTML_QueryParams(t *testing.T) {
	ctx := newTestContext()
	ctx.queryParams["filter"] = "active"
	ctx.queryParams["sort"] = "name"

	result := renderHTML(`Filter: {params.filter}, Sort: {params.sort}`, ctx)
	if !strings.Contains(result, "Filter: active") {
		t.Errorf("expected 'Filter: active', got %s", result)
	}
	if !strings.Contains(result, "Sort: name") {
		t.Errorf("expected 'Sort: name', got %s", result)
	}
}

func TestRenderHTML_CurrentUser(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["current_user"] = []database.Row{{"name": "Admin", "role": "admin"}}

	result := renderHTML(`{{if current_user.role == "admin"}}<button>Delete</button>{{end}}`, ctx)
	if !strings.Contains(result, "<button>Delete</button>") {
		t.Errorf("expected admin button, got %s", result)
	}
}

func TestRenderHTML_CurrentUserNotAdmin(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["current_user"] = []database.Row{{"name": "User", "role": "viewer"}}

	result := renderHTML(`{{if current_user.role == "admin"}}<button>Delete</button>{{end}}`, ctx)
	if strings.Contains(result, "<button>") {
		t.Errorf("viewer should not see admin button: %s", result)
	}
}

func TestRenderHTML_NoChangesForPlainHTML(t *testing.T) {
	ctx := newTestContext()
	result := renderHTML(`<div class="card"><p>Hello world</p></div>`, ctx)
	if result != `<div class="card"><p>Hello world</p></div>` {
		t.Errorf("plain HTML should not be modified: %s", result)
	}
}

func TestRenderHTML_UnresolvedTokensLeftAlone(t *testing.T) {
	ctx := newTestContext()
	result := renderHTML(`<p>{unknown.field}</p>`, ctx)
	if !strings.Contains(result, "{unknown.field}") {
		t.Errorf("unresolved tokens should be left unchanged: %s", result)
	}
}

func TestRenderHTML_IfTruthyCheck(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"name": "Alice"}}

	result := renderHTML(`{{if user.name}}<p>Has name</p>{{else}}<p>No name</p>{{end}}`, ctx)
	if !strings.Contains(result, "Has name") {
		t.Errorf("truthy check should pass for non-empty value, got %s", result)
	}
}

func TestRenderHTML_IfTruthyCheckEmpty(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"name": ""}}

	result := renderHTML(`{{if user.name}}<p>Has name</p>{{else}}<p>No name</p>{{end}}`, ctx)
	if !strings.Contains(result, "No name") {
		t.Errorf("truthy check should fail for empty value, got %s", result)
	}
}

func TestRenderHTML_MultipleCSRFTokensUnique(t *testing.T) {
	ctx := newTestContext()
	result := renderHTML(`<input value="{csrf}"><input value="{csrf}">`, ctx)

	// Extract both tokens
	parts := strings.Split(result, `value="`)
	if len(parts) < 3 {
		t.Fatalf("expected 2 tokens in output, got %s", result)
	}
	token1 := parts[1][:strings.Index(parts[1], `"`)]
	token2 := parts[2][:strings.Index(parts[2], `"`)]

	if token1 == token2 {
		t.Errorf("multiple {csrf} should generate unique tokens, both are %s", token1)
	}
	if !validateCSRFToken(token1) {
		t.Errorf("first csrf token failed validation")
	}
	if !validateCSRFToken(token2) {
		t.Errorf("second csrf token failed validation")
	}
}

func TestRenderHTML_EachElseInterpolation(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["items"] = []database.Row{}
	ctx.queries["info"] = []database.Row{{"category": "Books"}}

	result := renderHTML(`{{each items}}<li>{name}</li>{{else}}<p>No {info.category} found</p>{{end}}`, ctx)
	if !strings.Contains(result, "No Books found") {
		t.Errorf("else body should interpolate cross-query values, got %s", result)
	}
}

func TestRenderHTML_NestedEachBlocks(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["companies"] = []database.Row{{"name": "Acme"}, {"name": "Corp"}}
	ctx.queries["people"] = []database.Row{{"name": "Alice"}, {"name": "Bob"}}

	result := renderHTML(`{{each companies}}<h2>{name}</h2>{{each people}}<li>{name}</li>{{end}}{{end}}`, ctx)
	if !strings.Contains(result, "<h2>Acme</h2>") || !strings.Contains(result, "<h2>Corp</h2>") {
		t.Errorf("outer each not rendered: %s", result)
	}
	if !strings.Contains(result, "<li>Alice</li>") {
		t.Errorf("inner each not rendered: %s", result)
	}
}

func TestRenderHTML_IfWithNotEquals(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"role": "admin"}}

	result := renderHTML(`{{if user.role != "viewer"}}<p>Not viewer</p>{{end}}`, ctx)
	if !strings.Contains(result, "Not viewer") {
		t.Errorf("!= should work, got %s", result)
	}
}

func TestRenderHTML_IfWithQuotedOperator(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["item"] = []database.Row{{"label": "a>=b"}}

	result := renderHTML(`{{if item.label == "a>=b"}}<p>Match</p>{{end}}`, ctx)
	if !strings.Contains(result, "Match") {
		t.Errorf("operator inside quotes should not split condition, got %s", result)
	}
}

func TestRenderHTML_RawPlaceholderCollision(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["post"] = []database.Row{{"content": "<b>Bold</b>", "title": "__KILNX_RAW_old_0__"}}

	result := renderHTML(`{post.content | raw} | {post.title}`, ctx)
	if !strings.Contains(result, "<b>Bold</b>") {
		t.Errorf("raw should work: %s", result)
	}
	// The title should be escaped and not interfere with raw placeholders
	escaped := html.EscapeString("__KILNX_RAW_old_0__")
	if !strings.Contains(result, escaped) {
		t.Errorf("title with placeholder-like value should be escaped: %s", result)
	}
}

func TestRenderHTML_MalformedTemplateMissingEnd(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["users"] = []database.Row{{"name": "Alice"}}

	result := renderHTML(`{{each users}}<li>{name}</li>`, ctx)
	// Should not panic, should output the raw template
	if !strings.Contains(result, "{{each users}}") {
		t.Errorf("malformed template should be output as-is, got %s", result)
	}
}

func TestRenderHTML_EachInsideIf(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["users"] = []database.Row{{"name": "Alice"}, {"name": "Bob"}}

	result := renderHTML(`{{if users.count > 0}}{{each users}}<li>{name}</li>{{end}}{{end}}`, ctx)
	if !strings.Contains(result, "Alice") || !strings.Contains(result, "Bob") {
		t.Errorf("each inside if should work, got %s", result)
	}
}

// --- Logical operator tests (and/or) ---

func TestRenderHTML_IfAnd_BothTrue(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"role": "admin", "active": "1"}}

	result := renderHTML(`{{if user.role == "admin" and user.active}}<p>Admin active</p>{{end}}`, ctx)
	if !strings.Contains(result, "Admin active") {
		t.Errorf("both conditions true, expected 'Admin active', got %s", result)
	}
}

func TestRenderHTML_IfAnd_OneFalse(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"role": "viewer", "active": "1"}}

	result := renderHTML(`{{if user.role == "admin" and user.active}}<p>Admin active</p>{{end}}`, ctx)
	if strings.Contains(result, "Admin active") {
		t.Errorf("first condition false, should not render, got %s", result)
	}
}

func TestRenderHTML_IfOr_OneTrue(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"role": "admin"}}

	result := renderHTML(`{{if user.role == "admin" or user.role == "editor"}}<p>Can edit</p>{{end}}`, ctx)
	if !strings.Contains(result, "Can edit") {
		t.Errorf("first condition true, expected 'Can edit', got %s", result)
	}
}

func TestRenderHTML_IfOr_BothFalse(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"role": "viewer"}}

	result := renderHTML(`{{if user.role == "admin" or user.role == "editor"}}<p>Can edit</p>{{end}}`, ctx)
	if strings.Contains(result, "Can edit") {
		t.Errorf("both conditions false, should not render, got %s", result)
	}
}

func TestRenderHTML_IfAndOr_Combined(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"role": "editor", "active": "1"}}

	// "or" has lower precedence: (role == admin) or (role == editor and active)
	result := renderHTML(`{{if user.role == "admin" or user.role == "editor" and user.active}}<p>OK</p>{{end}}`, ctx)
	if !strings.Contains(result, "OK") {
		t.Errorf("expected OK for editor+active, got %s", result)
	}
}

func TestRenderHTML_IfAnd_InsideEach(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["deals"] = []database.Row{
		{"stage": "closed_won", "value": "1000"},
		{"stage": "lead", "value": "500"},
	}

	result := renderHTML(`{{each deals}}{{if stage == "closed_won" and value > 0}}<span>Big win: {value}</span>{{end}}{{end}}`, ctx)
	if !strings.Contains(result, "Big win: 1000") {
		t.Errorf("expected 'Big win: 1000', got %s", result)
	}
	if strings.Contains(result, "Big win: 500") {
		t.Errorf("lead should not match, got %s", result)
	}
}

func TestRenderHTML_IfOr_WithElse(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"role": "viewer"}}

	result := renderHTML(`{{if user.role == "admin" or user.role == "editor"}}<p>Privileged</p>{{else}}<p>Basic</p>{{end}}`, ctx)
	if !strings.Contains(result, "Basic") {
		t.Errorf("expected 'Basic' for viewer, got %s", result)
	}
}

func TestRenderHTML_IfAndQuotedString(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["item"] = []database.Row{{"name": "foo and bar", "active": "1"}}

	// "and" inside a quoted string should NOT be treated as logical operator
	result := renderHTML(`{{if item.name == "foo and bar"}}<p>Match</p>{{end}}`, ctx)
	if !strings.Contains(result, "Match") {
		t.Errorf("'and' inside quotes should not split condition, got %s", result)
	}
}
