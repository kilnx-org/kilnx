package runtime

import (
	"net/http"
	"strings"
	"testing"
)

func TestE2E_RateLimit_UnderLimit(t *testing.T) {
	src := `
page /
  html
    <h1>Home</h1>

limit /
  requests: 5 per minute per ip
`
	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	// 5 requests should all succeed
	for i := 0; i < 5; i++ {
		status, _ := httpGet(t, baseURL+"/")
		if status != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, status)
		}
	}
}

func TestE2E_RateLimit_OverLimit(t *testing.T) {
	src := `
page /limited
  html
    <h1>Limited</h1>

limit /limited
  requests: 3 per minute per ip
  message: "Slow down"
`
	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	// First 3 requests should succeed
	for i := 0; i < 3; i++ {
		status, _ := httpGet(t, baseURL+"/limited")
		if status != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, status)
		}
	}

	// 4th request should be rate limited
	status, body := httpGet(t, baseURL+"/limited")
	if status != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", status)
	}
	if !strings.Contains(body, "Slow down") {
		t.Errorf("expected custom message containing 'Slow down', got %q", body)
	}
}

func TestE2E_RateLimit_WildcardPath(t *testing.T) {
	src := `
page /api/users
  html
    <p>users</p>

page /api/posts
  html
    <p>posts</p>

limit /api/*
  requests: 2 per minute per ip
`
	baseURL, cleanup := startTestServer(t, src)
	defer cleanup()

	// 2 requests to different paths under /api/ should both count
	httpGet(t, baseURL+"/api/users")
	httpGet(t, baseURL+"/api/posts")

	status, _ := httpGet(t, baseURL+"/api/users")
	if status != http.StatusTooManyRequests {
		t.Errorf("expected 429 after exceeding wildcard limit, got %d", status)
	}
}
