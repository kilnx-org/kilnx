package runtime

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

func newTestServer(db database.Executor) *Server {
	app := &parser.App{
		Auth: &parser.AuthConfig{
			Table:        "users",
			Identity:     "email",
			Password:     "password_hash",
			LoginPath:    "/login",
			LogoutPath:   "/logout",
			RegisterPath: "/register",
			ForgotPath:   "/forgot-password",
			ResetPath:    "/reset-password",
		},
		Pages: []parser.Page{
			{Path: "/login"},
			{Path: "/forgot-password"},
			{Path: "/reset-password"},
		},
	}
	s := &Server{
		app:         app,
		db:          db,
		sessions:    NewSessionStore("test-secret"),
		logger:      NewLogger(nil),
		i18n:        NewI18n(nil, "en", false),
		tenants:     nil,
		rateLimiter: NewRateLimiter(nil),
	}
	if db != nil {
		s.sessions.SetDB(db)
	}
	return s
}

// ---------- handleLogout ----------

func TestHandleLogout_GET(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("GET", "/logout", nil)
	rec := httptest.NewRecorder()
	s.handleLogout(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("code = %d, want 405", rec.Code)
	}
}

func TestHandleLogout_InvalidCSRF(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("POST", "/logout", strings.NewReader("_csrf=bad"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleLogout(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", rec.Code)
	}
}

func TestHandleLogout_NoCookie(t *testing.T) {
	s := newTestServer(nil)
	// Generate valid CSRF token
	token := generateCSRFToken()
	req := httptest.NewRequest("POST", "/logout", strings.NewReader("_csrf="+url.QueryEscape(token)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleLogout(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	// Check cookie is cleared (Go normalizes MaxAge:-1 to Max-Age=0)
	setCookie := rec.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, "Max-Age=0") && !strings.Contains(setCookie, "Max-Age=-1") {
		t.Errorf("expected cookie cleared, got %q", setCookie)
	}
}

func TestHandleLogout_WithSession(t *testing.T) {
	s := newTestServer(nil)
	sessionID := s.sessions.Create(database.Row{"id": "1", "email": "user@example.com", "role": "user"}, "email")
	cookieValue := s.sessions.signSessionID(sessionID)
	// Generate valid CSRF token
	token := generateCSRFToken()
	req := httptest.NewRequest("POST", "/logout", strings.NewReader("_csrf="+url.QueryEscape(token)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "_kilnx_session", Value: cookieValue})
	rec := httptest.NewRecorder()
	s.handleLogout(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	// Verify session was deleted
	if s.sessions.Get(sessionID) != nil {
		t.Error("session should be deleted")
	}
}

// ---------- handleForgotPassword ----------

func TestHandleForgotPassword_GET(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("GET", "/forgot-password", nil)
	rec := httptest.NewRecorder()
	s.handleForgotPassword(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("code = %d, want 405", rec.Code)
	}
}

func TestHandleForgotPassword_NoAuth(t *testing.T) {
	s := &Server{
		app:      &parser.App{},
		sessions: NewSessionStore("secret"),
	}
	req := httptest.NewRequest("POST", "/forgot-password", nil)
	rec := httptest.NewRecorder()
	s.handleForgotPassword(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", rec.Code)
	}
}

func TestHandleForgotPassword_NoDB(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("POST", "/forgot-password", nil)
	rec := httptest.NewRecorder()
	s.handleForgotPassword(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", rec.Code)
	}
}

func TestHandleForgotPassword_EmptyEmail(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	token := generateCSRFToken()
	req := httptest.NewRequest("POST", "/forgot-password", strings.NewReader("_csrf="+url.QueryEscape(token)+"&email="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleForgotPassword(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "error=") {
		t.Errorf("expected error redirect, got %q", loc)
	}
}

func TestHandleForgotPassword_UserNotFound(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	token := generateCSRFToken()
	req := httptest.NewRequest("POST", "/forgot-password", strings.NewReader("_csrf="+url.QueryEscape(token)+"&email=missing@example.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleForgotPassword(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "sent=1") {
		t.Errorf("expected sent=1 redirect, got %q", loc)
	}
}

func TestHandleForgotPassword_UserFound(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT id, email FROM "users" WHERE email = :email`] = []database.Row{
		{"id": "1", "email": "user@example.com"},
	}
	s := newTestServer(mock)
	token := generateCSRFToken()
	req := httptest.NewRequest("POST", "/forgot-password", strings.NewReader("_csrf="+url.QueryEscape(token)+"&email=user@example.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleForgotPassword(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "sent=1") {
		t.Errorf("expected sent=1 redirect, got %q", loc)
	}
	// Verify token was stored
	foundDelete := false
	foundInsert := false
	for _, call := range mock.execCalled {
		if strings.Contains(call.SQL, "DELETE FROM _kilnx_password_resets") {
			foundDelete = true
		}
		if strings.Contains(call.SQL, "INSERT INTO _kilnx_password_resets") {
			foundInsert = true
		}
	}
	if !foundDelete {
		t.Error("expected DELETE from _kilnx_password_resets")
	}
	if !foundInsert {
		t.Error("expected INSERT into _kilnx_password_resets")
	}
}

func TestHandleForgotPassword_HTTPS(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT id, email FROM "users" WHERE email = :email`] = []database.Row{
		{"id": "1", "email": "user@example.com"},
	}
	s := newTestServer(mock)
	token := generateCSRFToken()
	req := httptest.NewRequest("POST", "/forgot-password", strings.NewReader("_csrf="+url.QueryEscape(token)+"&email=user@example.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.TLS = &tls.ConnectionState{} // simulate HTTPS
	rec := httptest.NewRecorder()
	s.handleForgotPassword(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
}

func TestHandleForgotPassword_ParseFormError(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	req := httptest.NewRequest("POST", "/forgot-password", strings.NewReader("% invalid form data"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleForgotPassword(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "error=") {
		t.Errorf("expected error redirect, got %q", loc)
	}
}

func TestHandleForgotPassword_WithSMTP(t *testing.T) {
	srv := newFakeSMTPServer(false)
	srv.start()
	defer srv.stop()

	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT id, email FROM "users" WHERE email = :email`] = []database.Row{
		{"id": "1", "email": "user@example.com"},
	}
	s := newTestServer(mock)
	token := generateCSRFToken()

	host, port, _ := net.SplitHostPort(srv.addr)
	_ = host
	t.Setenv("KILNX_SMTP_HOST", "127.0.0.1")
	t.Setenv("KILNX_SMTP_PORT", port)
	t.Setenv("KILNX_SMTP_FROM", "sender@example.com")

	req := httptest.NewRequest("POST", "/forgot-password", strings.NewReader("_csrf="+url.QueryEscape(token)+"&email=user@example.com"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleForgotPassword(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "sent=1") {
		t.Errorf("expected sent=1 redirect, got %q", loc)
	}
	// Allow goroutine to finish
	time.Sleep(100 * time.Millisecond)
	srv.mu.Lock()
	data := srv.data.String()
	srv.mu.Unlock()
	if !strings.Contains(data, "Reset Password") {
		t.Errorf("expected email body to contain reset link, got %q", data)
	}
}

// ---------- handleResetPassword ----------

func TestHandleResetPassword_GET(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("GET", "/reset-password?token=abc", nil)
	rec := httptest.NewRecorder()
	s.handleResetPassword(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("code = %d, want 405", rec.Code)
	}
}

func TestHandleResetPassword_NoToken(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	req := httptest.NewRequest("POST", "/reset-password", nil)
	rec := httptest.NewRecorder()
	s.handleResetPassword(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "/forgot-password") {
		t.Errorf("expected redirect to forgot, got %q", loc)
	}
}

func TestHandleResetPassword_ShortPassword(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	token := generateCSRFToken()
	req := httptest.NewRequest("POST", "/reset-password?token=abc", strings.NewReader("_csrf="+url.QueryEscape(token)+"&password=short&password_confirm=short"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleResetPassword(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "error=") {
		t.Errorf("expected error redirect, got %q", loc)
	}
}

func TestHandleResetPassword_Mismatch(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	token := generateCSRFToken()
	req := httptest.NewRequest("POST", "/reset-password?token=abc", strings.NewReader("_csrf="+url.QueryEscape(token)+"&password=password123&password_confirm=different"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleResetPassword(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "error=") {
		t.Errorf("expected error redirect, got %q", loc)
	}
}

func TestHandleResetPassword_InvalidToken(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT email FROM _kilnx_password_resets WHERE token = :token AND expires_at > datetime('now')`] = nil
	s := newTestServer(mock)
	token := generateCSRFToken()
	req := httptest.NewRequest("POST", "/reset-password?token=bad", strings.NewReader("_csrf="+url.QueryEscape(token)+"&password=password123&password_confirm=password123"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleResetPassword(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "error=") {
		t.Errorf("expected error redirect, got %q", loc)
	}
}

func TestHandleResetPassword_Success(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT email FROM _kilnx_password_resets WHERE token = :token AND expires_at > datetime('now')`] = []database.Row{
		{"email": "user@example.com"},
	}
	s := newTestServer(mock)
	token := generateCSRFToken()
	req := httptest.NewRequest("POST", "/reset-password?token=valid", strings.NewReader("_csrf="+url.QueryEscape(token)+"&password=password123&password_confirm=password123"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleResetPassword(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "reset=1") {
		t.Errorf("expected reset=1 redirect, got %q", loc)
	}
	// Verify password update and token deletion
	foundUpdate := false
	foundDelete := false
	for _, call := range mock.execCalled {
		if strings.Contains(call.SQL, "UPDATE \"users\"") {
			foundUpdate = true
		}
		if strings.Contains(call.SQL, "DELETE FROM _kilnx_password_resets") {
			foundDelete = true
		}
	}
	if !foundUpdate {
		t.Error("expected UPDATE users")
	}
	if !foundDelete {
		t.Error("expected DELETE from _kilnx_password_resets")
	}
}

func TestHandleResetPassword_ParseFormError(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT email FROM _kilnx_password_resets WHERE token = :token AND expires_at > datetime('now')`] = []database.Row{
		{"email": "user@example.com"},
	}
	s := newTestServer(mock)
	req := httptest.NewRequest("POST", "/reset-password?token=valid", strings.NewReader("% invalid form data"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleResetPassword(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "error=") {
		t.Errorf("expected error redirect, got %q", loc)
	}
}

func TestHandleResetPassword_UpdateError(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT email FROM _kilnx_password_resets WHERE token = :token AND expires_at > datetime('now')`] = []database.Row{
		{"email": "user@example.com"},
	}
	mock.execErr[`UPDATE "users" SET password_hash = :password WHERE email = :email`] = fmt.Errorf("db error")
	s := newTestServer(mock)
	token := generateCSRFToken()
	req := httptest.NewRequest("POST", "/reset-password?token=valid", strings.NewReader("_csrf="+url.QueryEscape(token)+"&password=password123&password_confirm=password123"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleResetPassword(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "error=") {
		t.Errorf("expected error redirect, got %q", loc)
	}
}

// ---------- renderFragment ----------

func TestRenderFragment_TextOnly(t *testing.T) {
	s := newTestServer(nil)
	frag := parser.Page{
		Body: []parser.Node{
			{Type: parser.NodeText, Value: "Hello World"},
		},
	}
	req := httptest.NewRequest("GET", "/frag", nil)
	got := s.renderFragment(frag, req)
	if !strings.Contains(got, "Hello World") {
		t.Errorf("expected text, got %q", got)
	}
}

func TestRenderFragment_HTML(t *testing.T) {
	s := newTestServer(nil)
	frag := parser.Page{
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: "<div>{test}</div>"},
		},
	}
	req := httptest.NewRequest("GET", "/frag", nil)
	got := s.renderFragment(frag, req)
	if !strings.Contains(got, "<div>") {
		t.Errorf("expected HTML, got %q", got)
	}
}

func TestRenderFragment_Query(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT id, name FROM users WHERE active = 1`] = []database.Row{
		{"id": "1", "name": "Alice"},
		{"id": "2", "name": "Bob"},
	}
	s := newTestServer(mock)
	frag := parser.Page{
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT id, name FROM users WHERE active = 1`, Name: "users"},
			{Type: parser.NodeHTML, HTMLContent: `<ul>{{each users}}<li>{name}</li>{{end}}</ul>`},
		},
	}
	req := httptest.NewRequest("GET", "/frag", nil)
	got := s.renderFragment(frag, req)
	if !strings.Contains(got, "Alice") || !strings.Contains(got, "Bob") {
		t.Errorf("expected query results, got %q", got)
	}
}

func TestRenderFragment_QueryError(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsErr[`SELECT bad`] = fmt.Errorf("syntax error")
	s := newTestServer(mock)
	frag := parser.Page{
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT bad`, Name: "bad"},
		},
	}
	req := httptest.NewRequest("GET", "/frag", nil)
	got := s.renderFragment(frag, req)
	if !strings.Contains(got, "Query error") {
		t.Errorf("expected error message, got %q", got)
	}
}

func TestRenderFragment_QueryRejected(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	// Set up tenant guard that rejects everything
	s.tenants = TenantMap{"users": "tenant_id"}
	frag := parser.Page{
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT * FROM users`, Name: "users"},
		},
	}
	req := httptest.NewRequest("GET", "/frag", nil)
	got := s.renderFragment(frag, req)
	if got != "" {
		t.Errorf("expected empty when query rejected, got %q", got)
	}
}

func TestRenderFragment_Paginate(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT COUNT(*) as _count FROM (SELECT id FROM users)`] = []database.Row{
		{"_count": "25"},
	}
	mock.queryRowsWithParamsResults[`SELECT id FROM users LIMIT 10 OFFSET 10`] = []database.Row{
		{"id": "3"}, {"id": "4"},
	}
	s := newTestServer(mock)
	frag := parser.Page{
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT id FROM users`, Name: "users", Paginate: 10},
			{Type: parser.NodeHTML, HTMLContent: `<div>{{each users}}{id}{{end}}</div>`},
		},
	}
	req := httptest.NewRequest("GET", "/frag?page=2", nil)
	got := s.renderFragment(frag, req)
	if !strings.Contains(got, "3") || !strings.Contains(got, "4") {
		t.Errorf("expected paginated results, got %q", got)
	}
}

// ---------- handleActionNodes ----------

func TestHandleActionNodes_Redirect(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("POST", "/action", nil)
	rec := httptest.NewRecorder()
	s.handleActionNodes(rec, req, []parser.Node{
		{Type: parser.NodeRedirect, Value: "/success"},
	}, map[string]string{}, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if loc != "/success" {
		t.Errorf("location = %q, want /success", loc)
	}
}

func TestHandleActionNodes_RedirectHX(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("POST", "/action", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	s.handleActionNodes(rec, req, []parser.Node{
		{Type: parser.NodeRedirect, Value: "/success"},
	}, map[string]string{}, s.getApp())
	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", rec.Code)
	}
	if rec.Header().Get("HX-Redirect") != "/success" {
		t.Errorf("hx-redirect = %q, want /success", rec.Header().Get("HX-Redirect"))
	}
}

func TestHandleActionNodes_RedirectWithParams(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("POST", "/action", nil)
	rec := httptest.NewRecorder()
	s.handleActionNodes(rec, req, []parser.Node{
		{Type: parser.NodeRedirect, Value: "/user/:id"},
	}, map[string]string{"id": "42"}, s.getApp())
	loc := rec.Header().Get("Location")
	if loc != "/user/42" {
		t.Errorf("location = %q, want /user/42", loc)
	}
}

func TestHandleActionNodes_Respond(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("POST", "/action", nil)
	rec := httptest.NewRecorder()
	s.handleActionNodes(rec, req, []parser.Node{
		{Type: parser.NodeRespond, StatusCode: 201},
	}, map[string]string{}, s.getApp())
	if rec.Code != 201 {
		t.Errorf("code = %d, want 201", rec.Code)
	}
}

func TestHandleActionNodes_RespondDefault(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("POST", "/action", nil)
	rec := httptest.NewRecorder()
	s.handleActionNodes(rec, req, []parser.Node{
		{Type: parser.NodeRespond},
	}, map[string]string{}, s.getApp())
	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("content-type = %q, want text/html", ct)
	}
}

func TestHandleActionNodes_QuerySelect(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT name FROM users WHERE id = :id`] = []database.Row{
		{"name": "Alice"},
	}
	s := newTestServer(mock)
	req := httptest.NewRequest("POST", "/action", nil)
	rec := httptest.NewRecorder()
	formData := map[string]string{"id": "1"}
	s.handleActionNodes(rec, req, []parser.Node{
		{Type: parser.NodeQuery, SQL: `SELECT name FROM users WHERE id = :id`, Name: "user"},
	}, formData, s.getApp())
	if formData["user.name"] != "Alice" {
		t.Errorf("user.name = %q, want Alice", formData["user.name"])
	}
}

func TestHandleActionNodes_QueryMutation(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	req := httptest.NewRequest("POST", "/action", nil)
	rec := httptest.NewRecorder()
	formData := map[string]string{"name": "New"}
	s.handleActionNodes(rec, req, []parser.Node{
		{Type: parser.NodeQuery, SQL: `INSERT INTO users (name) VALUES (:name)`},
	}, formData, s.getApp())
	found := false
	for _, call := range mock.execCalled {
		if strings.Contains(call.SQL, "INSERT INTO users") {
			found = true
		}
	}
	if !found {
		t.Error("expected INSERT to be executed")
	}
}

func TestHandleActionNodes_QueryError(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsErr[`SELECT bad`] = fmt.Errorf("syntax error")
	s := newTestServer(mock)
	req := httptest.NewRequest("POST", "/action", nil)
	rec := httptest.NewRecorder()
	s.handleActionNodes(rec, req, []parser.Node{
		{Type: parser.NodeQuery, SQL: `SELECT bad`, Name: "bad"},
	}, map[string]string{}, s.getApp())
	// Should continue after error, no panic
}

func TestHandleActionNodes_ValidateInline(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("POST", "/action", nil)
	rec := httptest.NewRecorder()
	s.handleActionNodes(rec, req, []parser.Node{
		{Type: parser.NodeValidate, Validations: []parser.Validation{
			{Field: "email", Rules: []string{"required"}},
		}},
	}, map[string]string{}, s.getApp())
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("code = %d, want 422", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Email is required") {
		t.Errorf("expected error message, got %q", body)
	}
}

func TestHandleActionNodes_ValidatePass(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("POST", "/action", nil)
	rec := httptest.NewRecorder()
	s.handleActionNodes(rec, req, []parser.Node{
		{Type: parser.NodeValidate, Validations: []parser.Validation{
			{Field: "email", Rules: []string{"required"}},
		}},
	}, map[string]string{"email": "test@example.com"}, s.getApp())
	if rec.Body.Len() != 0 { // No response body written
		t.Errorf("body should be empty, got %q", rec.Body.String())
	}
}

func TestHandleActionNodes_SendEmail(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT email FROM users WHERE id = :id`] = []database.Row{
		{"email": "user@example.com"},
	}
	s := newTestServer(mock)
	req := httptest.NewRequest("POST", "/action", nil)
	rec := httptest.NewRecorder()
	s.handleActionNodes(rec, req, []parser.Node{
		{Type: parser.NodeSendEmail, EmailTo: ":email", EmailSubject: "Hello", Props: map[string]string{
			"to_query": `SELECT email FROM users WHERE id = :id`,
			"body":     "Welcome!",
		}},
	}, map[string]string{"email": "fallback@example.com", "id": "1"}, s.getApp())
	// Email is sent in a goroutine, just verify no panic
}

func TestHandleActionNodes_Enqueue(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	app := s.getApp()
	app.Jobs = []parser.Job{
		{Name: "notify", MaxRetries: 3},
	}
	s.jobQueue = NewJobQueue(s)
	req := httptest.NewRequest("POST", "/action", nil)
	rec := httptest.NewRecorder()
	s.handleActionNodes(rec, req, []parser.Node{
		{Type: parser.NodeEnqueue, JobName: "notify", JobParams: map[string]string{
			"user_id": ":id",
			"msg":     "hello",
		}},
	}, map[string]string{"id": "42"}, s.getApp())
	// Enqueue is async, just verify no panic
}

func TestHandleActionNodes_EnqueueNoQueue(t *testing.T) {
	s := newTestServer(nil)
	s.jobQueue = nil
	req := httptest.NewRequest("POST", "/action", nil)
	rec := httptest.NewRecorder()
	s.handleActionNodes(rec, req, []parser.Node{
		{Type: parser.NodeEnqueue, JobName: "notify", JobParams: map[string]string{
			"user_id": ":id",
		}},
	}, map[string]string{"id": "42"}, s.getApp())
	// Should not panic when jobQueue is nil
}

func TestHandleActionNodes_QueryMutationError(t *testing.T) {
	mock := newMockExecutor()
	mock.execErr[`INSERT INTO users (name) VALUES (:name)`] = fmt.Errorf("constraint failed")
	s := newTestServer(mock)
	req := httptest.NewRequest("POST", "/action", nil)
	rec := httptest.NewRecorder()
	s.handleActionNodes(rec, req, []parser.Node{
		{Type: parser.NodeQuery, SQL: `INSERT INTO users (name) VALUES (:name)`},
	}, map[string]string{"name": "New"}, s.getApp())
	// Should log error and continue
}

func TestHandleActionNodes_ValidateModel(t *testing.T) {
	s := newTestServer(nil)
	app := s.getApp()
	app.Models = []parser.Model{{
		Name: "contact",
		Fields: []parser.Field{
			{Name: "email", Type: parser.FieldEmail, Required: true},
		},
	}}
	req := httptest.NewRequest("POST", "/action", nil)
	rec := httptest.NewRecorder()
	s.handleActionNodes(rec, req, []parser.Node{
		{Type: parser.NodeValidate, ModelName: "contact"},
	}, map[string]string{}, s.getApp())
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("code = %d, want 422", rec.Code)
	}
}

func TestHandleActionNodes_SendEmailNoToQuery(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("POST", "/action", nil)
	rec := httptest.NewRecorder()
	s.handleActionNodes(rec, req, []parser.Node{
		{Type: parser.NodeSendEmail, EmailTo: "user@example.com", EmailSubject: "Hello"},
	}, map[string]string{}, s.getApp())
	// Should send email without to_query
}

func TestHandleActionNodes_FetchError(t *testing.T) {
	s := newTestServer(nil)
	req := httptest.NewRequest("POST", "/action", nil)
	rec := httptest.NewRecorder()
	s.handleActionNodes(rec, req, []parser.Node{
		{Type: parser.NodeFetch, FetchURL: "http://invalid.localhost:99999", Name: "data"},
	}, map[string]string{}, s.getApp())
	// Should log error and continue
}

func TestHandleActionNodes_QueryTenantError(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	s.tenants = TenantMap{"orders": "org"}
	req := httptest.NewRequest("POST", "/action", nil)
	rec := httptest.NewRecorder()
	s.handleActionNodes(rec, req, []parser.Node{
		{Type: parser.NodeQuery, SQL: `SELECT * FROM orders`},
	}, map[string]string{}, s.getApp())
	// Should log security and continue
}

func TestHandleActionNodes_EnqueueMissingParam(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	app := s.getApp()
	app.Jobs = []parser.Job{{Name: "notify"}}
	s.jobQueue = NewJobQueue(s)
	req := httptest.NewRequest("POST", "/action", nil)
	rec := httptest.NewRecorder()
	s.handleActionNodes(rec, req, []parser.Node{
		{Type: parser.NodeEnqueue, JobName: "notify", JobParams: map[string]string{
			"user_id": ":missing",
		}},
	}, map[string]string{}, s.getApp())
	// Should fall through to literal value when param missing
}

// ---------- handleLogin ----------

func TestHandleLogin_Success(t *testing.T) {
	hash, _ := HashPassword("secret123")
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT * FROM "users" WHERE "email" = :identity`] = []database.Row{
		{"id": "1", "email": "user@example.com", "password_hash": hash, "role": "viewer"},
	}
	s := newTestServer(mock)
	csrf := generateCSRFToken()

	form := url.Values{"_csrf": {csrf}, "identity": {"user@example.com"}, "password": {"secret123"}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	s.handleLogin(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	// Should set session cookie
	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "_kilnx_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Error("expected session cookie to be set")
	}
}

func TestHandleLogin_InvalidCredentials(t *testing.T) {
	hash, _ := HashPassword("secret123")
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT * FROM "users" WHERE "email" = :identity`] = []database.Row{
		{"id": "1", "email": "user@example.com", "password_hash": hash, "role": "viewer"},
	}
	s := newTestServer(mock)
	csrf := generateCSRFToken()

	form := url.Values{"_csrf": {csrf}, "identity": {"user@example.com"}, "password": {"wrongpass"}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	s.handleLogin(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "error=Invalid+credentials") {
		t.Errorf("expected redirect with error, got %q", loc)
	}
}

func TestHandleLogin_MissingFields(t *testing.T) {
	s := newTestServer(nil)
	csrf := generateCSRFToken()

	form := url.Values{"_csrf": {csrf}, "identity": {""}, "password": {""}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	s.handleLogin(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "error=All+fields+are+required") {
		t.Errorf("expected redirect with missing fields error, got %q", loc)
	}
}

func TestHandleLogin_InvalidCSRF(t *testing.T) {
	s := newTestServer(nil)
	form := url.Values{"_csrf": {"bad-token"}, "identity": {"user@example.com"}, "password": {"secret123"}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	s.handleLogin(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", rec.Code)
	}
}

func TestHandleLogin_NoAuthConfig(t *testing.T) {
	s := newTestServer(nil)
	s.app.Auth = nil
	req := httptest.NewRequest("POST", "/login", nil)
	rec := httptest.NewRecorder()

	s.handleLogin(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", rec.Code)
	}
}

func TestHandleLogin_UserNotFound(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT * FROM "users" WHERE "email" = :identity`] = []database.Row{}
	s := newTestServer(mock)
	csrf := generateCSRFToken()

	form := url.Values{"_csrf": {csrf}, "identity": {"nobody@example.com"}, "password": {"secret123"}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	s.handleLogin(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "error=Invalid+credentials") {
		t.Errorf("expected redirect with error, got %q", loc)
	}
}

func TestHandleLogin_NextRedirect(t *testing.T) {
	hash, _ := HashPassword("secret123")
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT * FROM "users" WHERE "email" = :identity`] = []database.Row{
		{"id": "1", "email": "user@example.com", "password_hash": hash, "role": "viewer"},
	}
	s := newTestServer(mock)
	s.app.Auth.AfterLogin = "/dashboard"
	csrf := generateCSRFToken()

	form := url.Values{"_csrf": {csrf}, "identity": {"user@example.com"}, "password": {"secret123"}}
	req := httptest.NewRequest("POST", "/login?next=/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	s.handleLogin(rec, req)
	loc := rec.Header().Get("Location")
	if loc != "/profile" {
		t.Errorf("expected redirect to /profile, got %q", loc)
	}
}

func TestHandleLogin_NextRedirectInvalid(t *testing.T) {
	hash, _ := HashPassword("secret123")
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT * FROM "users" WHERE "email" = :identity`] = []database.Row{
		{"id": "1", "email": "user@example.com", "password_hash": hash, "role": "viewer"},
	}
	s := newTestServer(mock)
	s.app.Auth.AfterLogin = "/dashboard"
	csrf := generateCSRFToken()

	form := url.Values{"_csrf": {csrf}, "identity": {"user@example.com"}, "password": {"secret123"}}
	req := httptest.NewRequest("POST", "/login?next=https://evil.com", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	s.handleLogin(rec, req)
	loc := rec.Header().Get("Location")
	if loc != "/dashboard" {
		t.Errorf("expected redirect to /dashboard (open redirect prevented), got %q", loc)
	}
}

// ---------- handleRegister ----------

func TestHandleRegister_Success(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT id FROM "users" WHERE "email" = :identity`] = []database.Row{}
	s := newTestServer(mock)
	csrf := generateCSRFToken()

	form := url.Values{"_csrf": {csrf}, "identity": {"new@example.com"}, "password": {"secret123"}, "confirm_password": {"secret123"}}
	req := httptest.NewRequest("POST", "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	s.handleRegister(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "/login") {
		t.Errorf("expected redirect to login, got %q", loc)
	}
}

func TestHandleRegister_ShortPassword(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	csrf := generateCSRFToken()

	form := url.Values{"_csrf": {csrf}, "identity": {"new@example.com"}, "password": {"123"}, "confirm_password": {"123"}}
	req := httptest.NewRequest("POST", "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	s.handleRegister(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "error=Password+must+be+at+least+6+characters") {
		t.Errorf("expected short password error, got %q", loc)
	}
}

func TestHandleRegister_UserExists(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT id FROM "users" WHERE "email" = :identity`] = []database.Row{}
	mock.execErr[`INSERT INTO "users" (name, "email", "password_hash") VALUES (:name, :identity, :password)`] = fmt.Errorf("UNIQUE constraint failed")
	s := newTestServer(mock)
	csrf := generateCSRFToken()

	form := url.Values{"_csrf": {csrf}, "identity": {"existing@example.com"}, "password": {"secret123"}, "confirm_password": {"secret123"}}
	req := httptest.NewRequest("POST", "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	s.handleRegister(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "error=An+account+with+that+email+already+exists") {
		t.Errorf("expected user exists error, got %q", loc)
	}
}

func TestHandleRegister_MissingFields(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	csrf := generateCSRFToken()

	form := url.Values{"_csrf": {csrf}, "identity": {""}, "password": {""}, "confirm_password": {""}}
	req := httptest.NewRequest("POST", "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	s.handleRegister(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "error=All+fields+are+required") {
		t.Errorf("expected missing fields error, got %q", loc)
	}
}

func TestHandleRegister_InvalidCSRF(t *testing.T) {
	s := newTestServer(nil)
	form := url.Values{"_csrf": {"bad-token"}, "identity": {"new@example.com"}, "password": {"secret123"}, "confirm_password": {"secret123"}}
	req := httptest.NewRequest("POST", "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	s.handleRegister(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", rec.Code)
	}
}

func TestHandleRegister_NoAuthConfig(t *testing.T) {
	s := newTestServer(nil)
	s.app.Auth = nil
	req := httptest.NewRequest("POST", "/register", nil)
	rec := httptest.NewRecorder()

	s.handleRegister(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", rec.Code)
	}
}

func TestHandleRegister_DBError(t *testing.T) {
	mock := newMockExecutor()
	mock.execErr[`INSERT INTO "users" (name, "email", "password_hash") VALUES (:name, :identity, :password)`] = fmt.Errorf("disk full")
	s := newTestServer(mock)
	csrf := generateCSRFToken()

	form := url.Values{"_csrf": {csrf}, "identity": {"new@example.com"}, "password": {"secret123"}, "confirm_password": {"secret123"}}
	req := httptest.NewRequest("POST", "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	s.handleRegister(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "error=Could+not+create+account") {
		t.Errorf("expected db error, got %q", loc)
	}
}

// ---------- handleAction ----------

func TestHandleAction_InvalidCSRF(t *testing.T) {
	s := newTestServer(nil)
	form := url.Values{"_csrf": {"bad-token"}, "name": {"Alice"}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{Path: "/action", Body: []parser.Node{}}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", rec.Code)
	}
}

func TestHandleAction_InlineValidationFail(t *testing.T) {
	s := newTestServer(nil)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}, "email": {""}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeValidate, Validations: []parser.Validation{
				{Field: "email", Rules: []string{"required"}},
			}},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("code = %d, want 422", rec.Code)
	}
}

func TestHandleAction_ModelValidationFail(t *testing.T) {
	s := newTestServer(nil)
	s.app.Models = []parser.Model{
		{
			Name: "user",
			Fields: []parser.Field{
				{Name: "email", Type: "email", Required: true},
			},
		},
	}
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}, "email": {""}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeValidate, ModelName: "user"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("code = %d, want 422", rec.Code)
	}
}

func TestHandleAction_Redirect(t *testing.T) {
	s := newTestServer(nil)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeRedirect, Value: "/success"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	if rec.Header().Get("Location") != "/success" {
		t.Errorf("location = %q, want /success", rec.Header().Get("Location"))
	}
}

func TestHandleAction_Respond(t *testing.T) {
	s := newTestServer(nil)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeRespond, StatusCode: 201},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != 201 {
		t.Errorf("code = %d, want 201", rec.Code)
	}
}

func TestHandleAction_PathParams(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	if _, err := db.Conn().Exec("INSERT INTO users (id, name) VALUES (42, 'Alice')"); err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	s := newTestServer(db)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action/42", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action/:id",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT name FROM users WHERE id = :id`, Name: "user"},
			{Type: parser.NodeRedirect, Value: "/done"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	// Should proceed to redirect after query
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
}

func TestHandleAction_WithDB_Select(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	if _, err := db.Conn().Exec("INSERT INTO users (name) VALUES ('Alice'), ('Bob')"); err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	s := newTestServer(db)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT name FROM users WHERE name = :name`, Name: "user"},
			{Type: parser.NodeRedirect, Value: "/success"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
}

func TestHandleAction_WithDB_Mutation(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	s := newTestServer(db)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}, "name": {"Charlie"}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `INSERT INTO users (name) VALUES (:name)`},
			{Type: parser.NodeRedirect, Value: "/success"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}

	rows, _ := db.QueryRows("SELECT name FROM users")
	if len(rows) != 1 || rows[0]["name"] != "Charlie" {
		t.Errorf("expected Charlie to be inserted")
	}
}

func TestHandleAction_OnSuccess(t *testing.T) {
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
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT name FROM users WHERE name = 'Alice'`, Name: "user"},
			{Type: parser.NodeOn, Props: map[string]string{"condition": "success"}, Children: []parser.Node{
				{Type: parser.NodeRedirect, Value: "/success"},
			}},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	if rec.Header().Get("Location") != "/success" {
		t.Errorf("location = %q, want /success", rec.Header().Get("Location"))
	}
}

func TestHandleAction_OnError(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	s := newTestServer(db)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `INSERT INTO users (name) VALUES (NULL)`},
			{Type: parser.NodeOn, Props: map[string]string{"condition": "error"}, Children: []parser.Node{
				{Type: parser.NodeRedirect, Value: "/error-page"},
			}},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	if rec.Header().Get("Location") != "/error-page" {
		t.Errorf("location = %q, want /error-page", rec.Header().Get("Location"))
	}
}

func TestHandleAction_OnNotFound(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	s := newTestServer(db)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT name FROM users WHERE name = 'Nobody'`, Name: "user"},
			{Type: parser.NodeOn, Props: map[string]string{"condition": "not found"}, Children: []parser.Node{
				{Type: parser.NodeRedirect, Value: "/not-found-page"},
			}},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	if rec.Header().Get("Location") != "/not-found-page" {
		t.Errorf("location = %q, want /not-found-page", rec.Header().Get("Location"))
	}
}

func TestHandleAction_DefaultRedirect(t *testing.T) {
	s := newTestServer(nil)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "/origin")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	if rec.Header().Get("Location") != "/origin" {
		t.Errorf("location = %q, want /origin", rec.Header().Get("Location"))
	}
}

func TestHandleAction_SendEmail(t *testing.T) {
	s := newTestServer(nil)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}, "email": {"user@example.com"}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeSendEmail, EmailTo: ":email", EmailSubject: "Hello", Props: map[string]string{
				"body": "Welcome!",
			}},
			{Type: parser.NodeRedirect, Value: "/done"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
}

func TestHandleAction_Enqueue(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	s := newTestServer(db)
	s.app.Jobs = []parser.Job{{Name: "notify", MaxRetries: 3}}
	s.jobQueue = NewJobQueue(s)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}, "id": {"42"}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeEnqueue, JobName: "notify", JobParams: map[string]string{
				"user_id": ":id",
			}},
			{Type: parser.NodeRedirect, Value: "/done"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
}

func TestHandleAction_ValidateInlinePass(t *testing.T) {
	s := newTestServer(nil)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}, "email": {"user@example.com"}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeValidate, Validations: []parser.Validation{
				{Field: "email", Rules: []string{"required", "is", "email"}},
			}},
			{Type: parser.NodeRedirect, Value: "/done"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
}

func TestHandleAction_EnqueueLiteralParam(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()
	if _, err := db.Conn().Exec(`CREATE TABLE _kilnx_jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		params TEXT NOT NULL DEFAULT '{}',
		state TEXT NOT NULL DEFAULT 'available',
		attempts INTEGER NOT NULL DEFAULT 0,
		max_attempts INTEGER NOT NULL DEFAULT 1,
		scheduled_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		started_at DATETIME,
		completed_at DATETIME,
		last_error TEXT
	)`); err != nil {
		t.Fatalf("failed to create jobs table: %v", err)
	}

	s := newTestServer(db)
	s.app.Jobs = []parser.Job{{Name: "notify", MaxRetries: 3}}
	s.jobQueue = NewJobQueue(s)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeEnqueue, JobName: "notify", JobParams: map[string]string{
				"channel": "email",
			}},
			{Type: parser.NodeRedirect, Value: "/done"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
}

func TestHandleAction_BeginTxError(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeRedirect, Value: "/done"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", rec.Code)
	}
}

func TestHandleAction_QueryNoName(t *testing.T) {
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
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT name FROM users`},
			{Type: parser.NodeRedirect, Value: "/done"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
}

func TestHandleAction_OnForbidden(t *testing.T) {
	s := newTestServer(nil)
	s.app.Permissions = []parser.Permission{
		{Role: "admin", Rules: []string{"edit"}},
	}
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path:         "/action",
		RequiresRole: "admin",
		Body: []parser.Node{
			{Type: parser.NodeOn, Props: map[string]string{"condition": "forbidden"}, Children: []parser.Node{
				{Type: parser.NodeRedirect, Value: "/forbidden-page"},
			}},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	if rec.Header().Get("Location") != "/forbidden-page" {
		t.Errorf("location = %q, want /forbidden-page", rec.Header().Get("Location"))
	}
}

func TestHandleAction_OnForbiddenWithSession(t *testing.T) {
	s := newTestServer(nil)
	s.app.Permissions = []parser.Permission{
		{Role: "admin", Rules: []string{"edit"}},
	}
	// Create a session for a viewer user
	session := &Session{UserID: "1", Identity: "user@example.com", Role: "viewer", ExpiresAt: time.Now().Add(time.Hour)}
	s.sessions.sessions["session-viewer"] = session
	cookieVal := s.sessions.signSessionID("session-viewer")
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "_kilnx_session="+cookieVal)
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path:         "/action",
		RequiresRole: "admin",
		Body: []parser.Node{
			{Type: parser.NodeOn, Props: map[string]string{"condition": "forbidden"}, Children: []parser.Node{
				{Type: parser.NodeRedirect, Value: "/forbidden-page"},
			}},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	if rec.Header().Get("Location") != "/forbidden-page" {
		t.Errorf("location = %q, want /forbidden-page", rec.Header().Get("Location"))
	}
}

func TestHandleAction_SendEmailWithToQuery(t *testing.T) {
	db, err := database.Open("/tmp/test_email.db")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer os.Remove("/tmp/test_email.db")
	defer db.Close()

	if _, err := db.Conn().Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	if _, err := db.Conn().Exec("INSERT INTO users (id, email) VALUES (1, 'queried@example.com')"); err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	s := newTestServer(db)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}, "id": {"1"}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeSendEmail, EmailTo: ":email", EmailSubject: "Hello", Props: map[string]string{
				"to_query": `SELECT email FROM users WHERE id = :id`,
				"body":     "Welcome!",
			}},
			{Type: parser.NodeRedirect, Value: "/done"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
}

func TestHandleAction_HTMLNode(t *testing.T) {
	s := newTestServer(nil)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<div>Hello</div>`},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	// HTML node writes response directly, no redirect
	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Hello") {
		t.Errorf("expected Hello in body, got %q", rec.Body.String())
	}
}

func TestHandleAction_RespondWithQuery(t *testing.T) {
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
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT name FROM users`, Name: "users"},
			{Type: parser.NodeRespond, QuerySQL: `SELECT name FROM users`, StatusCode: 200},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", rec.Code)
	}
}

func TestHandleAction_RespondSwapDelete(t *testing.T) {
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
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT name FROM users`, Name: "users"},
			{Type: parser.NodeRespond, RespondSwap: "delete"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", rec.Code)
	}
	if rec.Header().Get("HX-Reswap") != "delete" {
		t.Errorf("hx-reswap = %q, want delete", rec.Header().Get("HX-Reswap"))
	}
}

func TestHandleAction_RespondWithTargetFragment(t *testing.T) {
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
	s.app.Fragments = []parser.Page{
		{Path: "/user-list", Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT name FROM users`, Name: "users"},
			{Type: parser.NodeHTML, HTMLContent: `<ul>{{each users}}<li>{name}</li>{{end}}</ul>`},
		}},
	}
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT name FROM users`, Name: "users"},
			{Type: parser.NodeRespond, RespondTarget: "user-list", QuerySQL: `SELECT name FROM users`},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Alice") {
		t.Errorf("expected Alice in response, got %q", body)
	}
}

func TestHandleAction_FetchNode(t *testing.T) {
	// Start a simple HTTP server to fetch from
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id": 1, "title": "Test Post"}`))
	}))
	defer ts.Close()

	s := newTestServer(nil)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeFetch, FetchURL: ts.URL, Name: "post"},
			{Type: parser.NodeRedirect, Value: "/done"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
}

func TestHandleAction_FetchNodeWithVar(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id": 1, "title": "Test Post"}`))
	}))
	defer ts.Close()

	s := newTestServer(nil)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeFetch, FetchURL: ts.URL, Name: "post", Props: map[string]string{"var": "fetched_post"}},
			{Type: parser.NodeRedirect, Value: "/done"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
}

func TestHandleAction_LLMNode(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec("CREATE TABLE prompts (id INTEGER PRIMARY KEY, content TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	if _, err := db.Conn().Exec("INSERT INTO prompts (content) VALUES ('Hello AI')"); err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	s := newTestServer(db)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT content FROM prompts`, Name: "prompt"},
			{Type: parser.NodeLLM, SQL: "Say hello", Name: "response", Props: map[string]string{
				"prompt_query": "prompt",
			}},
			{Type: parser.NodeRedirect, Value: "/done"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	// LLM may fail without API key, but should not panic
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
}

func TestHandleAction_QueryErrorNoOnError(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	s := newTestServer(db)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `INSERT INTO users (name) VALUES (NULL)`},
			// No on error handler
			{Type: parser.NodeRedirect, Value: "/done"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", rec.Code)
	}
}

func TestHandleAction_SendEmailEmptyBody(t *testing.T) {
	s := newTestServer(nil)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}, "email": {"user@example.com"}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeSendEmail, EmailTo: ":email", EmailSubject: "Hello"},
			{Type: parser.NodeRedirect, Value: "/done"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
}

func TestHandleAction_OnForbiddenRequiresClauses(t *testing.T) {
	s := newTestServer(nil)
	s.app.Permissions = []parser.Permission{
		{Role: "admin", Rules: []string{"edit"}},
	}
	// Create a session that passes auth but not the specific clause
	session := &Session{UserID: "1", Identity: "user@example.com", Role: "viewer", ExpiresAt: time.Now().Add(time.Hour)}
	s.sessions.sessions["session-viewer"] = session
	cookieVal := s.sessions.signSessionID("session-viewer")
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "_kilnx_session="+cookieVal)
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		RequiresClauses: []parser.RequiresClause{
			{Kind: parser.RequiresClauseExpr, Value: "current_user.role == 'admin'"},
		},
		Body: []parser.Node{
			{Type: parser.NodeOn, Props: map[string]string{"condition": "forbidden"}, Children: []parser.Node{
				{Type: parser.NodeRedirect, Value: "/forbidden-page"},
			}},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
	if rec.Header().Get("Location") != "/forbidden-page" {
		t.Errorf("location = %q, want /forbidden-page", rec.Header().Get("Location"))
	}
}

func TestHandleAction_SendEmailToQueryTenantReject(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	s := newTestServer(db)
	s.tenants = TenantMap{"users": "org_id"}
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}, "id": {"1"}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeSendEmail, EmailTo: ":email", EmailSubject: "Hello", Props: map[string]string{
				"to_query": `SELECT email FROM users WHERE id = :id`,
				"body":     "Welcome!",
			}},
			{Type: parser.NodeRedirect, Value: "/done"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	// Should continue after tenant rejection, proceed to redirect
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
}

func TestHandleAction_GeneratePDF(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	if _, err := db.Conn().Exec("INSERT INTO users (name) VALUES ('Alice'), ('Bob')"); err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	s := newTestServer(db)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT name FROM users`, Name: "users"},
			{Type: parser.NodeGeneratePDF, TemplateName: "Report", DataQueryName: "users"},
			{Type: parser.NodeRedirect, Value: "/done"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
}

func TestHandleAction_GeneratePDFTenantRejection(t *testing.T) {
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
	s.tenants = TenantMap{"users": "org_id"}
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT name FROM users`, Name: "users"},
			{Type: parser.NodeGeneratePDF, TemplateName: "Report", DataQueryName: "users"},
			{Type: parser.NodeRedirect, Value: "/done"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
}

func TestHandleAction_GeneratePDFNoDataQuery(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	s := newTestServer(db)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeGeneratePDF, TemplateName: "Report"},
			{Type: parser.NodeRedirect, Value: "/done"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
}

func TestHandleAction_FetchNodeDefaultName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ts.Close()

	s := newTestServer(nil)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeFetch, FetchURL: ts.URL},
			{Type: parser.NodeRedirect, Value: "/done"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
}

func TestHandleAction_LLMNodeDefaultName(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec("CREATE TABLE prompts (id INTEGER PRIMARY KEY, content TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	if _, err := db.Conn().Exec("INSERT INTO prompts (content) VALUES ('Hello AI')"); err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	s := newTestServer(db)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT content FROM prompts`, Name: "prompt"},
			{Type: parser.NodeLLM, SQL: "Say hello", Props: map[string]string{
				"prompt_query": "prompt",
			}},
			{Type: parser.NodeRedirect, Value: "/done"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
}

func TestHandleAction_LLMWithTx(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec("CREATE TABLE prompts (id INTEGER PRIMARY KEY, content TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	if _, err := db.Conn().Exec("INSERT INTO prompts (content) VALUES ('Say hi')"); err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	s := newTestServer(db)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT content FROM prompts`, Name: "prompt"},
			{Type: parser.NodeLLM, SQL: "Say hello", Name: "response", Props: map[string]string{
				"prompt_query": "prompt",
			}},
			{Type: parser.NodeRedirect, Value: "/done"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	// LLM may fail without API key, but should not panic and tx path should be covered
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
}

func TestHandleAction_RespondWithQueryTenantReject(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, tenant_id INTEGER)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	s := newTestServer(db)
	s.tenants = map[string]string{"users": "tenant_id"}
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeRespond, QuerySQL: `SELECT * FROM users`},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", rec.Code)
	}
}

func TestHandleAction_RespondWithQueryError(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	s := newTestServer(db)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeRespond, QuerySQL: `SELECT bad FROM missing`},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", rec.Code)
	}
}

func TestHandleAction_RespondWithQueryFallback(t *testing.T) {
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
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeRespond, QuerySQL: `SELECT name FROM users`},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Alice") {
		t.Errorf("expected Alice in response, got %q", body)
	}
}

func TestHandleAction_RespondStatusCode(t *testing.T) {
	s := newTestServer(nil)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeRespond, StatusCode: 204},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != 204 {
		t.Errorf("code = %d, want 204", rec.Code)
	}
}

func TestHandleAction_DefaultRedirectNonLocal(t *testing.T) {
	s := newTestServer(nil)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "evil.com/steal")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{},
	}
	s.handleAction(rec, req, action, s.getApp())
	loc := rec.Header().Get("Location")
	if loc != "/" {
		t.Errorf("location = %q, want /", loc)
	}
}

func TestHandleAction_FetchNodeError(t *testing.T) {
	// Transport-level fetch failures (DNS, connection refused, timeout)
	// abort the action with 502 and roll back the implicit transaction.
	// Silent failure used to commit partial state, masking real outages.
	s := newTestServer(nil)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeFetch, FetchURL: "http://invalid.localhost:99999", Name: "data"},
			{Type: parser.NodeRedirect, Value: "/done"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusBadGateway {
		t.Errorf("code = %d, want 502", rec.Code)
	}
}

func TestHandleAction_HTMLNodeWithTx(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	s := newTestServer(db)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<p>Hello</p>`},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Hello") {
		t.Errorf("expected Hello, got %q", rec.Body.String())
	}
}

func TestHandleAction_RespondEmpty(t *testing.T) {
	s := newTestServer(nil)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeRespond},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", rec.Code)
	}
}

func TestHandleAction_DefaultRedirectWithQuery(t *testing.T) {
	s := newTestServer(nil)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "/previous?foo=bar")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{},
	}
	s.handleAction(rec, req, action, s.getApp())
	loc := rec.Header().Get("Location")
	if loc != "/previous?foo=bar" {
		t.Errorf("location = %q, want /previous?foo=bar", loc)
	}
}

func TestHandleForgotPassword_InvalidCSRF(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	form := url.Values{"_csrf": {"bad-token"}, "email": {"user@example.com"}}
	req := httptest.NewRequest("POST", "/forgot-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	s.handleForgotPassword(rec, req)
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "error=Invalid+CSRF+token") {
		t.Errorf("expected CSRF error redirect, got %q", loc)
	}
}

func TestHandleResetPassword_InvalidCSRF(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	form := url.Values{"_csrf": {"bad-token"}, "token": {"abc"}, "password": {"secret123"}, "password_confirm": {"secret123"}}
	req := httptest.NewRequest("POST", "/reset-password?token=abc", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	s.handleResetPassword(rec, req)
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "error=Invalid+CSRF+token") {
		t.Errorf("expected CSRF error redirect, got %q", loc)
	}
}

func TestHandleResetPassword_NoAuth(t *testing.T) {
	s := &Server{
		app:      &parser.App{},
		sessions: NewSessionStore("secret"),
	}
	req := httptest.NewRequest("POST", "/reset-password", nil)
	rec := httptest.NewRecorder()
	s.handleResetPassword(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", rec.Code)
	}
}

func TestHandleResetPassword_NoDB(t *testing.T) {
	s := newTestServer(nil)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}, "token": {"abc"}, "password": {"secret123"}, "confirm_password": {"secret123"}}
	req := httptest.NewRequest("POST", "/reset-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	s.handleResetPassword(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", rec.Code)
	}
}

func TestRenderFragment_QueryNoName(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT id FROM users`] = []database.Row{
		{"id": "1"}, {"id": "2"},
	}
	s := newTestServer(mock)
	frag := parser.Page{
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT id FROM users`},
			{Type: parser.NodeHTML, HTMLContent: `<div>{{each _last}}{id}{{end}}</div>`},
		},
	}
	req := httptest.NewRequest("GET", "/frag", nil)
	got := s.renderFragment(frag, req)
	if !strings.Contains(got, "1") || !strings.Contains(got, "2") {
		t.Errorf("expected results, got %q", got)
	}
}

func TestRenderFragment_CurrentUser(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT id FROM users WHERE role = :current_user.role`] = []database.Row{
		{"id": "1"},
	}
	s := newTestServer(mock)
	session := &Session{UserID: "1", Identity: "user@example.com", Role: "viewer", ExpiresAt: time.Now().Add(time.Hour)}
	s.sessions.sessions["session-viewer"] = session
	cookieVal := s.sessions.signSessionID("session-viewer")

	frag := parser.Page{
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT id FROM users WHERE role = :current_user.role`, Name: "users"},
			{Type: parser.NodeHTML, HTMLContent: `<div>{{each users}}{id}{{end}}</div>`},
		},
	}
	req := httptest.NewRequest("GET", "/frag", nil)
	req.Header.Set("Cookie", "_kilnx_session="+cookieVal)
	got := s.renderFragment(frag, req)
	if !strings.Contains(got, "1") {
		t.Errorf("expected result, got %q", got)
	}
}

func TestRenderFragment_PageNumNegative(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT COUNT(*) as _count FROM (SELECT id FROM users)`] = []database.Row{{"_count": "5"}}
	mock.queryRowsWithParamsResults[`SELECT id FROM users LIMIT 10 OFFSET 0`] = []database.Row{
		{"id": "1"},
	}
	s := newTestServer(mock)
	frag := parser.Page{
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT id FROM users`, Name: "users", Paginate: 10},
			{Type: parser.NodeHTML, HTMLContent: `<div>{{each users}}{id}{{end}}</div>`},
		},
	}
	req := httptest.NewRequest("GET", "/frag?page=-5", nil)
	got := s.renderFragment(frag, req)
	if !strings.Contains(got, "1") {
		t.Errorf("expected result, got %q", got)
	}
}

func TestRenderPage_PageNumNegative(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT COUNT(*) as _count FROM (SELECT id FROM users)`] = []database.Row{{"_count": "5"}}
	mock.queryRowsWithParamsResults[`SELECT id FROM users LIMIT 10 OFFSET 0`] = []database.Row{
		{"id": "1"},
	}
	s := newTestServer(mock)
	page := parser.Page{
		Path: "/",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT id FROM users`, Name: "users", Paginate: 10},
			{Type: parser.NodeHTML, HTMLContent: `<div>{{each users}}{id}{{end}}</div>`},
		},
	}
	req := httptest.NewRequest("GET", "/?page=-5", nil)
	got := s.renderPage(page, nil, req)
	if !strings.Contains(got, "1") {
		t.Errorf("expected result, got %q", got)
	}
}

func TestRenderPage_FetchNode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id": 1, "title": "Test Post"}`))
	}))
	defer ts.Close()

	s := newTestServer(nil)
	page := parser.Page{
		Path: "/",
		Body: []parser.Node{
			{Type: parser.NodeFetch, FetchURL: ts.URL, Name: "post"},
			{Type: parser.NodeHTML, HTMLContent: `<div>{post.title}</div>`},
		},
	}
	req := httptest.NewRequest("GET", "/", nil)
	got := s.renderPage(page, nil, req)
	if !strings.Contains(got, "Test Post") {
		t.Errorf("expected fetch result, got %q", got)
	}
}

func TestRenderPage_QueryTenantRejection(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	s.tenants = TenantMap{"users": "org_id"}
	page := parser.Page{
		Path: "/",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT * FROM users`, Name: "users"},
			{Type: parser.NodeHTML, HTMLContent: `<div>hello</div>`},
		},
	}
	req := httptest.NewRequest("GET", "/", nil)
	got := s.renderPage(page, nil, req)
	// Tenant rejection skips the query but continues rendering
	if !strings.Contains(got, "hello") {
		t.Errorf("expected hello in output, got %q", got)
	}
}

func TestRenderPage_QueryError(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsErr[`SELECT bad`] = fmt.Errorf("syntax error")
	s := newTestServer(mock)
	page := parser.Page{
		Path: "/",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT bad`, Name: "users"},
			{Type: parser.NodeHTML, HTMLContent: `<div>hello</div>`},
		},
	}
	req := httptest.NewRequest("GET", "/", nil)
	got := s.renderPage(page, nil, req)
	// Query error is logged but page continues rendering
	if !strings.Contains(got, "hello") {
		t.Errorf("expected hello in output, got %q", got)
	}
}

func TestHandleAction_RedirectHXRequest(t *testing.T) {
	s := newTestServer(nil)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeRedirect, Value: "/success"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200 for htmx redirect", rec.Code)
	}
	hxRedirect := rec.Header().Get("HX-Redirect")
	if hxRedirect != "/success" {
		t.Errorf("hx-redirect = %q, want /success", hxRedirect)
	}
}

func TestHandleAction_RespondQueryError(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsErr[`SELECT name FROM users`] = fmt.Errorf("db error")
	s := newTestServer(mock)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeRespond, QuerySQL: `SELECT name FROM users`, StatusCode: 200},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", rec.Code)
	}
}

func TestHandleAction_RedirectHXRequestFragmentMatch(t *testing.T) {
	s := newTestServer(nil)
	s.app.Fragments = []parser.Page{
		{Path: "/channel/:id/messages", Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<div>messages</div>`},
		}},
	}
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeRedirect, Value: "/channel/6/messages"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "messages") {
		t.Errorf("expected fragment content, got %q", rec.Body.String())
	}
}

func TestHandleAction_RedirectHXRequestPrefixMatch(t *testing.T) {
	s := newTestServer(nil)
	s.app.Fragments = []parser.Page{
		{Path: "/channel/:id/messages/list", Body: []parser.Node{
			{Type: parser.NodeHTML, HTMLContent: `<div>list</div>`},
		}},
	}
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeRedirect, Value: "/channel/6/messages"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "list") {
		t.Errorf("expected fragment content, got %q", rec.Body.String())
	}
}

func TestHandleAction_GeneratePDFDataQueryNotFound(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	s := newTestServer(db)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}}
	req := httptest.NewRequest("POST", "/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	action := parser.Page{
		Path: "/action",
		Body: []parser.Node{
			{Type: parser.NodeQuery, SQL: `SELECT name FROM users`, Name: "users"},
			{Type: parser.NodeGeneratePDF, TemplateName: "Report", DataQueryName: "nonexistent"},
			{Type: parser.NodeRedirect, Value: "/done"},
		},
	}
	s.handleAction(rec, req, action, s.getApp())
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
}

func TestHandleResetPassword_WithSMTP(t *testing.T) {
	t.Setenv("KILNX_SMTP_HOST", "invalid.smtp.host")
	t.Setenv("KILNX_SMTP_FROM", "noreply@example.com")
	t.Setenv("KILNX_SMTP_USER", "user")

	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()
	db.Conn().Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT, password_hash TEXT)")
	db.Conn().Exec("INSERT INTO users (email, password_hash) VALUES ('user@example.com', 'oldhash')")
	db.Conn().Exec("CREATE TABLE _kilnx_password_resets (token TEXT, email TEXT, expires_at TEXT)")
	db.Conn().Exec("INSERT INTO _kilnx_password_resets (token, email, expires_at) VALUES ('abc123', 'user@example.com', datetime('now', '+1 hour'))")

	s := newTestServer(db)
	csrf := generateCSRFToken()
	form := url.Values{"_csrf": {csrf}, "password": {"newpass123"}, "confirm_password": {"newpass123"}}
	req := httptest.NewRequest("POST", "/reset-password?token=abc123", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.handleResetPassword(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("code = %d, want 303", rec.Code)
	}
}
