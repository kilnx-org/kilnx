package runtime

import (
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/parser"
)

func TestExpandActionAttributesBasic(t *testing.T) {
	actions := []parser.Page{
		{Path: "/tasks/:id/delete", Method: "DELETE"},
	}
	ctx := &renderContext{
		actions: actions,
	}
	content := `<button action="/tasks/123/delete">Delete</button>`
	got := expandActionAttributes(content, ctx)
	if !strings.Contains(got, `hx-delete="/tasks/123/delete"`) {
		t.Errorf("missing hx-delete, got: %s", got)
	}
	// CSRF placeholder must be emitted for non-GET verbs.
	if !strings.Contains(got, `hx-vals='{"_csrf":"{csrf}"}'`) {
		t.Errorf("missing hx-vals csrf, got: %s", got)
	}
	// hx-target intentionally omitted; htmx default ("this") applies.
	if strings.Contains(got, "hx-target=") {
		t.Errorf("expected no hx-target, got: %s", got)
	}
}

func TestExpandActionAttributesPost(t *testing.T) {
	actions := []parser.Page{
		{Path: "/tasks", Method: "POST"},
	}
	ctx := &renderContext{
		actions: actions,
	}
	content := `<button action="/tasks">Create</button>`
	got := expandActionAttributes(content, ctx)
	if !strings.Contains(got, `hx-post="/tasks"`) {
		t.Errorf("missing hx-post, got: %s", got)
	}
	if !strings.Contains(got, `hx-vals='{"_csrf":"{csrf}"}'`) {
		t.Errorf("missing csrf hx-vals, got: %s", got)
	}
}

func TestExpandActionAttributesUnknown(t *testing.T) {
	actions := []parser.Page{
		{Path: "/tasks", Method: "POST"},
	}
	ctx := &renderContext{
		actions: actions,
	}
	content := `<button action="/unknown">Do</button>`
	got := expandActionAttributes(content, ctx)
	if got != content {
		t.Errorf("expected unchanged for unknown action, got: %s", got)
	}
}

// TestExpandActionAttributesGetNoCSRF verifies GET actions do NOT get a CSRF
// hx-vals payload. Only non-GET verbs need it.
func TestExpandActionAttributesGetNoCSRF(t *testing.T) {
	actions := []parser.Page{
		{Path: "/search", Method: "GET"},
	}
	ctx := &renderContext{actions: actions}
	content := `<button action="/search">Search</button>`
	got := expandActionAttributes(content, ctx)
	if !strings.Contains(got, `hx-get="/search"`) {
		t.Errorf("missing hx-get, got: %s", got)
	}
	if strings.Contains(got, "hx-vals") {
		t.Errorf("GET should not emit hx-vals csrf, got: %s", got)
	}
}

// TestExpandActionAttributesSkipsForm guards against the unanchored-regex
// regression: <form action="..."> must remain untouched so native form
// submission and extractFormAction in testing.go keep working.
func TestExpandActionAttributesSkipsForm(t *testing.T) {
	actions := []parser.Page{
		{Path: "/login", Method: "POST"},
	}
	ctx := &renderContext{actions: actions}
	content := `<form action="/login" method="POST"><button action="/login">Go</button></form>`
	got := expandActionAttributes(content, ctx)
	if !strings.Contains(got, `<form action="/login" method="POST">`) {
		t.Errorf("form action must be preserved, got: %s", got)
	}
	// The button inside the form should still be rewritten.
	if !strings.Contains(got, `hx-post="/login"`) {
		t.Errorf("button inside form should still get hx-post, got: %s", got)
	}
}

// TestExpandActionAttributesPreservesOtherAttrs checks that surrounding
// attributes on the button stay intact when the action= attribute is rewritten.
func TestExpandActionAttributesPreservesOtherAttrs(t *testing.T) {
	actions := []parser.Page{
		{Path: "/tasks", Method: "POST"},
	}
	ctx := &renderContext{actions: actions}
	content := `<button class="btn" action="/tasks" id="x">Go</button>`
	got := expandActionAttributes(content, ctx)
	if !strings.Contains(got, `class="btn"`) || !strings.Contains(got, `id="x"`) {
		t.Errorf("surrounding attrs lost, got: %s", got)
	}
	if !strings.Contains(got, `hx-post="/tasks"`) {
		t.Errorf("missing hx-post, got: %s", got)
	}
}
