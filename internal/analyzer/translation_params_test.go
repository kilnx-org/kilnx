package analyzer

import (
	"testing"

	"github.com/kilnx-org/kilnx/internal/lexer"
	"github.com/kilnx-org/kilnx/internal/parser"
)

func TestCheckTranslationParamsMissing(t *testing.T) {
	src := `translations
  en
    greeting: "Hello {name}"

page /
  html
    <p>{t.greeting}</p>
`
	tokens := lexer.Tokenize(src)
	app, _ := parser.Parse(tokens, src)
	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Message == "missing parameter 'name' for translation 'greeting'" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error for missing param, got: %v", diags)
	}
}

func TestCheckTranslationParamsProvided(t *testing.T) {
	src := `translations
  en
    greeting: "Hello {name}"

page /
  html
    <p>{t.greeting name="World"}</p>
`
	tokens := lexer.Tokenize(src)
	app, _ := parser.Parse(tokens, src)
	diags := Analyze(app)
	for _, d := range diags {
		if d.Message == "missing parameter 'name' for translation 'greeting'" {
			t.Errorf("unexpected diagnostic: %v", d)
		}
	}
}

func TestCheckTranslationParamsLocaleDrift(t *testing.T) {
	src := `translations
  en
    greeting: "Hello {name}"
  pt
    greeting: "Ola"

page /
  html
    <p>{t.greeting name="World"}</p>
`
	tokens := lexer.Tokenize(src)
	app, _ := parser.Parse(tokens, src)
	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Message == "translation 'greeting' missing parameter 'name' in locale" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning for locale drift, got: %v", diags)
	}
}

func TestCheckTranslationParamsPluralIgnored(t *testing.T) {
	src := `translations
  en
    items: "{count} {count|plural:'item','items'}"

page /
  html
    <p>{t.items count=5}</p>
`
	tokens := lexer.Tokenize(src)
	app, _ := parser.Parse(tokens, src)
	diags := Analyze(app)
	for _, d := range diags {
		if d.Context == "page /" && d.Message == "missing parameter 'count' for translation 'items'" {
			t.Errorf("unexpected diagnostic for plural-only translation: %v", d)
		}
	}
}
