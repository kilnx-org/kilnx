package runtime

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// Named binding: `fetch payment: POST ...` must surface as `:payment.*`,
// not the legacy hardcoded `:fetch.*` prefix. Multiple fetches in the same
// action used to silently overwrite each other under the old prefix.
func TestFetch_NamedBindingPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id": "tx_42", "amount": 100}`))
	}))
	defer srv.Close()

	s := newTestServer(nil)
	params := map[string]string{}
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeFetch, FetchURL: srv.URL, Name: "payment"},
	}, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := params["payment.id"]; got != "tx_42" {
		t.Errorf("payment.id = %q, want tx_42", got)
	}
	if got := params["payment.amount"]; got != "100" {
		t.Errorf("payment.amount = %q, want 100", got)
	}
	// Legacy hardcoded prefix must be gone.
	if _, ok := params["fetch.id"]; ok {
		t.Errorf("legacy fetch.* prefix still present: %v", params)
	}
}

// Named binding plus status_code / ok lets users branch with `on payment.ok`.
func TestFetch_StatusFieldsExposed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(`{"detail":"nope"}`))
	}))
	defer srv.Close()

	s := newTestServer(nil)
	params := map[string]string{}
	err := s.executeNodes([]parser.Node{
		{Type: parser.NodeFetch, FetchURL: srv.URL, Name: "call"},
	}, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := params["call.status_code"]; got != "418" {
		t.Errorf("call.status_code = %q, want 418", got)
	}
	if got := params["call.ok"]; got != "false" {
		t.Errorf("call.ok = %q, want false", got)
	}
	if got := params["call.detail"]; got != "nope" {
		t.Errorf("call.detail = %q, want nope (body must still be parsed on 4xx)", got)
	}
}

// JSON body: when the user declares Content-Type: application/json the body
// is encoded as a JSON object with typed numbers/bools, not form-encoded.
// Required by Stripe-style APIs.
func TestFetch_JSONBodyEncoded(t *testing.T) {
	type captured struct {
		ContentType string
		Amount      json.Number
		Currency    string
		Live        bool
	}
	var got captured
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.ContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		dec := json.NewDecoder(strings.NewReader(string(body)))
		dec.UseNumber()
		var raw map[string]any
		_ = dec.Decode(&raw)
		if v, ok := raw["amount"].(json.Number); ok {
			got.Amount = v
		}
		if v, ok := raw["currency"].(string); ok {
			got.Currency = v
		}
		if v, ok := raw["live"].(bool); ok {
			got.Live = v
		}
		_, _ = w.Write([]byte(`{"ok":"true"}`))
	}))
	defer srv.Close()

	node := parser.Node{
		Type:        parser.NodeFetch,
		FetchURL:    srv.URL,
		FetchMethod: "POST",
		FetchHeaders: map[string]string{
			"Content-Type": "application/json",
		},
		FetchBody: map[string]string{
			"amount":   ":total",
			"currency": "usd",
			"live":     "true",
		},
	}
	rows, status, err := executeFetch(node, map[string]string{"total": "1000"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Errorf("status = %d, want 200", status)
	}
	if len(rows) == 0 || rows[0]["ok"] != "true" {
		t.Errorf("unexpected rows: %v", rows)
	}
	if !strings.Contains(got.ContentType, "application/json") {
		t.Errorf("server saw Content-Type=%q, want application/json", got.ContentType)
	}
	if got.Amount.String() != "1000" {
		t.Errorf("amount = %s, want 1000 (must be JSON number, not string)", got.Amount)
	}
	if got.Currency != "usd" {
		t.Errorf("currency = %q, want usd", got.Currency)
	}
	if !got.Live {
		t.Errorf("live = %v, want true (bool, not string)", got.Live)
	}
}

// Form-urlencoded remains the default for legacy callers that don't set a
// JSON Content-Type — preserves backward compatibility.
func TestFetch_FormBodyDefault(t *testing.T) {
	var seenContentType, seenName string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenContentType = r.Header.Get("Content-Type")
		_ = r.ParseForm()
		seenName = r.FormValue("name")
		_, _ = w.Write([]byte(`{"ok":"true"}`))
	}))
	defer srv.Close()

	node := parser.Node{
		Type:        parser.NodeFetch,
		FetchURL:    srv.URL,
		FetchMethod: "POST",
		FetchBody:   map[string]string{"name": "Alice"},
	}
	if _, _, err := executeFetch(node, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(seenContentType, "application/x-www-form-urlencoded") {
		t.Errorf("Content-Type = %q, want form-urlencoded", seenContentType)
	}
	if seenName != "Alice" {
		t.Errorf("name = %q, want Alice", seenName)
	}
}

// Transport failures must propagate so jobs/schedules can retry instead of
// silently committing partial state.
func TestFetch_TransportErrorPropagates(t *testing.T) {
	node := parser.Node{
		Type:        parser.NodeFetch,
		FetchURL:    "http://invalid.localhost:99999",
		FetchMethod: "GET",
		Name:        "x",
	}
	if _, _, err := executeFetch(node, nil); err == nil {
		t.Fatal("expected transport error, got nil")
	}
}

// 4xx/5xx responses are NOT errors: parse the body, expose the status, let
// the action decide how to react via `on x.ok`.
func TestFetch_HTTPErrorIsNotTransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer srv.Close()

	node := parser.Node{Type: parser.NodeFetch, FetchURL: srv.URL, FetchMethod: "GET"}
	rows, status, err := executeFetch(node, nil)
	if err != nil {
		t.Fatalf("HTTP 500 must not be reported as Go error, got %v", err)
	}
	if status != 500 {
		t.Errorf("status = %d, want 500", status)
	}
	if len(rows) == 0 || rows[0]["error"] != "boom" {
		t.Errorf("expected body parsed on 5xx, got %v", rows)
	}
}

// redactURL keeps the path but drops the query string so secrets passed via
// `:param` substitution don't surface in stdout.
func TestRedactURL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://api.example.com/v1/x", "https://api.example.com/v1/x"},
		{"https://api.example.com/v1/x?token=abc", "https://api.example.com/v1/x?<redacted>"},
		{"http://h/p?a=1&b=2", "http://h/p?<redacted>"},
	}
	for _, tc := range cases {
		if got := redactURL(tc.in); got != tc.want {
			t.Errorf("redactURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// jsonCoerce promotes unambiguous numbers/bools to typed JSON values; leaves
// anything else as a string. Sentinel cases that look numeric but are not
// (leading zeros, exponential text, plain words) stay strings.
func TestJSONCoerce(t *testing.T) {
	cases := []struct {
		in   string
		want any
	}{
		{"1000", int64(1000)},
		{"-42", int64(-42)},
		{"3.14", float64(3.14)},
		{"true", true},
		{"false", false},
		{"null", nil},
		{"", ""},
		{"hello", "hello"},
		{"123abc", "123abc"},
	}
	for _, tc := range cases {
		got := jsonCoerce(tc.in)
		if got != tc.want {
			t.Errorf("jsonCoerce(%q) = %v (%T), want %v (%T)", tc.in, got, got, tc.want, tc.want)
		}
	}
}
