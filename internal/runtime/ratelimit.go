package runtime

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kilnx-org/kilnx/internal/parser"
)

type rateLimitEntry struct {
	count     int
	expiresAt time.Time
}

type RateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rateLimitEntry
	rules   []parser.RateLimit
}

func NewRateLimiter(rules []parser.RateLimit) *RateLimiter {
	rl := &RateLimiter{
		entries: make(map[string]*rateLimitEntry),
		rules:   rules,
	}
	// Cleanup expired entries periodically
	go func() {
		for {
			time.Sleep(60 * time.Second)
			rl.cleanup()
		}
	}()
	return rl
}

// Check returns true if the request is allowed, false if rate limited
func (rl *RateLimiter) Check(r *http.Request, session *Session) bool {
	if len(rl.rules) == 0 {
		return true
	}

	for _, rule := range rl.rules {
		if !matchRateLimitPath(rule.PathPattern, r.URL.Path) {
			continue
		}

		// Build key based on "per" setting
		var key string
		switch rule.Per {
		case "user":
			if session != nil {
				key = rule.PathPattern + ":user:" + session.UserID
			} else {
				key = rule.PathPattern + ":ip:" + clientIP(r)
			}
		default: // "ip"
			key = rule.PathPattern + ":ip:" + clientIP(r)
		}

		window := windowDuration(rule.Window)

		rl.mu.Lock()
		entry, exists := rl.entries[key]
		if !exists || time.Now().After(entry.expiresAt) {
			rl.entries[key] = &rateLimitEntry{
				count:     1,
				expiresAt: time.Now().Add(window),
			}
			rl.mu.Unlock()
			return true
		}

		entry.count++
		if entry.count > rule.Requests {
			rl.mu.Unlock()
			return false
		}

		rl.mu.Unlock()
		return true
	}

	return true
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	for key, entry := range rl.entries {
		if now.After(entry.expiresAt) {
			delete(rl.entries, key)
		}
	}
}

func matchRateLimitPath(pattern, path string) bool {
	// Simple wildcard: /api/* matches /api/v1/users
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(path, prefix+"/") || path == prefix
	}
	return pattern == path
}

func windowDuration(window string) time.Duration {
	switch window {
	case "second":
		return time.Second
	case "minute":
		return time.Minute
	case "hour":
		return time.Hour
	default:
		return time.Minute
	}
}

func clientIP(r *http.Request) string {
	// Use RemoteAddr as the primary source.
	// Only trust X-Forwarded-For when the direct connection is from localhost
	// (i.e., behind a local reverse proxy).
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx >= 0 {
		addr = addr[:idx]
	}

	isLocal := addr == "127.0.0.1" || addr == "::1" || addr == "localhost"
	if isLocal {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.SplitN(xff, ",", 2)
			return strings.TrimSpace(parts[0])
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return xri
		}
	}

	return addr
}
