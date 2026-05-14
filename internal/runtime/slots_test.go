package runtime

import (
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

// TestSlotDefault verifies that block-form caller body fills the default
// {{slot}} marker in the fragment body.
func TestSlotDefault(t *testing.T) {
	card := &parser.Page{
		Path: "card",
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<div class="card">{{slot}}</div>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"card": card},
	}
	content := `{{card}}<p>hello</p>{{/card}}`
	got := expandFragmentCalls(content, ctx)
	want := `<div class="card"><p>hello</p></div>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestSlotDefaultWithFallback verifies that omitting the caller body falls
// back to content between {{slot}}fallback{{/slot}} in the fragment body.
func TestSlotDefaultWithFallback(t *testing.T) {
	card := &parser.Page{
		Path: "card",
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<div>{{slot}}fallback{{/slot}}</div>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"card": card},
	}
	got := expandFragmentCalls(`{{card}}`, ctx)
	want := `<div>fallback</div>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestSlotNamedAndDefault verifies that named slots are extracted from the
// caller body and matched to {{slot name="X"}} markers in the fragment, and
// that everything else in the caller body becomes the default slot.
func TestSlotNamedAndDefault(t *testing.T) {
	card := &parser.Page{
		Path: "card",
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<div><h1>{{slot name="header"}}</h1><main>{{slot}}</main></div>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"card": card},
	}
	content := `{{card}}{{slot name="header"}}Title{{/slot}}<p>body</p>{{/card}}`
	got := expandFragmentCalls(content, ctx)
	want := `<div><h1>Title</h1><main><p>body</p></main></div>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestSlotNamedFallback verifies that a named slot the caller did not fill
// falls back to the fragment's fallback content between markers.
func TestSlotNamedFallback(t *testing.T) {
	card := &parser.Page{
		Path: "card",
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<div><h1>{{slot name="header"}}Default Title{{/slot}}</h1><main>{{slot}}</main></div>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"card": card},
	}
	content := `{{card}}<p>body only</p>{{/card}}`
	got := expandFragmentCalls(content, ctx)
	want := `<div><h1>Default Title</h1><main><p>body only</p></main></div>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestSlotInsideEachItems verifies the headline use case: a generic kanban-ish
// fragment iterates items it received as a query/list arg, and the caller's
// default slot body supplies the per-item markup with row-field refs.
func TestSlotInsideEachItems(t *testing.T) {
	dvKanban := &parser.Page{
		Path: "DvKanban",
		FragmentArgs: []parser.FragmentArg{
			{Name: "items"},
		},
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<div class="kanban">{{each items}}<article>{{slot}}</article>{{end}}</div>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"DvKanban": dvKanban},
		queries: map[string][]database.Row{
			"people": {
				{"name": "Alice"},
				{"name": "Bob"},
			},
		},
		paginate:          map[string]PaginateInfo{},
		querySourceModels: map[string]string{},
		customManifests:   map[string]*parser.CustomFieldManifest{},
		queryParams:       map[string]string{},
	}
	content := `{{DvKanban items=people}}<h3>{name}</h3>{{/DvKanban}}`
	got := renderHTML(content, ctx)
	want := `<div class="kanban"><article><h3>Alice</h3></article><article><h3>Bob</h3></article></div>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestSlotEmptyCallerBody verifies that an empty caller body in block form is
// treated like the self-closing form: slot fallbacks survive.
func TestSlotEmptyCallerBody(t *testing.T) {
	card := &parser.Page{
		Path: "card",
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<div>{{slot}}fallback{{/slot}}</div>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"card": card},
	}
	got := expandFragmentCalls(`{{card}}{{/card}}`, ctx)
	want := `<div>fallback</div>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestSlotSelfClosingMarkerNoFallback verifies that a self-closing {{slot}}
// marker with no caller body and no fallback is removed entirely.
func TestSlotSelfClosingMarkerNoFallback(t *testing.T) {
	card := &parser.Page{
		Path: "card",
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<div>before{{slot}}after</div>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"card": card},
	}
	got := expandFragmentCalls(`{{card}}`, ctx)
	want := `<div>beforeafter</div>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestSlotBackwardsCompatSelfClosing verifies the existing self-closing form
// continues to work with fragments that have no slot markers.
func TestSlotBackwardsCompatSelfClosing(t *testing.T) {
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
	got := expandFragmentCalls(`{{badge status=active}}`, ctx)
	want := `<span class="badge">active</span>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestSlotNestedBlockForm verifies that block-form fragments can nest: an
// outer block-form call whose body contains another block-form call.
func TestSlotNestedBlockForm(t *testing.T) {
	box := &parser.Page{
		Path: "box",
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<div>{{slot}}</div>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"box": box},
		queries:            map[string][]database.Row{},
		paginate:           map[string]PaginateInfo{},
		querySourceModels:  map[string]string{},
		customManifests:    map[string]*parser.CustomFieldManifest{},
		queryParams:        map[string]string{},
	}
	content := `{{box}}{{box}}inner{{/box}}{{/box}}`
	got := renderHTML(content, ctx)
	want := `<div><div>inner</div></div>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestSlotBlockFormWithScalarArgs verifies that scalar args still flow into
// fragment body alongside slot content.
func TestSlotBlockFormWithScalarArgs(t *testing.T) {
	panel := &parser.Page{
		Path:         "panel",
		FragmentArgs: []parser.FragmentArg{{Name: "title"}},
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<section><h2>{title}</h2>{{slot}}</section>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"panel": panel},
	}
	content := `{{panel title="Hello"}}<p>body</p>{{/panel}}`
	got := expandFragmentCalls(content, ctx)
	want := `<section><h2>Hello</h2><p>body</p></section>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestSlotUnclosedTreatedAsSelfClosing verifies that if a block-form open tag
// has no matching close, the call is treated as self-closing (no crash, no
// runaway consumption).
func TestSlotUnclosedTreatedAsSelfClosing(t *testing.T) {
	card := &parser.Page{
		Path: "card",
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<div>{{slot}}fb{{/slot}}</div>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"card": card},
	}
	got := expandFragmentCalls(`{{card}}trailing`, ctx)
	want := `<div>fb</div>trailing`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestSlotRenderHTMLPipelineOutsideEach verifies the full renderHTML pipeline
// handles block-form fragments correctly outside any {{each}} block.
func TestSlotRenderHTMLPipelineOutsideEach(t *testing.T) {
	card := &parser.Page{
		Path: "card",
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<div class="card"><header>{{slot name="header"}}</header><main>{{slot}}</main></div>`},
		},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"card": card},
		queries:            map[string][]database.Row{},
		paginate:           map[string]PaginateInfo{},
		querySourceModels:  map[string]string{},
		customManifests:    map[string]*parser.CustomFieldManifest{},
		queryParams:        map[string]string{},
	}
	content := `<div>{{card}}{{slot name="header"}}Hi{{/slot}}<p>body</p>{{/card}}</div>`
	got := renderHTML(content, ctx)
	if !strings.Contains(got, `<header>Hi</header>`) {
		t.Errorf("named slot missing in output: %q", got)
	}
	if !strings.Contains(got, `<main><p>body</p></main>`) {
		t.Errorf("default slot missing in output: %q", got)
	}
}

// TestExtractSlots verifies the slot extractor unit-level.
func TestExtractSlots(t *testing.T) {
	body := `before{{slot name="header"}}H{{/slot}}between{{slot name="footer"}}F{{/slot}}after`
	named, def := extractSlots(body)
	if named["header"] != "H" {
		t.Errorf("named[header] = %q, want H", named["header"])
	}
	if named["footer"] != "F" {
		t.Errorf("named[footer] = %q, want F", named["footer"])
	}
	if def != "beforebetweenafter" {
		t.Errorf("default = %q, want beforebetweenafter", def)
	}
}

// TestSlotBlockFormInsideEach verifies the inside-each rendering path:
// caller is {{each users}}{{Frag}}...{{/Frag}}{{end}}, so each iteration
// invokes the block-form fragment with the current row visible to the slot
// body (refs like {name} should resolve against the each row, not the
// fragment).
func TestSlotBlockFormInsideEach(t *testing.T) {
	card := &parser.Page{
		Path: "Card",
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<div class="card">{{slot}}</div>`},
		},
		FragmentArgs: []parser.FragmentArg{},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"Card": card},
		queries: map[string][]database.Row{
			"users": {
				{"name": "Alice"},
				{"name": "Bob"},
			},
		},
		paginate:          map[string]PaginateInfo{},
		querySourceModels: map[string]string{},
		customManifests:   map[string]*parser.CustomFieldManifest{},
		queryParams:       map[string]string{},
	}
	content := `{{each users}}{{Card}}<p>{name}</p>{{/Card}}{{end}}`
	got := renderHTML(content, ctx)
	want := `<div class="card"><p>Alice</p></div><div class="card"><p>Bob</p></div>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestFindFragmentBlockEnd verifies the balanced scanner counts nested same-
// name opens correctly.
func TestFindFragmentBlockEnd(t *testing.T) {
	content := `{{box}}inner{{/box}}{{/box}}rest`
	body, end := findFragmentBlockEnd(content, "box")
	if body != `{{box}}inner{{/box}}` {
		t.Errorf("body = %q, want '{{box}}inner{{/box}}'", body)
	}
	if end < 0 || content[end:] != "rest" {
		t.Errorf("end = %d, content after = %q, want 'rest'", end, content[end:])
	}
}

// TestForwardNamedSlot covers the named-to-named forwarding path: outer fills
// breadcrumb+actions on the wrapper, the wrapper {{forward}}s them into a
// nested fragment call, and the inner fragment renders the forwarded content
// in its own named-slot placeholders.
func TestForwardNamedSlot(t *testing.T) {
	inner := &parser.Page{
		Path: "Inner",
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<div class="topbar"><span class="b">{{slot name="breadcrumb"}}{{/slot}}</span><span class="a">{{slot name="actions"}}{{/slot}}</span></div>`},
		},
		FragmentArgs: []parser.FragmentArg{},
	}
	outer := &parser.Page{
		Path: "Outer",
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `{{Inner}}{{forward name="breadcrumb"}}{{forward name="actions"}}{{/Inner}}`},
		},
		FragmentArgs: []parser.FragmentArg{},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"Inner": inner, "Outer": outer},
		queries:            map[string][]database.Row{},
		paginate:           map[string]PaginateInfo{},
		querySourceModels:  map[string]string{},
		customManifests:    map[string]*parser.CustomFieldManifest{},
		queryParams:        map[string]string{},
	}
	content := `{{Outer}}{{slot name="breadcrumb"}}<a>back</a>{{/slot}}{{slot name="actions"}}<btn>new</btn>{{/slot}}{{/Outer}}`
	got := renderHTML(content, ctx)
	want := `<div class="topbar"><span class="b"><a>back</a></span><span class="a"><btn>new</btn></span></div>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestForwardUnfilledFallsBackToChildFallback verifies that {{forward name="X"}}
// emits nothing when the outer caller did not provide X, letting the inner
// fragment use its own slot fallback rather than receiving an empty override.
func TestForwardUnfilledFallsBackToChildFallback(t *testing.T) {
	inner := &parser.Page{
		Path: "Inner",
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<header>{{slot name="title"}}default-title{{/slot}}</header>`},
		},
		FragmentArgs: []parser.FragmentArg{},
	}
	outer := &parser.Page{
		Path: "Outer",
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `{{Inner}}{{forward name="title"}}{{/Inner}}`},
		},
		FragmentArgs: []parser.FragmentArg{},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"Inner": inner, "Outer": outer},
		queries:            map[string][]database.Row{},
		paginate:           map[string]PaginateInfo{},
		querySourceModels:  map[string]string{},
		customManifests:    map[string]*parser.CustomFieldManifest{},
		queryParams:        map[string]string{},
	}
	content := `{{Outer}}{{/Outer}}`
	got := renderHTML(content, ctx)
	want := `<header>default-title</header>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestForwardNamelessIsStripped: {{forward}} without a name attribute is not
// a supported construct (default-to-default forwarding is covered by bare
// {{slot}}). The marker should be stripped, leaving no template residue.
func TestForwardNamelessIsStripped(t *testing.T) {
	inner := &parser.Page{
		Path: "Inner",
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<p>{{slot}}fb{{/slot}}</p>`},
		},
		FragmentArgs: []parser.FragmentArg{},
	}
	outer := &parser.Page{
		Path: "Outer",
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `{{Inner}}{{forward}}{{/Inner}}`},
		},
		FragmentArgs: []parser.FragmentArg{},
	}
	ctx := &renderContext{
		fragmentComponents: map[string]*parser.Page{"Inner": inner, "Outer": outer},
		queries:            map[string][]database.Row{},
		paginate:           map[string]PaginateInfo{},
		querySourceModels:  map[string]string{},
		customManifests:    map[string]*parser.CustomFieldManifest{},
		queryParams:        map[string]string{},
	}
	content := `{{Outer}}<x>{{/Outer}}`
	got := renderHTML(content, ctx)
	want := `<p>fb</p>`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
