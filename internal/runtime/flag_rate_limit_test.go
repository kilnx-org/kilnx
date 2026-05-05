package runtime

import (
	"net/http/httptest"
	"os"
	"testing"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

func TestResolveFlagEnv(t *testing.T) {
	os.Setenv("FLAG_BETA_UI", "true")
	defer os.Unsetenv("FLAG_BETA_UI")

	app := &parser.App{}
	srv := NewServer(app, nil, 0)
	if !srv.resolveFlag("beta_ui") {
		t.Error("expected flag to be true from env")
	}
}

func TestResolveFlagEnvFalse(t *testing.T) {
	os.Setenv("FLAG_BETA_UI", "false")
	defer os.Unsetenv("FLAG_BETA_UI")

	app := &parser.App{}
	srv := NewServer(app, nil, 0)
	if srv.resolveFlag("beta_ui") {
		t.Error("expected flag to be false from env")
	}
}

func TestResolveFlagDB(t *testing.T) {
	tmp := t.TempDir() + "/test.db"
	db, _ := database.Open(tmp)
	defer db.Close()

	// Create flags table manually
	db.Conn().Exec(`CREATE TABLE _kilnx_flags (name TEXT PRIMARY KEY, enabled BOOLEAN NOT NULL DEFAULT 0)`)
	db.Conn().Exec(`INSERT INTO _kilnx_flags (name, enabled) VALUES ('beta_ui', 1)`)

	app := &parser.App{}
	srv := NewServer(app, db, 0)
	if !srv.resolveFlag("beta_ui") {
		t.Error("expected flag to be true from DB")
	}
}

func TestRateLimitIP(t *testing.T) {
	tmp := t.TempDir() + "/test.db"
	db, _ := database.Open(tmp)
	defer db.Close()

	// Create rate limits table manually
	db.Conn().Exec(`CREATE TABLE _kilnx_rate_limits (
		scope_key TEXT NOT NULL,
		limit_period TEXT NOT NULL,
		window_start INTEGER NOT NULL,
		request_count INTEGER NOT NULL DEFAULT 1,
		PRIMARY KEY (scope_key, limit_period, window_start)
	)`)

	app := &parser.App{}
	srv := NewServer(app, db, 0)

	r := httptest.NewRequest("GET", "/", nil)
	// First 5 requests should pass
	for i := 0; i < 5; i++ {
		if !srv.checkRateLimit(5, "minute", "ip", r, nil) {
			t.Fatalf("request %d should pass", i+1)
		}
	}
	// 6th request should fail
	if srv.checkRateLimit(5, "minute", "ip", r, nil) {
		t.Error("6th request should be rate limited")
	}
}

func TestRateLimitUser(t *testing.T) {
	tmp := t.TempDir() + "/test.db"
	db, _ := database.Open(tmp)
	defer db.Close()

	db.Conn().Exec(`CREATE TABLE _kilnx_rate_limits (
		scope_key TEXT NOT NULL,
		limit_period TEXT NOT NULL,
		window_start INTEGER NOT NULL,
		request_count INTEGER NOT NULL DEFAULT 1,
		PRIMARY KEY (scope_key, limit_period, window_start)
	)`)

	app := &parser.App{}
	srv := NewServer(app, db, 0)

	sess := &Session{Identity: "user@example.com"}
	r := httptest.NewRequest("GET", "/", nil)
	if !srv.checkRateLimit(10, "hour", "user", r, sess) {
		t.Error("first request should pass")
	}
}
