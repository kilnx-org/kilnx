package parser

import (
	"testing"

	"github.com/kilnx-org/kilnx/internal/lexer"
)

func TestParseRequiresFlag(t *testing.T) {
	src := `page /beta requires auth, flag "beta_ui"
  "Beta"
`
	tokens := lexer.Tokenize(src)
	app, err := Parse(tokens, src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(app.Pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(app.Pages))
	}
	clauses := app.Pages[0].RequiresClauses
	if len(clauses) != 2 {
		t.Fatalf("expected 2 clauses, got %d", len(clauses))
	}
	if clauses[1].Kind != RequiresClauseFlag {
		t.Errorf("expected Flag clause, got %d", clauses[1].Kind)
	}
	if clauses[1].Value != "beta_ui" {
		t.Errorf("expected flag name 'beta_ui', got %q", clauses[1].Value)
	}
}

func TestParseRequiresRateLimit(t *testing.T) {
	src := `api /exports requires auth, limit 10/hour per tenant
  query: SELECT * FROM export
`
	tokens := lexer.Tokenize(src)
	app, err := Parse(tokens, src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(app.APIs) != 1 {
		t.Fatalf("expected 1 api, got %d", len(app.APIs))
	}
	clauses := app.APIs[0].RequiresClauses
	if len(clauses) != 2 {
		t.Fatalf("expected 2 clauses, got %d", len(clauses))
	}
	if clauses[1].Kind != RequiresClauseRateLimit {
		t.Errorf("expected RateLimit clause, got %d", clauses[1].Kind)
	}
	if clauses[1].LimitCount != 10 {
		t.Errorf("expected limit count 10, got %d", clauses[1].LimitCount)
	}
	if clauses[1].LimitPeriod != "hour" {
		t.Errorf("expected limit period 'hour', got %q", clauses[1].LimitPeriod)
	}
	if clauses[1].LimitScope != "tenant" {
		t.Errorf("expected limit scope 'tenant', got %q", clauses[1].LimitScope)
	}
}

func TestParseRequiresRateLimitDefaultScope(t *testing.T) {
	src := `page /login requires limit 20/minute
  "Login"
`
	tokens := lexer.Tokenize(src)
	app, err := Parse(tokens, src)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	clauses := app.Pages[0].RequiresClauses
	if len(clauses) != 1 {
		t.Fatalf("expected 1 clause, got %d", len(clauses))
	}
	if clauses[0].LimitScope != "ip" {
		t.Errorf("expected default scope 'ip', got %q", clauses[0].LimitScope)
	}
}
