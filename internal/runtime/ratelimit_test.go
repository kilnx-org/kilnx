package runtime

import (
	"net/http/httptest"
	"testing"

	"github.com/kilnx-org/kilnx/internal/parser"
)

func TestCheckWithRule_UserFallbackToIP(t *testing.T) {
	rl := NewRateLimiter([]parser.RateLimit{
		{PathPattern: "/api", Requests: 5, Window: "minute", Per: "user"},
	})
	req := httptest.NewRequest("GET", "/api", nil)
	exceeded, _ := rl.CheckWithRule(req, nil)
	if exceeded {
		t.Error("first request should not be exceeded")
	}
}

func TestCheckWithRule_NoRules(t *testing.T) {
	rl := NewRateLimiter([]parser.RateLimit{})
	req := httptest.NewRequest("GET", "/api", nil)
	exceeded, matched := rl.CheckWithRule(req, nil)
	if exceeded {
		t.Error("should not be exceeded with no rules")
	}
	if matched != nil {
		t.Error("matched should be nil with no rules")
	}
}

func TestCheck_PathNoMatch(t *testing.T) {
	rl := NewRateLimiter([]parser.RateLimit{
		{PathPattern: "/api", Requests: 5, Window: "minute", Per: "ip"},
	})
	req := httptest.NewRequest("GET", "/other", nil)
	if !rl.Check(req, nil) {
		t.Error("should allow request to non-matching path")
	}
}

func TestCheck_AllowedAfterMatch(t *testing.T) {
	rl := NewRateLimiter([]parser.RateLimit{
		{PathPattern: "/api", Requests: 5, Window: "minute", Per: "ip"},
	})
	req := httptest.NewRequest("GET", "/api", nil)
	if !rl.Check(req, nil) {
		t.Error("first request should be allowed")
	}
}

func TestCheckWithRule_UserWithSession(t *testing.T) {
	rl := NewRateLimiter([]parser.RateLimit{
		{PathPattern: "/api", Requests: 1, Window: "minute", Per: "user"},
	})
	req := httptest.NewRequest("GET", "/api", nil)
	session := &Session{UserID: "42"}
	exceeded, _ := rl.CheckWithRule(req, session)
	if exceeded {
		t.Error("first request should not be exceeded")
	}
	exceeded, _ = rl.CheckWithRule(req, session)
	if !exceeded {
		t.Error("second request should be exceeded")
	}
}

func TestCheck_UserFallbackToIP(t *testing.T) {
	rl := NewRateLimiter([]parser.RateLimit{
		{PathPattern: "/api", Requests: 1, Window: "minute", Per: "user"},
	})
	req := httptest.NewRequest("GET", "/api", nil)
	if !rl.Check(req, nil) {
		t.Error("first request should be allowed")
	}
	if rl.Check(req, nil) {
		t.Error("second request should be blocked")
	}
}

func TestCheck_NoRules(t *testing.T) {
	rl := NewRateLimiter([]parser.RateLimit{})
	req := httptest.NewRequest("GET", "/api", nil)
	if !rl.Check(req, nil) {
		t.Error("should allow request with no rules")
	}
}

func TestNewRateLimiter_ShortWindow(t *testing.T) {
	// window "second" -> minWindow = 1s -> interval = 500ms -> clamped to 1s
	rl := NewRateLimiter([]parser.RateLimit{
		{PathPattern: "/api", Requests: 5, Window: "second", Per: "ip"},
	})
	if rl == nil {
		t.Error("expected rate limiter")
	}
}

func TestNewRateLimiter_LongWindow(t *testing.T) {
	// window "hour" -> minWindow = 1h -> interval = 30m -> clamped to 60s
	rl := NewRateLimiter([]parser.RateLimit{
		{PathPattern: "/api", Requests: 5, Window: "hour", Per: "ip"},
	})
	if rl == nil {
		t.Error("expected rate limiter")
	}
}
