package runtime

import (
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
			{Name: "currency", DefaultValue: "R$"},
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



