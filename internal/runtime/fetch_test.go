package runtime

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kilnx-org/kilnx/internal/parser"
)

func TestExecuteFetch_EnvHeaderEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Token") != "" {
			t.Errorf("expected empty X-Token when env var is empty, got %s", r.Header.Get("X-Token"))
		}
		_, _ = w.Write([]byte(`{"ok":"true"}`))
	}))
	defer srv.Close()

	node := parser.Node{
		Type:         parser.NodeFetch,
		FetchURL:     srv.URL,
		FetchMethod:  "GET",
		FetchHeaders: map[string]string{"X-Token": "env:NONEXISTENT_ENV_VAR"},
	}
	rows, err := executeFetch(node, nil)
	if err != nil {
		t.Fatalf("executeFetch failed: %v", err)
	}
	if len(rows) != 1 || rows[0]["ok"] != "true" {
		t.Errorf("unexpected rows: %v", rows)
	}
}

func TestExecuteFetch_EnvHeaderSet(t *testing.T) {
	t.Setenv("FETCH_TEST_TOKEN", "secret123")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Token") != "secret123" {
			t.Errorf("expected secret123, got %s", r.Header.Get("X-Token"))
		}
		_, _ = w.Write([]byte(`{"ok":"true"}`))
	}))
	defer srv.Close()

	node := parser.Node{
		Type:         parser.NodeFetch,
		FetchURL:     srv.URL,
		FetchMethod:  "GET",
		FetchHeaders: map[string]string{"X-Token": "env:FETCH_TEST_TOKEN"},
	}
	rows, err := executeFetch(node, nil)
	if err != nil {
		t.Fatalf("executeFetch failed: %v", err)
	}
	if len(rows) != 1 || rows[0]["ok"] != "true" {
		t.Errorf("unexpected rows: %v", rows)
	}
}

func TestExecuteFetch_BodyParamResolution(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("name") != "Alice" {
			t.Errorf("expected name=Alice, got %s", r.FormValue("name"))
		}
		_, _ = w.Write([]byte(`{"saved":"true"}`))
	}))
	defer srv.Close()

	node := parser.Node{
		Type:        parser.NodeFetch,
		FetchURL:    srv.URL,
		FetchMethod: "POST",
		FetchBody:   map[string]string{"name": ":user_name"},
	}
	rows, err := executeFetch(node, map[string]string{"user_name": "Alice"})
	if err != nil {
		t.Fatalf("executeFetch failed: %v", err)
	}
	if len(rows) != 1 || rows[0]["saved"] != "true" {
		t.Errorf("unexpected rows: %v", rows)
	}
}

func TestExecuteFetch_HeaderParamResolution(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-User") != "Alice" {
			t.Errorf("expected X-User=Alice, got %s", r.Header.Get("X-User"))
		}
		_, _ = w.Write([]byte(`{"ok":"true"}`))
	}))
	defer srv.Close()

	node := parser.Node{
		Type:         parser.NodeFetch,
		FetchURL:     srv.URL,
		FetchMethod:  "GET",
		FetchHeaders: map[string]string{"X-User": ":user_name"},
	}
	rows, err := executeFetch(node, map[string]string{"user_name": "Alice"})
	if err != nil {
		t.Fatalf("executeFetch failed: %v", err)
	}
	if len(rows) != 1 || rows[0]["ok"] != "true" {
		t.Errorf("unexpected rows: %v", rows)
	}
}

func TestExecuteFetch_GetIgnoresBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && r.Body != http.NoBody {
			// GET should not have a body
			t.Error("GET request should not have a body")
		}
		_, _ = w.Write([]byte(`{"ok":"true"}`))
	}))
	defer srv.Close()

	node := parser.Node{
		Type:        parser.NodeFetch,
		FetchURL:    srv.URL,
		FetchMethod: "GET",
		FetchBody:   map[string]string{"key": "value"},
	}
	rows, err := executeFetch(node, nil)
	if err != nil {
		t.Fatalf("executeFetch failed: %v", err)
	}
	if len(rows) != 1 || rows[0]["ok"] != "true" {
		t.Errorf("unexpected rows: %v", rows)
	}
}
