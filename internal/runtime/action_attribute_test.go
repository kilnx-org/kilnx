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
	if !strContains(got, `hx-delete="/tasks/123/delete"`) {
		t.Errorf("missing hx-delete, got: %s", got)
	}
	if !strContains(got, `hx-target="closest [data-action-scope='/tasks/:id/delete']"`) {
		t.Errorf("missing hx-target, got: %s", got)
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
	if !strContains(got, `hx-post="/tasks"`) {
		t.Errorf("missing hx-post, got: %s", got)
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

func TestExpandActionAttributesInferFromSQL(t *testing.T) {
	actions := []parser.Page{
		{
			Path: "/tasks/:id",
			Body: []parser.Node{
				{Type: parser.NodeQuery, SQL: "DELETE FROM task WHERE id = :id"},
			},
		},
	}
	ctx := &renderContext{
		actions: actions,
	}
	content := `<button action="/tasks/123">Delete</button>`
	got := expandActionAttributes(content, ctx)
	if !strContains(got, `hx-delete="/tasks/123"`) {
		t.Errorf("expected hx-delete inferred from SQL, got: %s", got)
	}
}

func strContains(s, substr string) bool {
	return strings.Contains(s, substr)
}
