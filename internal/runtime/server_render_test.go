package runtime

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

// ---------- renderPage ----------

func TestRenderPage_WithQueryAndHTML(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT name FROM users WHERE id = :id`] = []database.Row{
		{"name": "Alice"},
	}
	s := newTestServer(mock)
	s.app.Pages = append(s.app.Pages, parser.Page{
		Path: "/profile",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT name FROM users WHERE id = :id`, Name: "user"},
			{Type: parser.NodeHTML, HTMLContent: `<h1>Hello {user.name}</h1>`},
		},
	})

	req := httptest.NewRequest("GET", "/profile?id=1", nil)
	page := findPageByPath(s.app.Pages, "/profile")
	if page == nil {
		t.Fatal("page not found")
	}
	got := s.renderPage(*page, s.app.Pages, req)
	if !strings.Contains(got, "Hello Alice") {
		t.Errorf("expected 'Hello Alice', got %q", got)
	}
}

func TestRenderPage_WithText(t *testing.T) {
	s := newTestServer(nil)
	s.app.Pages = append(s.app.Pages, parser.Page{
		Path:  "/about",
		Title: "About Us",
		Body: []parser.Node{
			{Type: parser.NodeText, Value: "We build things"},
		},
	})

	req := httptest.NewRequest("GET", "/about", nil)
	page := findPageByPath(s.app.Pages, "/about")
	if page == nil {
		t.Fatal("page not found")
	}
	got := s.renderPage(*page, s.app.Pages, req)
	if !strings.Contains(got, "<p>We build things</p>") {
		t.Errorf("expected text wrapped in p tag, got %q", got)
	}
	if !strings.Contains(got, "<title>About Us</title>") {
		t.Errorf("expected title in output, got %q", got)
	}
}

func TestRenderPage_NoDB(t *testing.T) {
	s := newTestServer(nil)
	s.app.Pages = append(s.app.Pages, parser.Page{
		Path: "/data",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT name FROM users`, Name: "users"},
			{Type: parser.NodeHTML, HTMLContent: `<p>{{each users}}{name}{{end}}</p>`},
		},
	})

	req := httptest.NewRequest("GET", "/data", nil)
	page := findPageByPath(s.app.Pages, "/data")
	if page == nil {
		t.Fatal("page not found")
	}
	got := s.renderPage(*page, s.app.Pages, req)
	// Should not panic; query is skipped when db is nil
	if !strings.Contains(got, "<p></p>") && !strings.Contains(got, "<p>") {
		t.Errorf("expected safe output without db, got %q", got)
	}
}

func TestRenderPage_WithLayout(t *testing.T) {
	s := newTestServer(nil)
	s.app.Layouts = []parser.Layout{
		{
			Name:        "main",
			HTMLContent: `<html><head><title>{page.title}</title></head><body>{nav}<main>{page.content}</main></body></html>`,
		},
	}
	s.app.Pages = append(s.app.Pages, parser.Page{
		Path:   "/dashboard",
		Title:  "Dashboard",
		Layout: "main",
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<h1>Dashboard</h1>`},
		},
	})

	req := httptest.NewRequest("GET", "/dashboard", nil)
	page := findPageByPath(s.app.Pages, "/dashboard")
	if page == nil {
		t.Fatal("page not found")
	}
	got := s.renderPage(*page, s.app.Pages, req)
	if !strings.Contains(got, "<title>Dashboard</title>") {
		t.Errorf("expected layout title, got %q", got)
	}
	if !strings.Contains(got, "<h1>Dashboard</h1>") {
		t.Errorf("expected page content, got %q", got)
	}
	if !strings.Contains(got, "<nav") {
		t.Errorf("expected nav in layout, got %q", got)
	}
}

func TestRenderPage_DefaultLayout(t *testing.T) {
	s := newTestServer(nil)
	s.app.Pages = append(s.app.Pages, parser.Page{
		Path:  "/simple",
		Title: "Simple",
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<p>Hello</p>`},
		},
	})

	req := httptest.NewRequest("GET", "/simple", nil)
	page := findPageByPath(s.app.Pages, "/simple")
	if page == nil {
		t.Fatal("page not found")
	}
	got := s.renderPage(*page, s.app.Pages, req)
	if !strings.Contains(got, "<!DOCTYPE html>") {
		t.Errorf("expected default layout HTML, got %q", got)
	}
	if !strings.Contains(got, "<title>Simple</title>") {
		t.Errorf("expected title, got %q", got)
	}
}

func TestRenderPage_WithCurrentUser(t *testing.T) {
	s := newTestServer(nil)
	sessionID := s.sessions.Create(database.Row{"id": "42", "email": "user@example.com", "role": "admin"}, "user@example.com")
	cookieValue := s.sessions.signSessionID(sessionID)
	s.app.Pages = append(s.app.Pages, parser.Page{
		Path: "/me",
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<p>Role: {current_user.role}</p>`},
		},
	})

	req := httptest.NewRequest("GET", "/me", nil)
	req.AddCookie(&http.Cookie{Name: "_kilnx_session", Value: cookieValue})
	page := findPageByPath(s.app.Pages, "/me")
	if page == nil {
		t.Fatal("page not found")
	}
	got := s.renderPage(*page, s.app.Pages, req)
	if !strings.Contains(got, "Role: admin") {
		t.Errorf("expected current user role, got %q", got)
	}
}

func TestRenderPage_WithQueryParams(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT name FROM users WHERE status = :status`] = []database.Row{
		{"name": "Active User"},
	}
	s := newTestServer(mock)
	s.app.Pages = append(s.app.Pages, parser.Page{
		Path: "/users",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT name FROM users WHERE status = :status`, Name: "users"},
			{Type: parser.NodeHTML, HTMLContent: `<p>First: {users.name}</p>`},
		},
	})

	req := httptest.NewRequest("GET", "/users?status=active", nil)
	page := findPageByPath(s.app.Pages, "/users")
	if page == nil {
		t.Fatal("page not found")
	}
	got := s.renderPage(*page, s.app.Pages, req)
	if !strings.Contains(got, "First: Active User") {
		t.Errorf("expected query param passed to SQL, got %q", got)
	}
}

func TestRenderPage_LayoutWithQueries(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsResults[`SELECT title FROM settings LIMIT 1`] = []database.Row{
		{"title": "MyApp"},
	}
	s := newTestServer(mock)
	s.app.Layouts = []parser.Layout{
		{
			Name:        "main",
			HTMLContent: `<html><body><header>{settings.title}</header>{page.content}</body></html>`,
			Queries: []parser.Node{
				{SQL: `SELECT title FROM settings LIMIT 1`, Name: "settings"},
			},
		},
	}
	s.app.Pages = append(s.app.Pages, parser.Page{
		Path:   "/with-layout-query",
		Layout: "main",
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<h1>Page</h1>`},
		},
	})

	req := httptest.NewRequest("GET", "/with-layout-query", nil)
	page := findPageByPath(s.app.Pages, "/with-layout-query")
	if page == nil {
		t.Fatal("page not found")
	}
	got := s.renderPage(*page, s.app.Pages, req)
	if !strings.Contains(got, "<header>MyApp</header>") {
		t.Errorf("expected layout query result, got %q", got)
	}
}

// ---------- renderWithLayout ----------

func TestRenderWithLayout_EachBlockInLayout(t *testing.T) {
	layout := parser.Layout{
		HTMLContent: `<html><body>{page.content}<ul>{{each items}}<li>{name}</li>{{end}}</ul></body></html>`,
	}
	ctx := &renderContext{
		queries: map[string][]database.Row{
			"items": {{"name": "A"}, {"name": "B"}},
		},
	}
	got := renderWithLayout(layout, "T", "", "<h1>X</h1>", ctx)
	if !strings.Contains(got, "<li>A</li>") {
		t.Errorf("expected A item, got %q", got)
	}
	if !strings.Contains(got, "<li>B</li>") {
		t.Errorf("expected B item, got %q", got)
	}
}

func TestRenderWithLayout_IfBlockInLayout(t *testing.T) {
	layout := parser.Layout{
		HTMLContent: `<html><body>{{if show_banner}}<div class="banner">Sale!</div>{{end}}{page.content}</body></html>`,
	}
	ctx := &renderContext{
		queries: map[string][]database.Row{
			"show_banner": {{"show_banner": "1"}},
		},
	}
	got := renderWithLayout(layout, "T", "", "<h1>X</h1>", ctx)
	if !strings.Contains(got, `<div class="banner">Sale!</div>`) {
		t.Errorf("expected banner, got %q", got)
	}
}

func TestRenderWithLayout_IfElseBlockInLayout(t *testing.T) {
	layout := parser.Layout{
		HTMLContent: `<html><body>{{if show_banner}}<div>On</div>{{else}}<div>Off</div>{{end}}{page.content}</body></html>`,
	}
	ctx := &renderContext{
		queries: map[string][]database.Row{
			"show_banner": {},
		},
	}
	got := renderWithLayout(layout, "T", "", "<h1>X</h1>", ctx)
	if !strings.Contains(got, "<div>Off</div>") {
		t.Errorf("expected else block, got %q", got)
	}
}

func TestRenderWithLayout_NilContext(t *testing.T) {
	layout := parser.Layout{
		HTMLContent: `<html><head><title>{page.title}</title></head><body>{page.content}</body></html>`,
	}
	got := renderWithLayout(layout, "Home", "", "<h1>Hello</h1>", nil)
	if !strings.Contains(got, "<title>Home</title>") {
		t.Errorf("expected title, got %q", got)
	}
	if !strings.Contains(got, "<h1>Hello</h1>") {
		t.Errorf("expected content, got %q", got)
	}
}

// ---------- renderHTML ----------

func TestRenderHTML_CSRFReplacementInForm(t *testing.T) {
	ctx := newTestContext()
	result := renderHTML(`<form><input type="hidden" name="_csrf" value="{csrf}"></form>`, ctx)
	if strings.Contains(result, "{csrf}") {
		t.Errorf("csrf token not replaced: %s", result)
	}
	if !strings.Contains(result, `name="_csrf"`) {
		t.Errorf("expected form field preserved: %s", result)
	}
}

func TestRenderHTML_EachBlockWithElseWhenEmpty(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["items"] = []database.Row{}
	result := renderHTML(`{{each items}}<li>{name}</li>{{else}}<p>No items found</p>{{end}}`, ctx)
	if !strings.Contains(result, "No items found") {
		t.Errorf("expected else block for empty query, got %s", result)
	}
	if strings.Contains(result, "<li>") {
		t.Errorf("did not expect list items, got %s", result)
	}
}

func TestRenderHTML_EachBlockWithElseWhenNotEmpty(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["items"] = []database.Row{{"name": "A"}}
	result := renderHTML(`{{each items}}<li>{name}</li>{{else}}<p>No items found</p>{{end}}`, ctx)
	if !strings.Contains(result, "<li>A</li>") {
		t.Errorf("expected item A, got %s", result)
	}
	if strings.Contains(result, "No items found") {
		t.Errorf("else block should not appear when data exists, got %s", result)
	}
}

func TestRenderHTML_IfBlockWithElse(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"active": "0"}}
	result := renderHTML(`{{if user.active == "1"}}<p>Active</p>{{else}}<p>Inactive</p>{{end}}`, ctx)
	if !strings.Contains(result, "Inactive") {
		t.Errorf("expected else block, got %s", result)
	}
}

func TestRenderHTML_IfBlockNestedInEach(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["items"] = []database.Row{
		{"name": "A", "status": "ok"},
		{"name": "B", "status": "fail"},
	}
	result := renderHTML(`{{each items}}{{if status == "ok"}}<span class="ok">{name}</span>{{else}}<span class="fail">{name}</span>{{end}}{{end}}`, ctx)
	if !strings.Contains(result, `<span class="ok">A</span>`) {
		t.Errorf("expected ok A, got %s", result)
	}
	if !strings.Contains(result, `<span class="fail">B</span>`) {
		t.Errorf("expected fail B, got %s", result)
	}
}

// ---------- interpolate ----------

func TestInterpolate_QueryField(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"name": "Alice"}}
	got := interpolate("Hello {user.name}", ctx)
	if got != "Hello Alice" {
		t.Errorf("expected 'Hello Alice', got %q", got)
	}
}

func TestInterpolate_QueryCount(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["items"] = []database.Row{{"id": "1"}, {"id": "2"}, {"id": "3"}}
	got := interpolate("Count: {items.count}", ctx)
	if got != "Count: 3" {
		t.Errorf("expected 'Count: 3', got %q", got)
	}
}

func TestInterpolate_CurrentUser(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["current_user"] = []database.Row{{"name": "Admin", "role": "admin"}}
	got := interpolate("User: {current_user.name}", ctx)
	if got != "User: Admin" {
		t.Errorf("expected 'User: Admin', got %q", got)
	}
}

func TestInterpolate_SingleNameSearch(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["users"] = []database.Row{{"name": "Bob"}}
	got := interpolate("Name: {name}", ctx)
	if got != "Name: Bob" {
		t.Errorf("expected 'Name: Bob', got %q", got)
	}
}

func TestInterpolate_UnresolvedToken(t *testing.T) {
	ctx := newTestContext()
	got := interpolate("Value: {missing.field}", ctx)
	if got != "Value: {missing.field}" {
		t.Errorf("expected unresolved token left alone, got %q", got)
	}
}

func TestInterpolate_NoQueries(t *testing.T) {
	ctx := newTestContext()
	got := interpolate("Hello world", ctx)
	if got != "Hello world" {
		t.Errorf("expected unchanged text, got %q", got)
	}
}

func TestInterpolate_MultipleReplacements(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{{"first": "Alice", "last": "Smith"}}
	got := interpolate("{user.first} {user.last}", ctx)
	if got != "Alice Smith" {
		t.Errorf("expected 'Alice Smith', got %q", got)
	}
}

func TestInterpolate_EmptyQueryReturnsEmpty(t *testing.T) {
	ctx := newTestContext()
	ctx.queries["user"] = []database.Row{}
	got := interpolate("Name: {user.name}", ctx)
	if got != "Name: " {
		t.Errorf("expected 'Name: ', got %q", got)
	}
}

// helpers

func findPageByPath(pages []parser.Page, path string) *parser.Page {
	for i := range pages {
		if pages[i].Path == path {
			return &pages[i]
		}
	}
	return nil
}

// ---------- renderFragmentWithParams ----------

func TestRenderFragmentWithParams_Query(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT name FROM users WHERE id = :id`] = []database.Row{
		{"name": "Alice"},
	}
	s := newTestServer(mock)
	frag := parser.Page{
		Path: "/frag",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT name FROM users WHERE id = :id`, Name: "user"},
			{Type: parser.NodeHTML, HTMLContent: `<p>Hello {user.name}</p>`},
		},
	}
	got := s.renderFragmentWithParams(frag, map[string]string{"id": "1"})
	if !strings.Contains(got, "Hello Alice") {
		t.Errorf("expected fragment with query result, got %q", got)
	}
}

func TestRenderFragmentWithParams_NoDB(t *testing.T) {
	s := newTestServer(nil)
	frag := parser.Page{
		Path: "/frag",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT name FROM users`, Name: "users"},
			{Type: parser.NodeHTML, HTMLContent: `<p>Hello</p>`},
		},
	}
	got := s.renderFragmentWithParams(frag, map[string]string{})
	if !strings.Contains(got, "Hello") {
		t.Errorf("expected fragment HTML, got %q", got)
	}
}

func TestRenderFragmentWithParams_TenantRejection(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	s.tenants = TenantMap{"users": "tenant_id"}
	frag := parser.Page{
		Path: "/frag",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT * FROM users`, Name: "users"},
		},
	}
	got := s.renderFragmentWithParams(frag, map[string]string{})
	if !strings.Contains(got, "Query rejected") {
		t.Errorf("expected tenant rejection message, got %q", got)
	}
}

func TestRenderFragmentWithParams_QueryError(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsErr[`SELECT name FROM users WHERE id = :id`] = fmt.Errorf("db error")
	s := newTestServer(mock)
	frag := parser.Page{
		Path: "/frag",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT name FROM users WHERE id = :id`, Name: "user"},
		},
	}
	got := s.renderFragmentWithParams(frag, map[string]string{"id": "1"})
	if !strings.Contains(got, "Query error") {
		t.Errorf("expected query error message, got %q", got)
	}
}

func TestRenderFragmentWithParams_TextNode(t *testing.T) {
	s := newTestServer(nil)
	frag := parser.Page{
		Path: "/frag",
		Body: []parser.Node{
			{Type: parser.NodeText, Value: "Hello world"},
		},
	}
	got := s.renderFragmentWithParams(frag, map[string]string{})
	if !strings.Contains(got, "<p>Hello world</p>") {
		t.Errorf("expected text wrapped in p tag, got %q", got)
	}
}

func TestRenderFragmentWithParams_CustomFields(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT title, custom FROM deals WHERE id = :id`] = []database.Row{
		{"title": "Deal A", "custom": `{"revenue":"500"}`},
	}
	s := newTestServer(mock)
	s.app.Models = []parser.Model{{
		Name:             "deal",
		CustomFieldsFile: "deal.json",
		Fields: []parser.Field{
			{Name: "title", Type: parser.FieldText},
		},
	}}
	s.app.CustomManifests = map[string]*parser.CustomFieldManifest{
		"deal": {
			ModelName: "deal",
			Fields: []parser.CustomFieldDef{
				{Name: "revenue", Kind: parser.CustomFieldKindNumber, Label: "Revenue"},
			},
		},
	}
	frag := parser.Page{
		Path: "/frag",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT title, custom FROM deals WHERE id = :id`, Name: "deal", SourceModel: "deal"},
			{Type: parser.NodeHTML, HTMLContent: `<p>{deal.title}: {deal.custom.revenue}</p>`},
		},
	}
	got := s.renderFragmentWithParams(frag, map[string]string{"id": "1"})
	if !strings.Contains(got, "Deal A: 500") {
		t.Errorf("expected custom field expansion, got %q", got)
	}
}

func TestRenderPage_WithPagination(t *testing.T) {
	mock := newMockExecutor()
	countSQL := `SELECT COUNT(*) as _count FROM (SELECT id, name FROM users)`
	dataSQL := `SELECT id, name FROM users LIMIT 10 OFFSET 10`
	mock.queryRowsWithParamsResults[countSQL] = []database.Row{{"_count": "25"}}
	mock.queryRowsWithParamsResults[countSQL+"|page=2"] = []database.Row{{"_count": "25"}}
	mock.queryRowsWithParamsResults[dataSQL] = []database.Row{
		{"id": "3", "name": "Carol"},
		{"id": "4", "name": "Dave"},
	}
	mock.queryRowsWithParamsResults[dataSQL+"|page=2"] = []database.Row{
		{"id": "3", "name": "Carol"},
		{"id": "4", "name": "Dave"},
	}
	s := newTestServer(mock)
	s.app.Pages = append(s.app.Pages, parser.Page{
		Path: "/users",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT id, name FROM users`, Name: "users", Paginate: 10},
			{Type: parser.NodeHTML, HTMLContent: `<p>Page {paginate.users.page} of {paginate.users.total_pages}</p>`},
		},
	})

	req := httptest.NewRequest("GET", "/users?page=2", nil)
	page := findPageByPath(s.app.Pages, "/users")
	if page == nil {
		t.Fatal("page not found")
	}
	got := s.renderPage(*page, s.app.Pages, req)
	if !strings.Contains(got, "Page 2 of 3") {
		t.Errorf("expected pagination info, got %q", got)
	}
}

func TestRenderPage_WithCustomFields(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT title, custom FROM deals WHERE id = :id`] = []database.Row{
		{"title": "Deal A", "custom": `{"revenue":"500"}`},
	}
	s := newTestServer(mock)
	s.app.Models = []parser.Model{{
		Name:             "deal",
		CustomFieldsFile: "deal.json",
		Fields:           []parser.Field{{Name: "title", Type: parser.FieldText}},
	}}
	s.app.CustomManifests = map[string]*parser.CustomFieldManifest{
		"deal": {
			ModelName: "deal",
			Fields:    []parser.CustomFieldDef{{Name: "revenue", Kind: parser.CustomFieldKindNumber, Label: "Revenue"}},
		},
	}
	s.app.Pages = append(s.app.Pages, parser.Page{
		Path: "/deal",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT title, custom FROM deals WHERE id = :id`, Name: "deal", SourceModel: "deal"},
			{Type: parser.NodeHTML, HTMLContent: `<p>{deal.title}: {deal.custom.revenue}</p>`},
		},
	})

	req := httptest.NewRequest("GET", "/deal?id=1", nil)
	page := findPageByPath(s.app.Pages, "/deal")
	if page == nil {
		t.Fatal("page not found")
	}
	got := s.renderPage(*page, s.app.Pages, req)
	if !strings.Contains(got, "Deal A: 500") {
		t.Errorf("expected custom field expansion, got %q", got)
	}
}

func TestRenderPage_WithFetch(t *testing.T) {
	s := newTestServer(nil)
	s.app.Pages = append(s.app.Pages, parser.Page{
		Path: "/external",
		Body: []parser.Node{
			{Type: parser.NodeFetch, Name: "data", FetchURL: "https://api.example.com/data"},
			{Type: parser.NodeHTML, HTMLContent: `<p>{data.status}</p>`},
		},
	})

	req := httptest.NewRequest("GET", "/external", nil)
	page := findPageByPath(s.app.Pages, "/external")
	if page == nil {
		t.Fatal("page not found")
	}
	// No db and no real fetch — should not panic
	got := s.renderPage(*page, s.app.Pages, req)
	if !strings.Contains(got, "{data.status}") {
		t.Errorf("expected unresolved token, got %q", got)
	}
}

func TestRenderPage_QueryNoName(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsResults[`SELECT name FROM users`] = []database.Row{
		{"name": "Alice"},
	}
	s := newTestServer(mock)
	s.app.Pages = append(s.app.Pages, parser.Page{
		Path: "/users",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT name FROM users`},
			{Type: parser.NodeHTML, HTMLContent: `<p>{_last.name}</p>`},
		},
	})

	req := httptest.NewRequest("GET", "/users", nil)
	page := findPageByPath(s.app.Pages, "/users")
	if page == nil {
		t.Fatal("page not found")
	}
	got := s.renderPage(*page, s.app.Pages, req)
	if !strings.Contains(got, "Alice") {
		t.Errorf("expected Alice using _last, got %q", got)
	}
}

func TestRenderPage_QueryEmptyResult(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT name FROM users WHERE id = :id`] = []database.Row{}
	s := newTestServer(mock)
	s.app.Pages = append(s.app.Pages, parser.Page{
		Path: "/user",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT name FROM users WHERE id = :id`, Name: "user"},
			{Type: parser.NodeHTML, HTMLContent: `<p>Hello {user.name}</p>`},
		},
	})

	req := httptest.NewRequest("GET", "/user?id=999", nil)
	page := findPageByPath(s.app.Pages, "/user")
	if page == nil {
		t.Fatal("page not found")
	}
	got := s.renderPage(*page, s.app.Pages, req)
	if !strings.Contains(got, "Hello ") {
		t.Errorf("expected empty name, got %q", got)
	}
}

func TestRenderPage_CustomFieldsNoManifest(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT title, custom FROM deals WHERE id = :id`] = []database.Row{
		{"title": "Deal A", "custom": `{"revenue":"500"}`},
	}
	s := newTestServer(mock)
	s.app.Models = []parser.Model{{
		Name:             "deal",
		CustomFieldsFile: "deal.json",
		Fields:           []parser.Field{{Name: "title", Type: parser.FieldText}},
	}}
	// No manifest registered
	s.app.Pages = append(s.app.Pages, parser.Page{
		Path: "/deal",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT title, custom FROM deals WHERE id = :id`, Name: "deal", SourceModel: "deal"},
			{Type: parser.NodeHTML, HTMLContent: `<p>{deal.title}</p>`},
		},
	})

	req := httptest.NewRequest("GET", "/deal?id=1", nil)
	page := findPageByPath(s.app.Pages, "/deal")
	if page == nil {
		t.Fatal("page not found")
	}
	got := s.renderPage(*page, s.app.Pages, req)
	if !strings.Contains(got, "Deal A") {
		t.Errorf("expected Deal A, got %q", got)
	}
}

func TestRenderPage_CustomFieldsEmptyRows(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT title, custom FROM deals WHERE id = :id`] = []database.Row{}
	s := newTestServer(mock)
	s.app.Models = []parser.Model{{
		Name:             "deal",
		CustomFieldsFile: "deal.json",
		Fields:           []parser.Field{{Name: "title", Type: parser.FieldText}},
	}}
	s.app.CustomManifests = map[string]*parser.CustomFieldManifest{
		"deal": {
			ModelName: "deal",
			Fields:    []parser.CustomFieldDef{{Name: "revenue", Kind: parser.CustomFieldKindNumber, Label: "Revenue"}},
		},
	}
	s.app.Pages = append(s.app.Pages, parser.Page{
		Path: "/deal",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT title, custom FROM deals WHERE id = :id`, Name: "deal", SourceModel: "deal"},
			{Type: parser.NodeHTML, HTMLContent: `<p>{deal.title}</p>`},
		},
	})

	req := httptest.NewRequest("GET", "/deal?id=1", nil)
	page := findPageByPath(s.app.Pages, "/deal")
	if page == nil {
		t.Fatal("page not found")
	}
	got := s.renderPage(*page, s.app.Pages, req)
	if !strings.Contains(got, "<p></p>") && !strings.Contains(got, "{deal.title}") {
		t.Errorf("expected empty or unresolved, got %q", got)
	}
}

func TestRenderFragment_CustomFieldsWithManifest(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT title, custom FROM deals WHERE id = :id`] = []database.Row{
		{"title": "Deal A", "custom": `{"revenue":"500"}`},
	}
	s := newTestServer(mock)
	s.app.Models = []parser.Model{{
		Name:             "deal",
		CustomFieldsFile: "deal.json",
		Fields:           []parser.Field{{Name: "title", Type: parser.FieldText}},
	}}
	s.app.CustomManifests = map[string]*parser.CustomFieldManifest{
		"deal": {
			ModelName: "deal",
			Fields:    []parser.CustomFieldDef{{Name: "revenue", Kind: parser.CustomFieldKindNumber, Label: "Revenue"}},
		},
	}
	frag := parser.Page{
		Path: "/frag/:id",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT title, custom FROM deals WHERE id = :id`, Name: "deal", SourceModel: "deal"},
			{Type: parser.NodeHTML, HTMLContent: `<p>{deal.title}: {deal.custom.revenue}</p>`},
		},
	}
	req := httptest.NewRequest("GET", "/frag/1", nil)
	got := s.renderFragment(frag, req)
	if !strings.Contains(got, "Deal A: 500") {
		t.Errorf("expected custom field expansion, got %q", got)
	}
}

func TestRenderFragment_CustomFieldsNoManifest(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT title, custom FROM deals WHERE id = :id`] = []database.Row{
		{"title": "Deal A", "custom": `{"revenue":"500"}`},
	}
	s := newTestServer(mock)
	s.app.Models = []parser.Model{{
		Name:             "deal",
		CustomFieldsFile: "deal.json",
		Fields:           []parser.Field{{Name: "title", Type: parser.FieldText}},
	}}
	// No manifest registered
	frag := parser.Page{
		Path: "/frag/:id",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT title, custom FROM deals WHERE id = :id`, Name: "deal", SourceModel: "deal"},
			{Type: parser.NodeHTML, HTMLContent: `<p>{deal.title}</p>`},
		},
	}
	req := httptest.NewRequest("GET", "/frag/1", nil)
	got := s.renderFragment(frag, req)
	if !strings.Contains(got, "Deal A") {
		t.Errorf("expected Deal A, got %q", got)
	}
}
