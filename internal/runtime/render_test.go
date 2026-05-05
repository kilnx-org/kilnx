package runtime

import (
	"html"
	"strings"
	"testing"
	"time"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

func newTestContext() *renderContext {
	return &renderContext{
		queries:           make(map[string][]database.Row),
		paginate:          make(map[string]PaginateInfo),
		queryParams:       make(map[string]string),
		querySourceModels: make(map[string]string),
		customManifests:   make(map[string]*parser.CustomFieldManifest),
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

func TestRenderHTML_NestedEachParentScope(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["companies"] = []database.Row{{"id": "1", "name": "Acme"}, {"id": "2", "name": "Corp"}}
	ctx.queries["people"] = []database.Row{{"id": "10", "name": "Alice"}, {"id": "11", "name": "Bob"}}

	result := renderHTML(`{{each companies}}<div id="c{id}"><h2>{name}</h2>{{each people}}<li data-cid="{^id}" data-pid="{id}">{name}</li>{{end}}</div>{{end}}`, ctx)
	if !strings.Contains(result, `<div id="c1">`) {
		t.Errorf("expected outer id c1, got: %s", result)
	}
	if !strings.Contains(result, `<li data-cid="1" data-pid="10">`) {
		t.Errorf("expected parent scope ^id=1 and inner id=10, got: %s", result)
	}
	if !strings.Contains(result, `<li data-cid="2" data-pid="11">`) {
		t.Errorf("expected parent scope ^id=2 and inner id=11, got: %s", result)
	}
	if strings.Contains(result, "{^id}") {
		t.Errorf("unresolved parent scope ref leaked: %s", result)
	}
}

func TestRenderHTML_NestedEachParentScopeMultiLevel(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["a"] = []database.Row{{"name": "A1"}}
	ctx.queries["b"] = []database.Row{{"name": "B1"}}
	ctx.queries["c"] = []database.Row{{"name": "C1"}}

	result := renderHTML(`{{each a}}<a>{name}</a>{{each b}}<b>{name}</b>{{each c}}<c>{name}|{^^name}|{^name}</c>{{end}}{{end}}{{end}}`, ctx)
	expected := "<c>C1|A1|B1</c>"
	if !strings.Contains(result, expected) {
		t.Errorf("expected multi-level parent scope %s, got: %s", expected, result)
	}
}

func TestRenderHTML_NestedEachParentScopeNotFound(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["outer"] = []database.Row{{"id": "1"}}
	ctx.queries["inner"] = []database.Row{{"id": "10"}}

	result := renderHTML(`{{each outer}}<o>{id}</o>{{each inner}}<i>{^id}</i><x>{^nonexistent}</x>{{end}}{{end}}`, ctx)
	if !strings.Contains(result, "<o>1</o>") {
		t.Errorf("expected outer id, got: %s", result)
	}
	if !strings.Contains(result, "<i>1</i>") {
		t.Errorf("expected parent scope ^id=1, got: %s", result)
	}
	if !strings.Contains(result, "{^nonexistent}") {
		t.Errorf("expected unresolved parent ref to stay literal, got: %s", result)
	}
}

func TestRenderHTML_NestedEachParentScopeWithFilter(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["companies"] = []database.Row{{"id": "1", "name": "Acme"}}
	ctx.queries["people"] = []database.Row{{"id": "10", "name": "Alice"}}

	result := renderHTML(`{{each companies}}{{each people}}<span>{^name | upcase} {name | upcase}</span>{{end}}{{end}}`, ctx)
	if !strings.Contains(result, "<span>ACME ALICE</span>") {
		t.Errorf("expected filtered parent scope ACME ALICE, got: %s", result)
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

func TestExpandCustomFields_DirectAccess(t *testing.T) {
	rows := []database.Row{
		{"title": "Deal A", "custom": `{"revenue":"500","region":"S"}`},
	}
	rows = expandCustomFields(rows)
	if rows[0]["custom.revenue"] != "500" {
		t.Errorf("expected custom.revenue='500', got %q", rows[0]["custom.revenue"])
	}
	if rows[0]["custom.region"] != "S" {
		t.Errorf("expected custom.region='S', got %q", rows[0]["custom.region"])
	}
}

func TestExpandCustomFields_NoCustomColumn(t *testing.T) {
	rows := []database.Row{{"title": "Deal B"}}
	rows = expandCustomFields(rows)
	if _, ok := rows[0]["custom.anything"]; ok {
		t.Error("should not expand if no custom column")
	}
}

func TestExpandCustomFields_TemplateResolution(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["d"] = expandCustomFields([]database.Row{
		{"title": "Deal A", "custom": `{"revenue":"500"}`},
	})
	result := renderHTML("<span>{d.custom.revenue}</span>", ctx)
	if !strings.Contains(result, "500") {
		t.Errorf("expected 500 in output, got %s", result)
	}
}

func TestSerializeCustomBrackets_Basic(t *testing.T) {
	data := map[string]string{
		"title":           "Deal A",
		"custom[revenue]": "500",
		"custom[region]":  "S",
	}
	serializeCustomBrackets(data)
	if _, ok := data["custom[revenue]"]; ok {
		t.Error("bracket key should be removed")
	}
	customJSON := data["custom"]
	if !strings.Contains(customJSON, `"revenue"`) || !strings.Contains(customJSON, `"500"`) {
		t.Errorf("expected revenue in custom JSON, got %q", customJSON)
	}
	if !strings.Contains(customJSON, `"region"`) || !strings.Contains(customJSON, `"S"`) {
		t.Errorf("expected region in custom JSON, got %q", customJSON)
	}
	if data["title"] != "Deal A" {
		t.Error("non-custom keys must not be removed")
	}
}

func TestSerializeCustomBrackets_MergeExisting(t *testing.T) {
	data := map[string]string{
		"custom":          `{"notes":"abc"}`,
		"custom[revenue]": "999",
	}
	serializeCustomBrackets(data)
	customJSON := data["custom"]
	if !strings.Contains(customJSON, `"notes"`) {
		t.Errorf("existing custom key must be preserved, got %q", customJSON)
	}
	if !strings.Contains(customJSON, `"revenue"`) {
		t.Errorf("new key must be merged, got %q", customJSON)
	}
}

func TestSerializeCustomBrackets_NoBrackets(t *testing.T) {
	data := map[string]string{"title": "X", "value": "1"}
	serializeCustomBrackets(data)
	if _, ok := data["custom"]; ok {
		t.Error("custom key must not be created when no bracket keys exist")
	}
}

func TestBuildCustomIterRows_WithManifest(t *testing.T) {
	manifest := &parser.CustomFieldManifest{
		ModelName: "deal",
		Fields: []parser.CustomFieldDef{
			{Name: "revenue", Kind: parser.CustomFieldKindNumber, Label: "Receita"},
			{Name: "region", Kind: parser.CustomFieldKindOption, Label: "Região"},
		},
	}
	row := database.Row{"custom": `{"revenue":"500","region":"S"}`}
	result := buildCustomIterRows(row, manifest)
	if len(result) != 2 {
		t.Fatalf("expected 2 synthetic rows, got %d", len(result))
	}
	if result[0]["name"] != "revenue" || result[0]["value"] != "500" || result[0]["label"] != "Receita" {
		t.Errorf("first row wrong: %v", result[0])
	}
	if result[1]["name"] != "region" || result[1]["value"] != "S" || result[1]["label"] != "Região" {
		t.Errorf("second row wrong: %v", result[1])
	}
}

func TestBuildCustomIterRows_EmptyCustomColumn(t *testing.T) {
	manifest := &parser.CustomFieldManifest{
		ModelName: "deal",
		Fields:    []parser.CustomFieldDef{{Name: "revenue", Kind: parser.CustomFieldKindNumber, Label: "Revenue"}},
	}
	row := database.Row{"title": "No custom data"}
	result := buildCustomIterRows(row, manifest)
	if len(result) != 1 {
		t.Fatalf("expected 1 row (empty value), got %d", len(result))
	}
	if result[0]["value"] != "" {
		t.Errorf("expected empty value, got %q", result[0]["value"])
	}
}

// TestEachCustomRebindPerRow verifies that {{each q.custom}} inside {{each q}}
// shows each row's own custom fields, not always row[0] (#66).
func TestEachCustomRebindPerRow(t *testing.T) {
	manifest := &parser.CustomFieldManifest{
		ModelName: "deal",
		Fields: []parser.CustomFieldDef{
			{Name: "revenue", Kind: parser.CustomFieldKindNumber, Label: "Revenue"},
		},
	}
	ctx := newTestContext()
	ctx.querySourceModels["d"] = "deal"
	ctx.customManifests["deal"] = manifest
	ctx.queries["d"] = []database.Row{
		{"title": "Deal A", "custom": `{"revenue":"100"}`},
		{"title": "Deal B", "custom": `{"revenue":"200"}`},
	}
	// Pre-populate with row[0] data (as server.go does at query time)
	ctx.queries["d.custom"] = buildCustomIterRows(ctx.queries["d"][0], manifest)

	// Inside {{each d}}, bare {title} uses current row; {d.title} always uses row[0]
	tmpl := `{{each d}}<div>{title}:{{each d.custom}}{value}{{end}}</div>{{end}}`
	result := renderHTML(tmpl, ctx)

	if !strings.Contains(result, "Deal A:100") {
		t.Errorf("expected Deal A:100 in output, got: %s", result)
	}
	if !strings.Contains(result, "Deal B:200") {
		t.Errorf("expected Deal B:200 in output, got: %s", result)
	}
}

// --- goDateFormat tests ---

func TestGoDateFormat_YMD(t *testing.T) {
	got := goDateFormat("%Y-%m-%d")
	if got != "2006-01-02" {
		t.Errorf("expected 2006-01-02, got %q", got)
	}
}

func TestGoDateFormat_HMS(t *testing.T) {
	got := goDateFormat("%H:%M:%S")
	if got != "15:04:05" {
		t.Errorf("expected 15:04:05, got %q", got)
	}

}

func TestGoDateFormat_Full(t *testing.T) {
	got := goDateFormat("%B %d, %Y at %I:%M %p")
	if got != "January 02, 2006 at 03:04 PM" {
		t.Errorf("expected full format, got %q", got)
	}
}

// --- timeAgo tests ---

func TestTimeAgo_JustNow(t *testing.T) {
	got := timeAgo(time.Now().Add(-5 * time.Second))
	if got != "just now" {
		t.Errorf("expected just now, got %q", got)
	}
}

func TestTimeAgo_Minutes(t *testing.T) {
	got := timeAgo(time.Now().Add(-5 * time.Minute))
	if got != "5 minutes ago" {
		t.Errorf("expected 5 minutes ago, got %q", got)
	}
}

func TestTimeAgo_OneMinute(t *testing.T) {
	got := timeAgo(time.Now().Add(-1 * time.Minute))
	if got != "1 minute ago" {
		t.Errorf("expected 1 minute ago, got %q", got)
	}
}

func TestTimeAgo_Hours(t *testing.T) {
	got := timeAgo(time.Now().Add(-3 * time.Hour))
	if got != "3 hours ago" {
		t.Errorf("expected 3 hours ago, got %q", got)
	}
}

func TestTimeAgo_OneHour(t *testing.T) {
	got := timeAgo(time.Now().Add(-1 * time.Hour))
	if got != "1 hour ago" {
		t.Errorf("expected 1 hour ago, got %q", got)
	}
}

func TestTimeAgo_Days(t *testing.T) {
	got := timeAgo(time.Now().Add(-5 * 24 * time.Hour))
	if got != "5 days ago" {
		t.Errorf("expected 5 days ago, got %q", got)
	}
}

func TestTimeAgo_OneDay(t *testing.T) {
	got := timeAgo(time.Now().Add(-24 * time.Hour))
	if got != "1 day ago" {
		t.Errorf("expected 1 day ago, got %q", got)
	}
}

func TestTimeAgo_Months(t *testing.T) {
	got := timeAgo(time.Now().Add(-100 * 24 * time.Hour))
	if got != "3 months ago" {
		t.Errorf("expected 3 months ago, got %q", got)
	}
}

func TestTimeAgo_OneMonth(t *testing.T) {
	got := timeAgo(time.Now().Add(-45 * 24 * time.Hour))
	if got != "1 month ago" {
		t.Errorf("expected 1 month ago, got %q", got)
	}
}

// --- formatNumber tests ---

func TestFormatNumber_Integer(t *testing.T) {
	got := formatNumber(1234567, 0)
	if got != "1,234,567" {
		t.Errorf("expected 1,234,567, got %q", got)
	}
}

func TestFormatNumber_WithDecimals(t *testing.T) {
	got := formatNumber(1234.567, 2)
	if got != "1,234.57" {
		t.Errorf("expected 1,234.57, got %q", got)
	}
}

func TestFormatNumber_Negative(t *testing.T) {
	got := formatNumber(-1234.5, 1)
	if got != "-1,234.5" {
		t.Errorf("expected -1,234.5, got %q", got)
	}
}

func TestFormatNumber_Small(t *testing.T) {
	got := formatNumber(42, 0)
	if got != "42" {
		t.Errorf("expected 42, got %q", got)
	}
}

func TestFormatNumber_Zero(t *testing.T) {
	got := formatNumber(0, 0)
	if got != "0" {
		t.Errorf("expected 0, got %q", got)
	}
}

func TestFormatNumber_ZeroDecimals(t *testing.T) {
	got := formatNumber(0, 2)
	if got != "0.00" {
		t.Errorf("expected 0.00, got %q", got)
	}
}

// --- evaluateCondition / evaluateSingleCondition tests ---

func TestEvaluateCondition_TruthyCheck(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"name": "Alice"}}
	if !evaluateCondition("user.name", ctx, nil) {
		t.Error("expected truthy for non-empty value")
	}
}

func TestEvaluateCondition_TruthyCheckEmpty(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"name": ""}}
	if evaluateCondition("user.name", ctx, nil) {
		t.Error("expected falsy for empty value")
	}
}

func TestEvaluateCondition_TruthyCheckZero(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"score": "0"}}
	if evaluateCondition("user.score", ctx, nil) {
		t.Error("expected falsy for zero value")
	}
}

func TestEvaluateCondition_Equals(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"role": "admin"}}
	if !evaluateCondition(`user.role == "admin"`, ctx, nil) {
		t.Error("expected true for equal values")
	}
	if evaluateCondition(`user.role == "viewer"`, ctx, nil) {
		t.Error("expected false for non-equal values")
	}
}

func TestEvaluateCondition_NotEquals(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"role": "admin"}}
	if !evaluateCondition(`user.role != "viewer"`, ctx, nil) {
		t.Error("expected true for not-equal values")
	}
}

func TestEvaluateCondition_GreaterThan(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["items"] = []database.Row{{"score": "5"}}
	if !evaluateCondition("items.score > 3", ctx, nil) {
		t.Error("expected true for 5 > 3")
	}
	if evaluateCondition("items.score > 10", ctx, nil) {
		t.Error("expected false for 5 > 10")
	}
}

func TestEvaluateCondition_LessThan(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["items"] = []database.Row{{"count": "5"}}
	if !evaluateCondition("items.count < 10", ctx, nil) {
		t.Error("expected true for 5 < 10")
	}
}

func TestEvaluateCondition_And(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"role": "admin", "active": "1"}}
	if !evaluateCondition(`user.role == "admin" and user.active`, ctx, nil) {
		t.Error("expected true for both conditions")
	}
	if evaluateCondition(`user.role == "admin" and user.active == "0"`, ctx, nil) {
		t.Error("expected false for second condition false")
	}
}

func TestEvaluateCondition_Or(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"role": "editor"}}
	if !evaluateCondition(`user.role == "admin" or user.role == "editor"`, ctx, nil) {
		t.Error("expected true for one condition true")
	}
	if evaluateCondition(`user.role == "admin" or user.role == "viewer"`, ctx, nil) {
		t.Error("expected false for both conditions false")
	}
}

func TestEvaluateCondition_UnresolvedVariable(t *testing.T) {
	ctx := newTestContext()
	if evaluateCondition("missing.field", ctx, nil) {
		t.Error("expected false for unresolved variable")
	}
}

func TestEvaluateCondition_VariableComparison(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["a"] = []database.Row{{"val": "5"}}
	ctx.queries["b"] = []database.Row{{"val": "5"}}
	if !evaluateCondition("a.val == b.val", ctx, nil) {
		t.Error("expected true when comparing resolved variables")
	}
}

// --- splitCondition tests ---

func TestSplitCondition_Equals(t *testing.T) {
	left, op, right := splitCondition(`user.role == "admin"`)
	if left != `user.role` || op != "==" || right != `"admin"` {
		t.Errorf("unexpected split: %q %q %q", left, op, right)
	}
}

func TestSplitCondition_NotEquals(t *testing.T) {
	left, op, right := splitCondition(`user.role != "viewer"`)
	if left != `user.role` || op != "!=" || right != `"viewer"` {
		t.Errorf("unexpected split: %q %q %q", left, op, right)
	}
}

func TestSplitCondition_GreaterThan(t *testing.T) {
	left, op, right := splitCondition("items.count > 3")
	if left != "items.count" || op != ">" || right != "3" {
		t.Errorf("unexpected split: %q %q %q", left, op, right)
	}
}

func TestSplitCondition_LessThan(t *testing.T) {
	left, op, right := splitCondition("items.count < 10")
	if left != "items.count" || op != "<" || right != "10" {
		t.Errorf("unexpected split: %q %q %q", left, op, right)
	}
}

func TestSplitCondition_GreaterThanOrEqual(t *testing.T) {
	left, op, right := splitCondition("score >= 50")
	if left != "score" || op != ">=" || right != "50" {
		t.Errorf("unexpected split: %q %q %q", left, op, right)
	}
}

func TestSplitCondition_LessThanOrEqual(t *testing.T) {
	left, op, right := splitCondition("score <= 100")
	if left != "score" || op != "<=" || right != "100" {
		t.Errorf("unexpected split: %q %q %q", left, op, right)
	}
}

func TestSplitCondition_OperatorInQuotes(t *testing.T) {
	left, op, right := splitCondition(`label == "a>=b"`)
	if left != `label` || op != "==" || right != `"a>=b"` {
		t.Errorf("unexpected split: %q %q %q", left, op, right)
	}
}

func TestSplitCondition_NoOperator(t *testing.T) {
	left, op, right := splitCondition("user.name")
	if left != "user.name" || op != "" || right != "" {
		t.Errorf("unexpected split: %q %q %q", left, op, right)
	}
}

func TestSplitCondition_SingleQuote(t *testing.T) {
	left, op, right := splitCondition(`name == 'Alice'`)
	if left != "name" || op != "==" || right != `'Alice'` {
		// Note: single quotes are not stripped by splitCondition
		t.Errorf("unexpected split: %q %q %q", left, op, right)
	}
}

// --- stripQuotes tests ---

func TestStripQuotes_Double(t *testing.T) {
	got := stripQuotes(`"hello"`)
	if got != "hello" {
		t.Errorf("expected hello, got %q", got)
	}
}

func TestStripQuotes_Single(t *testing.T) {
	got := stripQuotes(`'hello'`)
	if got != "hello" {
		t.Errorf("expected hello, got %q", got)
	}
}

func TestStripQuotes_NoQuotes(t *testing.T) {
	got := stripQuotes("hello")
	if got != "hello" {
		t.Errorf("expected hello, got %q", got)
	}
}

func TestStripQuotes_Empty(t *testing.T) {
	got := stripQuotes(``)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestStripQuotes_SingleChar(t *testing.T) {
	got := stripQuotes(`a`)
	if got != "a" {
		t.Errorf("expected a, got %q", got)
	}
}

func TestSplitCondition_MixedQuotes(t *testing.T) {
	left, op, right := splitCondition(`name == "Alice"`)
	if left != `name` || op != "==" || right != `"Alice"` {
		t.Errorf("unexpected split: %q %q %q", left, op, right)
	}
}

func TestSplitCondition_EscapedQuote(t *testing.T) {
	// Backslash-escaped quotes should not confuse the scanner
	left, op, right := splitCondition(`label == "a\"b"`)
	if left != `label` || op != "==" || right != `"a\"b"` {
		t.Errorf("unexpected split: %q %q %q", left, op, right)
	}
}

func TestSplitCondition_SingleQuoteNested(t *testing.T) {
	left, op, right := splitCondition(`name == 'O\'Brien'`)
	if left != `name` || op != "==" || right != `'O\'Brien'` {
		t.Errorf("unexpected split: %q %q %q", left, op, right)
	}
}

func TestSplitCondition_OperatorAfterQuote(t *testing.T) {
	left, op, right := splitCondition(`status >= 'active'`)
	if left != `status` || op != ">=" || right != `'active'` {
		t.Errorf("unexpected split: %q %q %q", left, op, right)
	}
}

func TestEvaluateCondition_GreaterThanOrEqual(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["users"] = []database.Row{{"score": "10"}}
	if !evaluateCondition("users.score >= 5", ctx, nil) {
		t.Error("expected true for 10 >= 5")
	}
	if evaluateCondition("users.score >= 15", ctx, nil) {
		t.Error("expected false for 10 >= 15")
	}
}

func TestEvaluateCondition_LessThanOrEqual(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["users"] = []database.Row{{"score": "10"}}
	if !evaluateCondition("users.score <= 15", ctx, nil) {
		t.Error("expected true for 10 <= 15")
	}
	if evaluateCondition("users.score <= 5", ctx, nil) {
		t.Error("expected false for 10 <= 5")
	}
}

func TestEvaluateCondition_NestedIf(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"role": "admin"}}
	template := `{{if user.role == "admin"}}admin{{if user.role == "super"}}super{{end}}content{{end}}`
	got := renderHTML(template, ctx)
	if got != "admincontent" {
		t.Errorf("expected 'admincontent', got %q", got)
	}
}

func TestEvaluateCondition_IfElse(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"role": "viewer"}}
	template := `{{if user.role == "admin"}}admin{{else}}viewer{{end}}`
	got := renderHTML(template, ctx)
	if got != "viewer" {
		t.Errorf("expected 'viewer', got %q", got)
	}
}

func TestRenderHTML_DefaultFilter(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"name": "", "bio": "<nil>"}}
	result := renderHTML(`{user.name | default:"Anonymous"}`, ctx)
	if result != "Anonymous" {
		t.Errorf("expected 'Anonymous', got %s", result)
	}
	result2 := renderHTML(`{user.bio | default:"No bio"}`, ctx)
	if result2 != "No bio" {
		t.Errorf("expected 'No bio' for <nil>, got %s", result2)
	}
}

func TestRenderHTML_CapitalizeFilter(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"name": "alice"}}
	result := renderHTML(`{user.name | capitalize}`, ctx)
	if result != "Alice" {
		t.Errorf("expected 'Alice', got %s", result)
	}
}

func TestRenderHTML_TruncateFilter(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["post"] = []database.Row{{"body": "This is a very long text that should be truncated"}}
	result := renderHTML(`{post.body | truncate:10}`, ctx)
	if !strings.Contains(result, "...") {
		t.Errorf("expected truncation with ..., got %s", result)
	}
}

func TestRenderHTML_DateFilter(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["event"] = []database.Row{{"created": "2024-03-15T10:30:00Z"}}
	result := renderHTML(`{event.created | date:"%Y-%m-%d"}`, ctx)
	if !strings.Contains(result, "2024-03-15") {
		t.Errorf("expected formatted date, got %s", result)
	}
}

func TestRenderHTML_TimeAgoFilter(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["post"] = []database.Row{{"created": time.Now().Add(-2 * time.Hour).Format(time.RFC3339)}}
	result := renderHTML(`{post.created | timeago}`, ctx)
	if !strings.Contains(result, "hour") {
		t.Errorf("expected time ago with hour, got %s", result)
	}
}

func TestRenderHTML_CurrencyFilter(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["order"] = []database.Row{{"total": "99.99"}}
	result := renderHTML(`{order.total | currency:"$"}`, ctx)
	if !strings.Contains(result, "$99.99") {
		t.Errorf("expected currency format, got %s", result)
	}
}

func TestRenderHTML_NumberFilter(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["stats"] = []database.Row{{"value": "3.14159"}}
	result := renderHTML(`{stats.value | number:2}`, ctx)
	if !strings.Contains(result, "3.14") {
		t.Errorf("expected number with 2 decimals, got %s", result)
	}
}

func TestRenderHTML_PluralizeFilter(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["cart"] = []database.Row{{"qty": "1"}}
	result := renderHTML(`{cart.qty | pluralize:"item,items"}`, ctx)
	if result != "item" {
		t.Errorf("expected singular 'item', got %s", result)
	}
	ctx.queries["cart2"] = []database.Row{{"qty": "5"}}
	result2 := renderHTML(`{cart2.qty | pluralize:"item,items"}`, ctx)
	if result2 != "items" {
		t.Errorf("expected plural 'items', got %s", result2)
	}
}

func TestRenderHTML_FilterInsideEach(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["users"] = []database.Row{{"name": "alice"}, {"name": "bob"}}
	result := renderHTML(`{{each users}}<span>{name | upcase}</span>{{end}}`, ctx)
	if !strings.Contains(result, "ALICE") || !strings.Contains(result, "BOB") {
		t.Errorf("expected uppercase names, got %s", result)
	}
}

func TestProcessRawInRow_Unresolved(t *testing.T) {
	ctx := newTestContext()
	row := database.Row{"name": "Alice"}
	// {missing} should remain unchanged
	result := processRawInRow(`<p>{missing | upcase}</p>`, row, ctx, "nonce1")
	if !strings.Contains(result, "{missing | upcase}") {
		t.Errorf("expected unresolved filter to remain, got %q", result)
	}
}

func TestFindMatchingEnd_Malformed(t *testing.T) {
	// Missing {{end}}
	body, elseBody, endPos := findMatchingEnd(`content {{if true}}inner`)
	if endPos != -1 {
		t.Errorf("expected -1 for malformed, got %d", endPos)
	}
	if body != "" || elseBody != "" {
		t.Error("expected empty body/elseBody for malformed")
	}
}

func TestExpandIfBlocks_MalformedNoClose(t *testing.T) {
	ctx := newTestContext()
	result := expandIfBlocks(`{{if true`, ctx, nil)
	if result != `{{if true` {
		t.Errorf("expected unchanged malformed, got %q", result)
	}
}

func TestExpandIfBlocks_MalformedNoEnd(t *testing.T) {
	ctx := newTestContext()
	result := expandIfBlocks(`{{if true}}content`, ctx, nil)
	if result != `{{if true}}content` {
		t.Errorf("expected unchanged malformed, got %q", result)
	}
}

func TestEvaluateSingleCondition_UnresolvedLeft(t *testing.T) {
	ctx := newTestContext()
	if evaluateSingleCondition("missing.field == 'test'", ctx, nil) {
		t.Error("expected false when left side unresolved")
	}
}

func TestEvaluateSingleCondition_UnknownOp(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["a"] = []database.Row{{"val": "1"}}
	if evaluateSingleCondition("a.val ~~ '1'", ctx, nil) {
		t.Error("expected false for unknown operator")
	}
}

func TestRenderHTML_UnresolvedFilter(t *testing.T) {
	ctx := newTestContext()
	result := renderHTML(`<p>{missing.field | upcase}</p>`, ctx)
	if !strings.Contains(result, "{missing.field | upcase}") {
		t.Errorf("expected unresolved filter to remain, got %q", result)
	}
}

func TestExpandEachBlocks_MalformedNoClose(t *testing.T) {
	ctx := newTestContext()
	result := expandEachBlocks(`{{each users`, ctx, "nonce")
	if result != `{{each users` {
		t.Errorf("expected unchanged malformed, got %q", result)
	}
}

func TestResolveValue_PaginateNotFound(t *testing.T) {
	ctx := newTestContext()
	result := resolveValue("paginate.missing.page", ctx, nil)
	if result != "{paginate.missing.page}" {
		t.Errorf("expected unresolved, got %q", result)
	}
}

func TestResolveValue_ParamsNotFound(t *testing.T) {
	ctx := newTestContext()
	ctx.queryParams = map[string]string{"other": "value"}
	result := resolveValue("params.missing", ctx, nil)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestIsInsideEachBlock_Malformed(t *testing.T) {
	fn := isInsideEachBlock(`{{each users`)
	// Should return false for any position since no valid each block
	if fn(0) {
		t.Error("expected false for malformed each block")
	}
}

func TestProcessRawInRow_RawFilter(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["post"] = []database.Row{{"content": "<b>Bold</b>"}}
	row := database.Row{"name": "Alice"}
	result := processRawInRow(`<div>{post.content | raw}</div>`, row, ctx, "nonce2")
	if !strings.Contains(result, "<b>Bold</b>") {
		t.Errorf("expected raw HTML to be preserved, got %q", result)
	}
}

func TestFindMatchingEnd_NoCloseTag(t *testing.T) {
	body, elseBody, endPos := findMatchingEnd(`content {{if true}}inner`)
	if endPos != -1 {
		t.Errorf("expected -1 for no close tag, got %d", endPos)
	}
	if body != "" || elseBody != "" {
		t.Error("expected empty body/elseBody")
	}
}

func TestFindMatchingEnd_MissingCloseTagInside(t *testing.T) {
	body, elseBody, endPos := findMatchingEnd(`content {{if true inner{{end}}`)
	if endPos != -1 {
		t.Errorf("expected -1 for missing close tag inside, got %d", endPos)
	}
	if body != "" || elseBody != "" {
		t.Error("expected empty body/elseBody")
	}
}

func TestFindMatchingEnd_LoopExhausted(t *testing.T) {
	// No {{end}} at all
	body, elseBody, endPos := findMatchingEnd(`content`)
	if endPos != -1 {
		t.Errorf("expected -1 when no end tag, got %d", endPos)
	}
	if body != "" || elseBody != "" {
		t.Error("expected empty body/elseBody")
	}
}

// ---------- inferHTTPVerb ----------

func TestInferHTTPVerb(t *testing.T) {
	tests := []struct {
		name   string
		action parser.Page
		want   string
	}{
		{"explicit GET", parser.Page{Method: "GET"}, "GET"},
		{"explicit post lowercase", parser.Page{Method: "post"}, "POST"},
		{"empty body defaults to POST", parser.Page{Body: []parser.Node{}}, "POST"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferHTTPVerb(&tt.action)
			if got != tt.want {
				t.Errorf("inferHTTPVerb() = %q, want %q", got, tt.want)
			}
		})
	}
}
