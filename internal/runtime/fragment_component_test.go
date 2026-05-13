package runtime

import (
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

func TestExpandFragmentCallsBasic(t *testing.T) {
	badge := &parser.Page{
		Path:         "badge",
		FragmentArgs: []parser.FragmentArg{{Name: "status"}},
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<span class="badge">{status}</span>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"badge": badge},
	}
	content := `{{badge status=active}}`
	got := expandFragmentCalls(content, ctx)
	want := `<span class="badge">active</span>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandFragmentCallsWithDefault(t *testing.T) {
	money := &parser.Page{
		Path: "money",
		FragmentArgs: []parser.FragmentArg{
			{Name: "amount"},
			{Name: "currency", HasDefault: true, DefaultValue: "R$"},
		},
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<span>{currency} {amount}</span>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"money": money},
	}
	content := `{{money amount=100}}`
	got := expandFragmentCalls(content, ctx)
	want := `<span>R$ 100</span>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandFragmentCallsUnknownComponent(t *testing.T) {
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{},
	}
	content := `{{unknown status=active}}`
	got := expandFragmentCalls(content, ctx)
	if got != content {
		t.Errorf("expected unchanged for unknown component, got %q", got)
	}
}

func TestExpandFragmentCallsRecursionDepth(t *testing.T) {
	badge := &parser.Page{
		Path:         "badge",
		FragmentArgs: []parser.FragmentArg{{Name: "status"}},
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<span>{status}</span>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"badge": badge},
		fragmentDepth:      2,
	}
	content := `{{badge status=active}}`
	got := expandFragmentCalls(content, ctx)
	if got != content {
		t.Errorf("expected unchanged at depth 2, got %q", got)
	}
}

func TestExpandFragmentCallsNestedComponent(t *testing.T) {
	inner := &parser.Page{
		Path:         "inner",
		FragmentArgs: []parser.FragmentArg{{Name: "val"}},
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<b>{val}</b>`},
		},
	}
	outer := &parser.Page{
		Path:         "outer",
		FragmentArgs: []parser.FragmentArg{{Name: "text"}},
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<div>{{inner val=text}}</div>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{
			"inner": inner,
			"outer": outer,
		},
	}
	content := `{{outer text=hello}}`
	got := expandFragmentCalls(content, ctx)
	want := `<div><b>hello</b></div>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestExpandFragmentCallsQuotedSpaces verifies that quoted argument values
// containing spaces are tokenized as a single value (not split on spaces).
// Regression test for splitArgStr replacing strings.Split(argStr, " ").
func TestExpandFragmentCallsQuotedSpaces(t *testing.T) {
	card := &parser.Page{
		Path: "card",
		FragmentArgs: []parser.FragmentArg{
			{Name: "title"},
			{Name: "count"},
		},
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<div><h2>{title}</h2><span>{count}</span></div>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"card": card},
	}
	content := `{{card title="Hello World" count=5}}`
	got := expandFragmentCalls(content, ctx)
	want := `<div><h2>Hello World</h2><span>5</span></div>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestExpandFragmentCallsSingleQuotedSpaces ensures single quotes work too.
func TestExpandFragmentCallsSingleQuotedSpaces(t *testing.T) {
	card := &parser.Page{
		Path:         "card",
		FragmentArgs: []parser.FragmentArg{{Name: "title"}},
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<h2>{title}</h2>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"card": card},
	}
	content := `{{card title='Hello World'}}`
	got := expandFragmentCalls(content, ctx)
	want := `<h2>Hello World</h2>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestRenderHTMLNoFragmentReexpansionOfRowData verifies the security fix:
// when an each-block iterates rows and a row column value contains
// a literal {{badge ...}} string, that string must NOT be re-interpreted
// as a fragment call after row interpolation. (Template injection guard.)
func TestRenderHTMLNoFragmentReexpansionOfRowData(t *testing.T) {
	badge := &parser.Page{
		Path:         "badge",
		FragmentArgs: []parser.FragmentArg{{Name: "status"}},
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<span class="badge">{status}</span>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"badge": badge},
		queries: map[string][]database.Row{
			"items": {
				{"name": "{{badge status=active}}"},
			},
		},
		paginate:          map[string]PaginateInfo{},
		querySourceModels: map[string]string{},
		customManifests:   map[string]*parser.CustomFieldManifest{},
		queryParams:       map[string]string{},
	}
	content := `{{each items}}<li>{name}</li>{{end}}`
	got := renderHTML(content, ctx)
	// The row's name field must appear as escaped literal text, not as
	// an expanded badge component.
	if strings.Contains(got, `<span class="badge">`) {
		t.Errorf("row data was incorrectly expanded as fragment call. got: %q", got)
	}
	// Should contain the escaped literal braces
	if !strings.Contains(got, "{{badge") {
		t.Errorf("expected literal text containing '{{badge', got: %q", got)
	}
}

// TestExpandFragmentCallsOutsideEachDuplicateMatchOrder verifies that when an
// identical fragment-call string appears both inside and outside an {{each}}
// block, the outside occurrence is still expanded. Regression for first-match
// position bug in expandFragmentCallsOutsideEach.
func TestExpandFragmentCallsOutsideEachDuplicateMatchOrder(t *testing.T) {
	badge := &parser.Page{
		Path:         "badge",
		FragmentArgs: []parser.FragmentArg{{Name: "status", HasDefault: true, DefaultValue: "active"}},
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<span class="badge">{status}</span>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"badge": badge},
	}
	// inside-each occurrence textually precedes outside-each occurrence.
	content := `{{each items}}{{badge status=active}}{{end}}{{badge status=active}}`
	got := expandFragmentCallsOutsideEach(content, ctx)
	// inside-each should remain literal; outside should expand exactly once.
	if !strings.Contains(got, `{{each items}}{{badge status=active}}{{end}}`) {
		t.Errorf("inside-each occurrence must remain literal. got: %q", got)
	}
	// Trailing call must be expanded.
	if !strings.HasSuffix(got, `<span class="badge">active</span>`) {
		t.Errorf("outside-each occurrence must be expanded. got: %q", got)
	}
}

// TestExpandFragmentCallsEmptyStringDefault verifies that fragment(label="")
// is treated as having a default (empty string), not as required.
// Uses HasDefault flag rather than DefaultValue == "".
func TestExpandFragmentCallsEmptyStringDefault(t *testing.T) {
	badge := &parser.Page{
		Path: "badge",
		FragmentArgs: []parser.FragmentArg{
			{Name: "label", HasDefault: true, DefaultValue: ""},
		},
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<span>[{label}]</span>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"badge": badge},
	}
	// Call without providing label - default empty string should apply.
	content := `{{badge}}`
	got := expandFragmentCalls(content, ctx)
	want := `<span>[]</span>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestServerReloadRebuildsFragmentComponents verifies that Server.Reload
// rebuilds the fragmentComponents index so newly added components are
// visible to the renderer after hot-reload.
func TestServerReloadRebuildsFragmentComponents(t *testing.T) {
	// Initial app has no component fragments.
	initialApp := &parser.App{}
	srv := NewServer(initialApp, nil, 0)
	if len(srv.fragmentComponents) != 0 {
		t.Fatalf("expected empty fragmentComponents, got %d", len(srv.fragmentComponents))
	}

	// New app introduces a component fragment.
	newApp := &parser.App{
		Fragments: []parser.Page{
			{
				Path:         "badge",
				FragmentArgs: []parser.FragmentArg{{Name: "status"}},
				Body: []parser.Node{
					{Type: parser.NodeHTML, HTMLContent: `<span>{status}</span>`},
				},
			},
		},
	}
	srv.Reload(newApp)

	// fragmentComponents must contain the new component.
	if _, ok := srv.fragmentComponents["badge"]; !ok {
		t.Fatalf("Reload did not register 'badge' in fragmentComponents")
	}

	// And the renderer must actually expand it via the server's index.
	ctx := &renderContext{fragmentComponents: srv.fragmentComponents}
	got := expandFragmentCalls(`{{badge status=active}}`, ctx)
	want := `<span>active</span>`
	if got != want {
		t.Errorf("after Reload, expansion got %q, want %q", got, want)
	}

	// And the server's standard renderHTML path picks it up.
	got2 := renderHTML(`<div>{{badge status=ok}}</div>`, &renderContext{
		fragmentComponents: srv.fragmentComponents,
		queries:            map[string][]database.Row{},
		paginate:           map[string]PaginateInfo{},
		querySourceModels:  map[string]string{},
		customManifests:    map[string]*parser.CustomFieldManifest{},
		queryParams:        map[string]string{},
	})
	if !strings.Contains(got2, `<span>ok</span>`) {
		t.Errorf("renderHTML after Reload did not expand badge. got %q", got2)
	}
}

func TestExpandFragmentCallsWithQueryValue(t *testing.T) {
	badge := &parser.Page{
		Path:         "badge",
		FragmentArgs: []parser.FragmentArg{{Name: "status"}},
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<span>{status}</span>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"badge": badge},
		queries: map[string][]database.Row{
			"orders": {{"status": "shipped"}},
		},
	}
	content := `{{badge status=orders.status}}`
	got := expandFragmentCalls(content, ctx)
	want := `<span>shipped</span>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExpandFragmentCallsNilComponents(t *testing.T) {
	ctx := &renderContext{}
	content := `{{badge status=active}}`
	got := expandFragmentCalls(content, ctx)
	if got != content {
		t.Errorf("expected unchanged when components nil, got %q", got)
	}
}

func TestExpandFragmentCallsEmptyComponents(t *testing.T) {
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{},
	}
	content := `{{badge status=active}}`
	got := expandFragmentCalls(content, ctx)
	if got != content {
		t.Errorf("expected unchanged when components empty, got %q", got)
	}
}

func TestExpandFragmentCallsNoHTMLBody(t *testing.T) {
	badge := &parser.Page{
		Path:         "badge",
		FragmentArgs: []parser.FragmentArg{{Name: "status"}},
		Body:         []parser.Node{},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"badge": badge},
	}
	content := `{{badge status=active}}`
	got := expandFragmentCalls(content, ctx)
	if got != "" {
		t.Errorf("expected empty when no HTML body, got %q", got)
	}
}

func TestExpandFragmentCallsLiteralFallback(t *testing.T) {
	badge := &parser.Page{
		Path:         "badge",
		FragmentArgs: []parser.FragmentArg{{Name: "status"}},
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<span>{status}</span>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"badge": badge},
	}
	content := `{{badge status=unresolved_var}}`
	got := expandFragmentCalls(content, ctx)
	want := `<span>unresolved_var</span>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestExpandFragmentCallsQueryArg verifies that a fragment can accept a query
// name as an argument and iterate over it via {{each argname}} inside its body.
// This is the "Select with options from DB" pattern: the fragment is generic;
// the page picks which query feeds it.
func TestExpandFragmentCallsQueryArg(t *testing.T) {
	selectFrag := &parser.Page{
		Path: "select",
		FragmentArgs: []parser.FragmentArg{
			{Name: "name"},
			{Name: "options"},
		},
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<select name="{name}">{{each options}}<option value="{id}">{label}</option>{{end}}</select>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"select": selectFrag},
		queries: map[string][]database.Row{
			"roles": {
				{"id": "1", "label": "Admin"},
				{"id": "2", "label": "User"},
			},
		},
		paginate:          map[string]PaginateInfo{},
		querySourceModels: map[string]string{},
		customManifests:   map[string]*parser.CustomFieldManifest{},
		queryParams:       map[string]string{},
	}
	content := `{{select name="role" options=roles}}`
	got := renderHTML(content, ctx)
	want := `<select name="role"><option value="1">Admin</option><option value="2">User</option></select>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestExpandFragmentCallsQueryArgEmpty verifies that an empty query bound as
// a fragment arg produces an empty iteration (and the optional {{else}} branch
// inside the fragment body fires).
func TestExpandFragmentCallsQueryArgEmpty(t *testing.T) {
	listFrag := &parser.Page{
		Path: "list",
		FragmentArgs: []parser.FragmentArg{
			{Name: "items"},
		},
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<ul>{{each items}}<li>{name}</li>{{else}}<li class="empty">none</li>{{end}}</ul>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"list": listFrag},
		queries: map[string][]database.Row{
			"users": {},
		},
		paginate:          map[string]PaginateInfo{},
		querySourceModels: map[string]string{},
		customManifests:   map[string]*parser.CustomFieldManifest{},
		queryParams:       map[string]string{},
	}
	got := renderHTML(`{{list items=users}}`, ctx)
	want := `<ul><li class="empty">none</li></ul>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestExpandFragmentCallsQueryArgUnknownIdentFallsBackToScalar verifies that
// when a bare identifier arg does NOT match any query, it falls through to the
// existing scalar resolution path (treated as literal string), preserving the
// pre-feature behavior for plain values like color=red or status=active.
func TestExpandFragmentCallsQueryArgUnknownIdentFallsBackToScalar(t *testing.T) {
	badge := &parser.Page{
		Path:         "badge",
		FragmentArgs: []parser.FragmentArg{{Name: "color"}},
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<span>{color}</span>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"badge": badge},
		queries:            map[string][]database.Row{},
		paginate:           map[string]PaginateInfo{},
		querySourceModels:  map[string]string{},
		customManifests:    map[string]*parser.CustomFieldManifest{},
		queryParams:        map[string]string{},
	}
	got := renderHTML(`{{badge color=red}}`, ctx)
	want := `<span>red</span>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestExpandFragmentCallsQueryArgMixed verifies that scalar and query args can
// coexist on the same fragment call without one clobbering the other.
func TestExpandFragmentCallsQueryArgMixed(t *testing.T) {
	frag := &parser.Page{
		Path: "labeledlist",
		FragmentArgs: []parser.FragmentArg{
			{Name: "label"},
			{Name: "items"},
		},
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<div>{label}: <ul>{{each items}}<li>{name}</li>{{end}}</ul></div>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"labeledlist": frag},
		queries: map[string][]database.Row{
			"tags": {{"name": "go"}, {"name": "kilnx"}},
		},
		paginate:          map[string]PaginateInfo{},
		querySourceModels: map[string]string{},
		customManifests:   map[string]*parser.CustomFieldManifest{},
		queryParams:       map[string]string{},
	}
	got := renderHTML(`{{labeledlist label="Tags" items=tags}}`, ctx)
	want := `<div>Tags: <ul><li>go</li><li>kilnx</li></ul></div>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
