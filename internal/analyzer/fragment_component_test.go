package analyzer

import (
	"testing"

	"github.com/kilnx-org/kilnx/internal/lexer"
	"github.com/kilnx-org/kilnx/internal/parser"
)

func TestCheckFragmentComponentsUnknown(t *testing.T) {
	src := `page /
  html
    <div>{{badge status=active}}</div>
`
	tokens := lexer.Tokenize(src)
	app, _ := parser.Parse(tokens, src)
	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Message == "unknown component fragment 'badge'" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error for unknown component 'badge', got: %v", diags)
	}
}

func TestCheckFragmentComponentsMissingRequired(t *testing.T) {
	src := `fragment badge(status)
  html
    <span>{status}</span>

page /
  html
    <div>{{badge}}</div>
`
	tokens := lexer.Tokenize(src)
	app, _ := parser.Parse(tokens, src)
	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Message == "missing required argument 'status' for component 'badge'" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error for missing required arg, got: %v", diags)
	}
}

func TestCheckFragmentComponentsUnknownArg(t *testing.T) {
	src := `fragment badge(status)
  html
    <span>{status}</span>

page /
  html
    <div>{{badge status=active color=red}}</div>
`
	tokens := lexer.Tokenize(src)
	app, _ := parser.Parse(tokens, src)
	diags := Analyze(app)
	found := false
	for _, d := range diags {
		if d.Message == "unknown argument 'color' for component 'badge'" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error for unknown arg, got: %v", diags)
	}
}

func TestCheckFragmentComponentsValid(t *testing.T) {
	src := `fragment badge(status)
  html
    <span>{status}</span>

page /
  html
    <div>{{badge status=active}}</div>
`
	tokens := lexer.Tokenize(src)
	app, _ := parser.Parse(tokens, src)
	diags := Analyze(app)
	for _, d := range diags {
		if d.Context == "page /" && (d.Message == "unknown component fragment 'badge'" ||
			d.Message == "missing required argument 'status' for component 'badge'" ||
			d.Message == "unknown argument 'color' for component 'badge'") {
			t.Errorf("unexpected diagnostic: %v", d)
		}
	}
}
