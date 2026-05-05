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

// TestCheckActionAttributesIgnoresForm guards Issue 2/6: <form action="/login">
// must not be flagged as an unknown action — analyzer only checks button/a/input.
func TestCheckActionAttributesIgnoresForm(t *testing.T) {
	src := `page /login
  html
    <form action="/login" method="POST"><button type="submit">Go</button></form>
`
	tokens := lexer.Tokenize(src)
	app, _ := parser.Parse(tokens, src)
	diags := Analyze(app)
	for _, d := range diags {
		if d.Message == "action attribute references unknown route '/login'" {
			t.Errorf("form action should not be flagged: %v", d)
		}
	}
}

// TestCheckActionAttributesPageRoute guards Issue 6: a button or link with
// action="/page-path" pointing at a page route should not be flagged.
func TestCheckActionAttributesPageRoute(t *testing.T) {
	src := `page /login
  html
    Login

page /home
  html
    <a action="/login">Sign in</a>
`
	tokens := lexer.Tokenize(src)
	app, _ := parser.Parse(tokens, src)
	diags := Analyze(app)
	for _, d := range diags {
		if d.Message == "action attribute references unknown route '/login'" {
			t.Errorf("page route should suppress diagnostic: %v", d)
		}
	}
}
