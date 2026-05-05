package runtime

import (
	"testing"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

// ---------- extractFormAction ----------

func TestExtractFormAction_NoActionAttribute(t *testing.T) {
	html := `<form method="POST">`
	if got := extractFormAction(html); got != "" {
		t.Errorf("expected empty for form without action, got %q", got)
	}
}

func TestExtractFormAction_SingleQuotesMethodFirst(t *testing.T) {
	html := `<form method='POST' action='/submit'>`
	if got := extractFormAction(html); got != "/submit" {
		t.Errorf("expected /submit, got %q", got)
	}
}

func TestExtractFormAction_MultipleFormsFirstWins(t *testing.T) {
	html := `<form action="/first"><form action="/second">`
	if got := extractFormAction(html); got != "/first" {
		t.Errorf("expected /first, got %q", got)
	}
}

// ---------- extractCSRFFromHTML ----------

func TestExtractCSRFFromHTML_ReorderedAttributes(t *testing.T) {
	html := `<input type="hidden" value="abc123" name="_csrf">`
	if got := extractCSRFFromHTML(html); got != "abc123" {
		t.Errorf("expected abc123 with reordered attrs, got %q", got)
	}
}

func TestExtractCSRFFromHTML_MissingClosingQuote(t *testing.T) {
	html := `<input name="_csrf" value="abc123`
	if got := extractCSRFFromHTML(html); got != "" {
		t.Errorf("expected empty for missing closing quote, got %q", got)
	}
}

func TestExtractCSRFFromHTML_EmptyValue(t *testing.T) {
	html := `<input name="_csrf" value="">`
	if got := extractCSRFFromHTML(html); got != "" {
		t.Errorf("expected empty for empty value, got %q", got)
	}
}

func TestExtractCSRFFromHTML_NotHiddenInput(t *testing.T) {
	html := `<input type="text" name="_csrf" value="abc123">`
	if got := extractCSRFFromHTML(html); got != "abc123" {
		t.Errorf("expected abc123 regardless of input type, got %q", got)
	}
}

// ---------- evaluateExpect (query: returns variant) ----------

func TestEvaluateExpect_QueryReturnsMatch(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec("CREATE TABLE test_items (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	if _, err := db.Conn().Exec("INSERT INTO test_items (name) VALUES ('Alice'), ('Bob')"); err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	step := parser.TestStep{
		Target: "query: SELECT COUNT(*) FROM test_items returns",
		Value:  "2",
	}
	if !evaluateExpect(step, "", 200, "", db) {
		t.Error("expected true for matching query return value")
	}
}

func TestEvaluateExpect_QueryReturnsMismatch(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec("CREATE TABLE test_items (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	step := parser.TestStep{
		Target: "query: SELECT COUNT(*) FROM test_items returns",
		Value:  "5",
	}
	if evaluateExpect(step, "", 200, "", db) {
		t.Error("expected false for mismatching query return value")
	}
}

func TestEvaluateExpect_QueryError(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	step := parser.TestStep{
		Target: "query: SELECT * FROM missing_table returns",
		Value:  "1",
	}
	if evaluateExpect(step, "", 200, "", db) {
		t.Error("expected false for query error")
	}
}

func TestEvaluateExpect_QueryReturnsEmptyResult(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	step := parser.TestStep{
		Target: "query: SELECT 1 WHERE 1=0 returns",
		Value:  "0",
	}
	if !evaluateExpect(step, "", 200, "", db) {
		t.Error("expected true when query returns empty result (defaults to 0)")
	}
}

func TestEvaluateExpect_QueryReturnsStringValue(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec("CREATE TABLE test_items (name TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	if _, err := db.Conn().Exec("INSERT INTO test_items (name) VALUES ('Widget')"); err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	step := parser.TestStep{
		Target: "query: SELECT name FROM test_items WHERE name = 'Widget' returns",
		Value:  "Widget",
	}
	if !evaluateExpect(step, "", 200, "", db) {
		t.Error("expected true for matching string query return value")
	}
}

// ---------- runSingleTest (smoke: no HTTP server needed) ----------

func TestRunSingleTest_NoSteps(t *testing.T) {
	app := &parser.App{}
	test := parser.Test{Name: "empty"}
	// No steps → should return true
	if !runSingleTest(test, app, nil, "") {
		t.Error("expected true for test with no steps")
	}
}

func TestRunSingleTest_VisitWithoutServer(t *testing.T) {
	app := &parser.App{}
	test := parser.Test{
		Name: "visit_fail",
		Steps: []parser.TestStep{
			{Action: "visit", Target: "/"},
		},
	}
	// No server running → visit should fail
	if runSingleTest(test, app, nil, "") {
		t.Error("expected false when visiting without a server")
	}
}

// ---------- evaluateExpect edge cases ----------

func TestEvaluateExpect_StatusInvalidNumber(t *testing.T) {
	step := parser.TestStep{Target: "status abc", Value: ""}
	if evaluateExpect(step, "", 200, "", nil) {
		t.Error("expected false for invalid status code format")
	}
}

func TestEvaluateExpect_RedirectWithoutLocation(t *testing.T) {
	step := parser.TestStep{Target: "redirect to /home", Value: ""}
	if evaluateExpect(step, "", 200, "/other", nil) {
		t.Error("expected false for wrong redirect location")
	}
}

func TestEvaluateExpect_ContainsNotFound(t *testing.T) {
	step := parser.TestStep{Target: "page contains", Value: "missing"}
	if evaluateExpect(step, "hello world", 200, "", nil) {
		t.Error("expected false when page does not contain expected text")
	}
}
