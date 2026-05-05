package runtime

import (
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

func newComputedTestContext() *renderContext {
	return &renderContext{
		queries:           make(map[string][]database.Row),
		paginate:          make(map[string]PaginateInfo),
		queryParams:       make(map[string]string),
		querySourceModels: make(map[string]string),
		customManifests:   make(map[string]*parser.CustomFieldManifest),
	}
}

func computedOrderModel() parser.Model {
	return parser.Model{
		Name: "order",
		Fields: []parser.Field{
			{Name: "quantity", Type: parser.FieldInt},
			{Name: "unit_price", Type: parser.FieldFloat},
			{Name: "total", Type: parser.FieldComputed, Computed: true, ComputedExpr: "quantity * unit_price"},
		},
	}
}

func TestEvaluateComputedExpr_BasicMultiply(t *testing.T) {
	row := database.Row{"quantity": "3", "unit_price": "12"}
	got := evaluateComputedExpr("quantity * unit_price", row)
	if got != "36" {
		t.Errorf("got %q, want %q", got, "36")
	}
}

func TestEvaluateComputedExpr_FloatResult(t *testing.T) {
	row := database.Row{"quantity": "3", "unit_price": "12.5"}
	got := evaluateComputedExpr("quantity * unit_price", row)
	if got != "37.5" {
		t.Errorf("got %q, want %q", got, "37.5")
	}
}

func TestEvaluateComputedExpr_Precedence(t *testing.T) {
	row := database.Row{"a": "2", "b": "3", "c": "4"}
	// 2 + 3 * 4 = 14, not 20
	got := evaluateComputedExpr("a + b * c", row)
	if got != "14" {
		t.Errorf("got %q, want %q", got, "14")
	}
}

func TestEvaluateComputedExpr_Parens(t *testing.T) {
	row := database.Row{"price": "200", "discount": "10"}
	got := evaluateComputedExpr("price * (1 - discount / 100)", row)
	if got != "180" {
		t.Errorf("got %q, want %q", got, "180")
	}
}

func TestEvaluateComputedExpr_MissingField(t *testing.T) {
	row := database.Row{"quantity": "3"}
	got := evaluateComputedExpr("quantity * unit_price", row)
	if got != "" {
		t.Errorf("expected empty for missing field, got %q", got)
	}
}

func TestEvaluateComputedExpr_NonNumeric(t *testing.T) {
	row := database.Row{"a": "hello", "b": "1"}
	got := evaluateComputedExpr("a + b", row)
	if got != "" {
		t.Errorf("expected empty for non-numeric input, got %q", got)
	}
}

func TestEvaluateComputedExpr_DivisionByZero(t *testing.T) {
	row := database.Row{"a": "10", "b": "0"}
	got := evaluateComputedExpr("a / b", row)
	if got != "" {
		t.Errorf("expected empty for division by zero, got %q", got)
	}
}

// TestRenderHTML_ComputedField_QueryDot verifies {order.total} renders the
// computed value when the order model declares total as a computed expression.
func TestRenderHTML_ComputedField_QueryDot(t *testing.T) {
	ctx := newComputedTestContext()
	ctx.models = []parser.Model{computedOrderModel()}
	ctx.queries["order"] = []database.Row{{"quantity": "5", "unit_price": "10"}}
	ctx.querySourceModels["order"] = "order"

	out := renderHTML("Total: {order.total}", ctx)
	if !strings.Contains(out, "Total: 50") {
		t.Errorf("expected computed total to render as 50, got %q", out)
	}
}

// TestRenderHTML_ComputedField_BareNameInEach verifies {total} resolves to the
// computed value when iterating rows of a query whose source model has a
// computed total field.
func TestRenderHTML_ComputedField_BareNameInEach(t *testing.T) {
	ctx := newComputedTestContext()
	ctx.models = []parser.Model{computedOrderModel()}
	ctx.queries["orders"] = []database.Row{
		{"quantity": "2", "unit_price": "5"},
		{"quantity": "3", "unit_price": "4"},
	}
	ctx.querySourceModels["orders"] = "order"

	out := renderHTML("{{each orders}}<li>{total}</li>{{end}}", ctx)
	if !strings.Contains(out, "<li>10</li>") {
		t.Errorf("expected first row total 10, got %q", out)
	}
	if !strings.Contains(out, "<li>12</li>") {
		t.Errorf("expected second row total 12, got %q", out)
	}
}

// TestRenderHTML_ComputedField_QueryDotMissing returns empty rather than
// leaving the {expr} placeholder untouched, so users get a clean blank when
// the computation can't be performed (e.g. missing input).
func TestRenderHTML_ComputedField_QueryDotMissing(t *testing.T) {
	ctx := newComputedTestContext()
	ctx.models = []parser.Model{computedOrderModel()}
	ctx.queries["order"] = []database.Row{{"quantity": "5"}} // no unit_price
	ctx.querySourceModels["order"] = "order"

	out := renderHTML("Total: '{order.total}'", ctx)
	if strings.Contains(out, "{order.total}") {
		t.Errorf("expected placeholder to be replaced, got %q", out)
	}
}
