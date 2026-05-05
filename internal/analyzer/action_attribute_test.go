package analyzer

import (
	"testing"

	"github.com/kilnx-org/kilnx/internal/lexer"
	"github.com/kilnx-org/kilnx/internal/parser"
)

func TestCheckActionAttributesUnknown(t *testing.T) {
	src := `page /
  html
    <button action="/unknown">Do</button>
`
	tokens := lexer.Tokenize(src)
	app, _ := parser.Parse(tokens, src)
	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Message == "action attribute references unknown route '/unknown'" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error for unknown action, got: %v", diags)
	}
}

func TestCheckActionAttributesValid(t *testing.T) {
	src := `action /tasks/:id/delete
  query: DELETE FROM task WHERE id = :id

page /
  html
    <button action="/tasks/123/delete">Delete</button>
`
	tokens := lexer.Tokenize(src)
	app, _ := parser.Parse(tokens, src)
	diags := Analyze(app)
	for _, d := range diags {
		if d.Message == "action attribute references unknown route '/tasks/123/delete'" {
			t.Errorf("unexpected diagnostic: %v", d)
		}
	}
}
