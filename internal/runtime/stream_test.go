package runtime

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

// flushRecorder is an httptest.ResponseRecorder that implements http.Flusher
type flushRecorder struct {
	*httptest.ResponseRecorder
}

func (f *flushRecorder) Flush() {}

func newFlushRecorder() *flushRecorder {
	return &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
}

func TestSendSSEEvent_NoDB(t *testing.T) {
	s := newTestServer(nil)
	rec := newFlushRecorder()
	stream := parser.Stream{SQL: `SELECT name FROM users`}
	s.sendSSEEvent(rec, rec, stream, nil)
	if rec.Body.Len() != 0 {
		t.Errorf("expected empty body when db is nil, got %q", rec.Body.String())
	}
}

func TestSendSSEEvent_EmptySQL(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	rec := newFlushRecorder()
	stream := parser.Stream{SQL: ""}
	s.sendSSEEvent(rec, rec, stream, nil)
	if rec.Body.Len() != 0 {
		t.Errorf("expected empty body when SQL is empty, got %q", rec.Body.String())
	}
}

func TestSendSSEEvent_Success(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT name FROM users`] = []database.Row{
		{"name": "Alice"},
		{"name": "Bob"},
	}
	s := newTestServer(mock)
	rec := newFlushRecorder()
	stream := parser.Stream{SQL: `SELECT name FROM users`, EventName: "update"}
	s.sendSSEEvent(rec, rec, stream, nil)

	body := rec.Body.String()
	if !strings.Contains(body, "event: update") {
		t.Errorf("expected event name, got %q", body)
	}
	if !strings.Contains(body, "Alice") {
		t.Errorf("expected Alice in data, got %q", body)
	}
}

func TestSendSSEEvent_DefaultEventName(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsResults[`SELECT 1`] = []database.Row{{"1": "1"}}
	s := newTestServer(mock)
	rec := newFlushRecorder()
	stream := parser.Stream{SQL: `SELECT 1`}
	s.sendSSEEvent(rec, rec, stream, nil)

	body := rec.Body.String()
	if !strings.Contains(body, "event: message") {
		t.Errorf("expected default event name 'message', got %q", body)
	}
}

func TestSendSSEEvent_QueryError(t *testing.T) {
	mock := newMockExecutor()
	mock.queryRowsWithParamsErr[`SELECT name FROM users`] = fmt.Errorf("db error")
	s := newTestServer(mock)
	rec := newFlushRecorder()
	stream := parser.Stream{SQL: `SELECT name FROM users`}
	s.sendSSEEvent(rec, rec, stream, nil)

	body := rec.Body.String()
	if !strings.Contains(body, "event: error") {
		t.Errorf("expected error event, got %q", body)
	}
}

func TestSendSSEEvent_TenantRejection(t *testing.T) {
	mock := newMockExecutor()
	s := newTestServer(mock)
	s.tenants = TenantMap{"users": "tenant_id"}
	rec := newFlushRecorder()
	stream := parser.Stream{SQL: `SELECT * FROM users`}
	s.sendSSEEvent(rec, rec, stream, nil)

	body := rec.Body.String()
	if !strings.Contains(body, "event: error") {
		t.Errorf("expected error event for tenant rejection, got %q", body)
	}
}


func TestHandleStream_AuthRequired(t *testing.T) {
	s := newTestServer(nil)
	stream := parser.Stream{Path: "/stream", Auth: true}
	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	rec := httptest.NewRecorder()

	s.handleStream(rec, req, stream)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("code = %d, want 401", rec.Code)
	}
}

func TestHandleStream_RequiresRole(t *testing.T) {
	s := newTestServer(nil)
	s.app.Permissions = []parser.Permission{
		{Role: "admin", Rules: []string{"all"}},
	}
	session := &Session{UserID: "1", Identity: "user@example.com", Role: "viewer", ExpiresAt: time.Now().Add(time.Hour)}
	s.sessions.sessions["session-viewer"] = session
	cookieVal := s.sessions.signSessionID("session-viewer")

	stream := parser.Stream{Path: "/stream", Auth: true, RequiresRole: "admin"}
	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	req.Header.Set("Cookie", "_kilnx_session="+cookieVal)
	rec := httptest.NewRecorder()

	s.handleStream(rec, req, stream)
	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", rec.Code)
	}
}

func TestHandleStream_RequiresClauses(t *testing.T) {
	s := newTestServer(nil)
	session := &Session{UserID: "1", Identity: "user@example.com", Role: "viewer", Data: database.Row{"plan": "basic"}, ExpiresAt: time.Now().Add(time.Hour)}
	s.sessions.sessions["session-viewer"] = session
	cookieVal := s.sessions.signSessionID("session-viewer")

	stream := parser.Stream{
		Path: "/stream",
		Auth: true,
		RequiresClauses: []parser.RequiresClause{
			{Kind: parser.RequiresClauseExpr, Value: "current_user.plan == 'premium'"},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	req.Header.Set("Cookie", "_kilnx_session="+cookieVal)
	rec := httptest.NewRecorder()

	s.handleStream(rec, req, stream)
	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", rec.Code)
	}
}

func TestHandleStream_Success(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Conn().Exec("CREATE TABLE events (id INTEGER PRIMARY KEY, msg TEXT)"); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	if _, err := db.Conn().Exec("INSERT INTO events (msg) VALUES ('hello')"); err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	s := newTestServer(db)
	stream := parser.Stream{Path: "/stream", SQL: `SELECT msg FROM events`, IntervalSecs: 1}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/stream", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	s.handleStream(rec, req, stream)

	body := rec.Body.String()
	if !strings.Contains(body, "hello") {
		t.Errorf("expected 'hello' in SSE output, got %q", body)
	}
}

func TestHandleStream_NoFlusher(t *testing.T) {
	s := newTestServer(nil)
	stream := parser.Stream{Path: "/stream"}
	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	rec := &mockResponseWriter{}

	s.handleStream(rec, req, stream)
	if rec.statusCode != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", rec.statusCode)
	}
}

// mockResponseWriter that does not implement http.Flusher
type mockResponseWriter struct {
	header     http.Header
	statusCode int
	body       []byte
}

func (m *mockResponseWriter) Header() http.Header {
	if m.header == nil {
		m.header = make(http.Header)
	}
	return m.header
}

func (m *mockResponseWriter) Write(b []byte) (int, error) {
	m.body = append(m.body, b...)
	return len(b), nil
}

func (m *mockResponseWriter) WriteHeader(code int) {
	m.statusCode = code
}
