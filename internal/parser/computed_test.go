package parser

import (
	"testing"

	"github.com/kilnx-org/kilnx/internal/lexer"
)

func TestParseComputedField(t *testing.T) {
	src := `model order
  quantity: int required
  unit_price: float required
  total: computed quantity * unit_price`

	tokens := lexer.Tokenize(src)
	app, err := Parse(tokens, src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(app.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(app.Models))
	}

	m := app.Models[0]
	if len(m.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(m.Fields))
	}

	f := m.Fields[2]
	if f.Name != "total" {
		t.Errorf("expected field name 'total', got %q", f.Name)
	}
	if !f.Computed {
		t.Errorf("expected computed=true")
	}
	if f.ComputedExpr != "quantity * unit_price" {
		t.Errorf("expected computed expr 'quantity * unit_price', got %q", f.ComputedExpr)
	}
}

func TestParseComputedFieldWithComplexExpr(t *testing.T) {
	src := `model item
  price: float required
  discount: float default 0
  final: computed price * (1 - discount / 100)`

	tokens := lexer.Tokenize(src)
	app, err := Parse(tokens, src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	m := app.Models[0]
	f := m.Fields[2]
	if !f.Computed {
		t.Errorf("expected computed=true")
	}
	expected := "price * (1 - discount / 100)"
	if f.ComputedExpr != expected {
		t.Errorf("expected computed expr %q, got %q", expected, f.ComputedExpr)
	}
}

func TestParseComputedFieldEmptyExpr(t *testing.T) {
	src := `model order
  total: computed`

	tokens := lexer.Tokenize(src)
	app, err := Parse(tokens, src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	m := app.Models[0]
	f := m.Fields[0]
	if !f.Computed {
		t.Errorf("expected computed=true")
	}
	if f.ComputedExpr != "" {
		t.Errorf("expected empty computed expr, got %q", f.ComputedExpr)
	}
}
