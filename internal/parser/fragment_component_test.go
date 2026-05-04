package parser

import (
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/lexer"
)

func TestParseFragmentComponent(t *testing.T) {
	src := `fragment badge(status)
  html
    <span class="badge">{status}</span>
`
	tokens := lexer.Tokenize(src)
	app, err := Parse(tokens, src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(app.Fragments) != 1 {
		t.Fatalf("expected 1 fragment, got %d", len(app.Fragments))
	}
	f := app.Fragments[0]
	if f.Path != "badge" {
		t.Errorf("expected path 'badge', got %q", f.Path)
	}
	if len(f.FragmentArgs) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(f.FragmentArgs))
	}
	if f.FragmentArgs[0].Name != "status" {
		t.Errorf("expected arg name 'status', got %q", f.FragmentArgs[0].Name)
	}
	if f.FragmentArgs[0].DefaultValue != "" {
		t.Errorf("expected no default, got %q", f.FragmentArgs[0].DefaultValue)
	}
}

func TestParseFragmentComponentWithDefault(t *testing.T) {
	src := `fragment money(amount, currency="R$")
  html
    <span>{currency} {amount}</span>
`
	tokens := lexer.Tokenize(src)
	app, err := Parse(tokens, src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	f := app.Fragments[0]
	if len(f.FragmentArgs) != 2 {
		t.Fatalf("expected 2 args, got %d", len(f.FragmentArgs))
	}
	if f.FragmentArgs[0].Name != "amount" {
		t.Errorf("expected arg[0] name 'amount', got %q", f.FragmentArgs[0].Name)
	}
	if f.FragmentArgs[1].Name != "currency" {
		t.Errorf("expected arg[1] name 'currency', got %q", f.FragmentArgs[1].Name)
	}
	if f.FragmentArgs[1].DefaultValue != "R$" {
		t.Errorf("expected default 'R$', got %q", f.FragmentArgs[1].DefaultValue)
	}
}

func TestParseFragmentPathStillWorks(t *testing.T) {
	src := `fragment /users/:id/card
  query user: SELECT name FROM user WHERE id = :id
  html
    <div class="card">{user.name}</div>
`
	tokens := lexer.Tokenize(src)
	app, err := Parse(tokens, src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(app.Fragments) != 1 {
		t.Fatalf("expected 1 fragment, got %d", len(app.Fragments))
	}
	f := app.Fragments[0]
	if f.Path != "/users/:id/card" {
		t.Errorf("expected path '/users/:id/card', got %q", f.Path)
	}
	if f.FragmentArgs != nil {
		t.Errorf("expected nil FragmentArgs for path-based fragment")
	}
}

func TestParseFragmentComponentMissingParen(t *testing.T) {
	src := `fragment badge status`
	tokens := lexer.Tokenize(src)
	_, err := Parse(tokens, src)
	if err == nil {
		t.Fatal("expected error for missing '(' after component name")
	}
	if !strings.Contains(err.Error(), "expected '('") {
		t.Errorf("expected 'expected (' error, got: %v", err)
	}
}
