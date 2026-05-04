package runtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, map[string]string{"status": "ok"})

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("expected Content-Type application/json; charset=utf-8, got %q", ct)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %q", body["status"])
	}
}

func TestWriteJSON_Error(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusBadRequest, map[string]string{"error": "invalid"})

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestRequireAPIAuth_NoAuth(t *testing.T) {
	s := newTestServer(nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	api := parser.Page{Path: "/api/test", Auth: false}
	if !s.requireAPIAuth(rec, req, api) {
		t.Error("expected requireAPIAuth to return true when page.Auth is false")
	}
}

func TestRequireAPIAuth_NoAppAuth(t *testing.T) {
	s := newTestServer(nil)
	s.app.Auth = nil
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	api := parser.Page{Path: "/api/test", Auth: true}
	if !s.requireAPIAuth(rec, req, api) {
		t.Error("expected requireAPIAuth to return true when app.Auth is nil")
	}
}

func TestRequireAPIAuth_RequiresAuthRole(t *testing.T) {
	s := newTestServer(nil)
	sessionID := s.sessions.Create(database.Row{"id": "1", "email": "user@example.com", "role": "user"}, "email")
	cookieValue := s.sessions.signSessionID(sessionID)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.AddCookie(&http.Cookie{Name: "_kilnx_session", Value: cookieValue})
	api := parser.Page{Path: "/api/test", Auth: true, RequiresRole: "auth"}
	if !s.requireAPIAuth(rec, req, api) {
		t.Error("expected requireAPIAuth to return true when RequiresRole is auth")
	}
}

// routeAPI simulates the server's API routing logic for testing.
func routeAPI(s *Server, w http.ResponseWriter, r *http.Request) {
	app := s.getApp()
	for _, api := range app.APIs {
		if matchPath(api.Path, r.URL.Path) {
			if r.Method == http.MethodOptions {
				s.handleAPI(w, r, api)
				return
			}
			if api.Method != "" && !strings.EqualFold(api.Method, r.Method) {
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
				return
			}
			if !s.requireAPIAuth(w, r, api) {
				return
			}
			s.handleAPI(w, r, api)
			return
		}
	}
	http.NotFound(w, r)
}

func TestHandleAPI_MethodNotAllowed(t *testing.T) {
	s := newTestServer(nil)
	s.app.APIs = []parser.Page{
		{Path: "/api/users", Method: "POST"},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rec := httptest.NewRecorder()
	routeAPI(s, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("code = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleAPI_MissingAuth(t *testing.T) {
	s := newTestServer(nil)
	s.app.APIs = []parser.Page{
		{Path: "/api/profile", Auth: true},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	rec := httptest.NewRecorder()
	routeAPI(s, rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "unauthorized") {
		t.Errorf("expected unauthorized error, got %q", body)
	}
}

func TestHandleAPI_InvalidAuth(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	s.app.APIs = []parser.Page{
		{Path: "/api/admin", Auth: true, RequiresRole: "admin"},
	}

	// Create a session for a regular user
	sessionID := s.sessions.Create(database.Row{"id": "1", "email": "user@example.com", "role": "user"}, "email")
	cookieValue := s.sessions.signSessionID(sessionID)

	req := httptest.NewRequest(http.MethodGet, "/api/admin", nil)
	req.AddCookie(&http.Cookie{Name: "_kilnx_session", Value: cookieValue})
	rec := httptest.NewRecorder()
	routeAPI(s, rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, want %d", rec.Code, http.StatusForbidden)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "forbidden") {
		t.Errorf("expected forbidden error, got %q", body)
	}
}

func TestHandleAPI_ValidRole(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	s.app.APIs = []parser.Page{
		{Path: "/api/admin", Auth: true, RequiresRole: "admin"},
	}

	sessionID := s.sessions.Create(database.Row{"id": "1", "email": "admin@example.com", "role": "admin"}, "email")
	cookieValue := s.sessions.signSessionID(sessionID)

	req := httptest.NewRequest(http.MethodGet, "/api/admin", nil)
	req.AddCookie(&http.Cookie{Name: "_kilnx_session", Value: cookieValue})
	rec := httptest.NewRecorder()
	routeAPI(s, rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleAPI_RequiresClausesForbidden(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	s.app.APIs = []parser.Page{
		{Path: "/api/admin", Auth: true, RequiresClauses: []parser.RequiresClause{
			{Kind: parser.RequiresClauseRole, Value: "admin"},
		}},
	}

	sessionID := s.sessions.Create(database.Row{"id": "1", "email": "user@example.com", "role": "user"}, "email")
	cookieValue := s.sessions.signSessionID(sessionID)

	req := httptest.NewRequest(http.MethodGet, "/api/admin", nil)
	req.AddCookie(&http.Cookie{Name: "_kilnx_session", Value: cookieValue})
	rec := httptest.NewRecorder()
	routeAPI(s, rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestHandleAPI_RequiresClausesAllowed(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	s.app.APIs = []parser.Page{
		{Path: "/api/admin", Auth: true, RequiresClauses: []parser.RequiresClause{
			{Kind: parser.RequiresClauseRole, Value: "admin"},
		}},
	}

	sessionID := s.sessions.Create(database.Row{"id": "1", "email": "admin@example.com", "role": "admin"}, "email")
	cookieValue := s.sessions.signSessionID(sessionID)

	req := httptest.NewRequest(http.MethodGet, "/api/admin", nil)
	req.AddCookie(&http.Cookie{Name: "_kilnx_session", Value: cookieValue})
	rec := httptest.NewRecorder()
	routeAPI(s, rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleAPI_QuerySuccess(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT id, name FROM users`] = []database.Row{
		{"id": "1", "name": "Bob"},
		{"id": "2", "name": "Bob"},
	}

	s := newTestServer(mock)
	s.app.APIs = []parser.Page{
		{
			Path: "/api/users",
			Body: []parser.Node{
				{Type: parser.NodeQuery, SQL: `SELECT id, name FROM users`, Name: "users"},
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rec := httptest.NewRecorder()
	s.handleAPI(rec, req, s.app.APIs[0])

	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	data, ok := body["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data array, got %T", body["data"])
	}
	if len(data) != 2 {
		t.Errorf("expected 2 rows, got %d", len(data))
	}
}

func TestHandleAPI_QueryError(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsErr[`SELECT id FROM users`] = fmt.Errorf("database error")

	s := newTestServer(mock)
	s.app.APIs = []parser.Page{
		{
			Path: "/api/users",
			Body: []parser.Node{
				{Type: parser.NodeQuery, SQL: `SELECT id FROM users`, Name: "users"},
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rec := httptest.NewRecorder()
	s.handleAPI(rec, req, s.app.APIs[0])

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["error"] != "Internal server error" {
		t.Errorf("expected 'Internal server error', got %q", body["error"])
	}
}

func TestHandleAPI_RespondStatus(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	s.app.APIs = []parser.Page{
		{
			Path: "/api/create",
			Body: []parser.Node{
				{Type: parser.NodeRespond, StatusCode: 201},
				{Type: parser.NodeQuery, SQL: `SELECT id FROM users`, Name: "users"},
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/create", nil)
	rec := httptest.NewRecorder()
	s.handleAPI(rec, req, s.app.APIs[0])

	if rec.Code != 201 {
		t.Errorf("code = %d, want %d", rec.Code, 201)
	}
}

func TestHandleAPI_PathParams(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT name FROM users WHERE id = :id`] = []database.Row{
		{"name": "Bob"},
	}

	s := newTestServer(mock)
	s.app.APIs = []parser.Page{
		{
			Path: "/api/users/:id",
			Body: []parser.Node{
				{Type: parser.NodeQuery, SQL: `SELECT name FROM users WHERE id = :id`, Name: "user"},
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/users/42", nil)
	rec := httptest.NewRecorder()
	s.handleAPI(rec, req, s.app.APIs[0])

	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	data, ok := body["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data array, got %T", body["data"])
	}
	if len(data) != 1 {
		t.Errorf("expected 1 row, got %d", len(data))
	}
}

func TestHandleAPI_OPTIONS(t *testing.T) {
	s := newTestServer(nil)
	s.app.Config = &parser.AppConfig{CORSOrigins: []string{"*"}}
	s.app.APIs = []parser.Page{{Path: "/api/users", Method: "GET"}}

	req := httptest.NewRequest(http.MethodOptions, "/api/users", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	s.handleAPI(rec, req, s.app.APIs[0])

	if rec.Code != http.StatusNoContent {
		t.Errorf("code = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://example.com" {
		t.Errorf("expected CORS header, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestHandleAPI_CORSDisallowed(t *testing.T) {
	s := newTestServer(nil)
	s.app.Config = &parser.AppConfig{CORSOrigins: []string{"https://trusted.com"}}
	s.app.APIs = []parser.Page{{Path: "/api/users", Method: "GET"}}

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Origin", "http://evil.com")
	rec := httptest.NewRecorder()
	s.handleAPI(rec, req, s.app.APIs[0])

	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("expected no CORS header for disallowed origin, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestHandleAPI_NoDB(t *testing.T) {
	s := newTestServer(nil)
	s.app.APIs = []parser.Page{
		{
			Path: "/api/users",
			Body: []parser.Node{
				{Type: parser.NodeQuery, SQL: `SELECT id FROM users`, Name: "users"},
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rec := httptest.NewRecorder()
	s.handleAPI(rec, req, s.app.APIs[0])

	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want %d", rec.Code, http.StatusOK)
	}
	var body map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &body)
	// When db is nil, data remains nil (not empty slice)
	if body["data"] != nil {
		t.Errorf("expected nil data when no db, got %v", body["data"])
	}
}

func TestHandleAPI_QueryWithParams(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT name FROM users WHERE status = :status`] = []database.Row{
		{"name": "Active User"},
	}

	s := newTestServer(mock)
	s.app.APIs = []parser.Page{
		{
			Path: "/api/users",
			Body: []parser.Node{
				{Type: parser.NodeQuery, SQL: `SELECT name FROM users WHERE status = :status`, Name: "users"},
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/users?status=active", nil)
	rec := httptest.NewRecorder()
	s.handleAPI(rec, req, s.app.APIs[0])

	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want %d", rec.Code, http.StatusOK)
	}
	var body map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &body)
	data, _ := body["data"].([]interface{})
	if len(data) != 1 {
		t.Errorf("expected 1 row, got %d", len(data))
	}
}

func TestHandleAPI_ValidateInlineFail(t *testing.T) {
	s := newTestServer(nil)
	s.app.APIs = []parser.Page{
		{
			Path: "/api/create",
			Body: []parser.Node{
				{Type: parser.NodeValidate, Validations: []parser.Validation{
					{Field: "email", Rules: []string{"required", "is", "email"}},
				}},
			},
		},
	}

	form := url.Values{"email": {"bad-email"}}
	req := httptest.NewRequest(http.MethodPost, "/api/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleAPI(rec, req, s.app.APIs[0])

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	var body map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &body)
	errs, ok := body["errors"].([]interface{})
	if !ok || len(errs) == 0 {
		t.Errorf("expected validation errors, got %v", body)
	}
}

func TestHandleAPI_ValidateModelFail(t *testing.T) {
	s := newTestServer(nil)
	s.app.Models = []parser.Model{{
		Name: "contact",
		Fields: []parser.Field{
			{Name: "name", Type: parser.FieldText, Required: true},
		},
	}}
	s.app.APIs = []parser.Page{
		{
			Path: "/api/create",
			Body: []parser.Node{
				{Type: parser.NodeValidate, ModelName: "contact"},
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/create", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleAPI(rec, req, s.app.APIs[0])

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
}

func TestHandleAPI_RedirectWithTx(t *testing.T) {
	mock := newMockExecutor()
	// Need tx support for mutation; skip if BeginTxHandle fails
	if _, err := mock.BeginTxHandle(); err != nil {
		t.Skip("tx not supported in mock")
	}

	s := newTestServer(mock)
	s.app.APIs = []parser.Page{
		{
			Path: "/api/create",
			Body: []parser.Node{
				{Type: parser.NodeQuery, SQL: `INSERT INTO users (name) VALUES (:name)`},
				{Type: parser.NodeRedirect, Value: "/users/:name"},
			},
		},
	}

	form := url.Values{"name": {"Alice"}}
	req := httptest.NewRequest(http.MethodPost, "/api/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleAPI(rec, req, s.app.APIs[0])

	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want %d", rec.Code, http.StatusOK)
	}
	var body map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &body)
	redirect, ok := body["redirect"].(string)
	if !ok || redirect != "/users/Alice" {
		t.Errorf("expected redirect to /users/Alice, got %v", body["redirect"])
	}
}

func TestHandleAPI_Pagination(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT COUNT(*) as _count FROM (SELECT id FROM users)`] = []database.Row{{"_count": "25"}}
	mock.queryRowsWithParamsResults[`SELECT id FROM users LIMIT 10 OFFSET 0`] = []database.Row{
		{"id": "1"}, {"id": "2"},
	}

	s := newTestServer(mock)
	s.app.APIs = []parser.Page{
		{
			Path: "/api/users",
			Body: []parser.Node{
				{Type: parser.NodeQuery, SQL: `SELECT id FROM users`, Name: "users", Paginate: 10},
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/users?page=1", nil)
	rec := httptest.NewRecorder()
	s.handleAPI(rec, req, s.app.APIs[0])

	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want %d", rec.Code, http.StatusOK)
	}
	var body map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &body)
	pg, ok := body["pagination"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected pagination, got %v", body)
	}
	if pg["page"] != float64(1) {
		t.Errorf("expected page 1, got %v", pg["page"])
	}
	if pg["total"] != float64(25) {
		t.Errorf("expected total 25, got %v", pg["total"])
	}
}


func TestHandleAPI_CORSAllowed(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	s.app.Config = &parser.AppConfig{CORSOrigins: []string{"https://example.com"}}
	s.app.APIs = []parser.Page{
		{Path: "/api/test", Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT 1`},
		}},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	s.handleAPI(rec, req, s.app.APIs[0])

	if rec.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Error("expected CORS header")
	}
}

func TestHandleAPI_PageNumNegative(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT COUNT(*) as _count FROM (SELECT id FROM users)`] = []database.Row{{"_count": "5"}}
	mock.queryRowsWithParamsResults[`SELECT id FROM users LIMIT 10 OFFSET 0`] = []database.Row{
		{"id": "1"},
	}

	s := newTestServer(mock)
	s.app.APIs = []parser.Page{
		{Path: "/api/users", Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT id FROM users`, Name: "users", Paginate: 10},
		}},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/users?page=-5", nil)
	rec := httptest.NewRecorder()
	s.handleAPI(rec, req, s.app.APIs[0])

	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", rec.Code)
	}
}

func TestHandleAPI_MutationWithTx(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	s := newTestServer(db)
	s.app.APIs = []parser.Page{
		{Path: "/api/users", Method: http.MethodPost, Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `INSERT INTO users (name) VALUES (:name)`},
		}},
	}

	form := url.Values{"name": {"Alice"}}
	req := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleAPI(rec, req, s.app.APIs[0])

	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", rec.Code)
	}

	rows, _ := db.QueryRows("SELECT name FROM users")
	if len(rows) != 1 || rows[0]["name"] != "Alice" {
		t.Error("expected Alice to be inserted")
	}
}

func TestHandleAPI_MutationSelectWithTx(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	if _, err := db.Conn().Exec("INSERT INTO users (name) VALUES ('Alice')"); err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	s := newTestServer(db)
	s.app.APIs = []parser.Page{
		{Path: "/api/users", Method: http.MethodPost, Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `INSERT INTO users (name) VALUES (:name)`},
			{Type: parser.NodeQuery, SQL: `SELECT name FROM users WHERE name = :name`, Name: "user"},
		}},
	}

	form := url.Values{"name": {"Bob"}}
	req := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleAPI(rec, req, s.app.APIs[0])

	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", rec.Code)
	}
	var body map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &body)
	data, ok := body["data"].([]interface{})
	if !ok || len(data) != 1 {
		t.Fatalf("expected 1 row, got %v", body)
	}
}

func TestHandleAPI_TenantRejection(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	s.tenants = TenantMap{"users": "org_id"}
	s.app.APIs = []parser.Page{
		{Path: "/api/users", Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT * FROM users`},
		}},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rec := httptest.NewRecorder()
	s.handleAPI(rec, req, s.app.APIs[0])

	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", rec.Code)
	}
}

func TestHandleAPI_Redirect(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	if _, err := db.Conn().Exec("INSERT INTO users (id, name) VALUES (42, 'Bob')"); err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	s := newTestServer(db)
	s.app.APIs = []parser.Page{
		{Path: "/api/users/:id", Method: http.MethodPost, Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `UPDATE users SET name = :name WHERE id = :id`},
			{Type: parser.NodeRedirect, Value: "/api/users/:id"},
		}},
	}

	form := url.Values{"name": {"Bob"}}
	req := httptest.NewRequest(http.MethodPost, "/api/users/42", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleAPI(rec, req, s.app.APIs[0])

	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", rec.Code)
	}
	var body map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["redirect"] != "/api/users/42" {
		t.Errorf("expected redirect /api/users/42, got %v", body["redirect"])
	}
}

func TestHandleAPI_MutationQueryError(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	s := newTestServer(db)
	s.app.APIs = []parser.Page{
		{Path: "/api/users", Method: http.MethodPost, Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `INSERT INTO users (name) VALUES (NULL)`},
		}},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/users", nil)
	rec := httptest.NewRecorder()
	s.handleAPI(rec, req, s.app.APIs[0])

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", rec.Code)
	}
}

func TestHandleAPI_ValidateInlinePass(t *testing.T) {
	s := newTestServer(nil)
	s.app.APIs = []parser.Page{
		{Path: "/api/users", Method: http.MethodPost, Body: []parser.Node{
			{Type: parser.NodeValidate, Validations: []parser.Validation{
				{Field: "email", Rules: []string{"required"}},
			}},
			{Type: parser.NodeRespond, StatusCode: 201},
		}},
	}

	form := url.Values{"email": {"test@example.com"}}
	req := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleAPI(rec, req, s.app.APIs[0])

	if rec.Code != 201 {
		t.Errorf("code = %d, want 201", rec.Code)
	}
}


func TestHandleAPI_QueryNoName(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT id, name FROM users`] = []database.Row{
		{"id": "1", "name": "Bob"},
	}

	s := newTestServer(mock)
	s.app.APIs = []parser.Page{
		{
			Path: "/api/users",
			Body: []parser.Node{
				{Type: parser.NodeQuery, SQL: `SELECT id, name FROM users`},
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rec := httptest.NewRecorder()
	s.handleAPI(rec, req, s.app.APIs[0])

	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	data, ok := body["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data array, got %T", body["data"])
	}
	if len(data) != 1 {
		t.Errorf("expected 1 row, got %d", len(data))
	}
}
