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

const maxRateLimitEntries = 100_000

// defaultAuthRateLimits are implicit protections for auth endpoints
// when the developer has not configured explicit rate limits.
var defaultAuthRateLimits = []parser.RateLimit{
	{PathPattern: "/login", Requests: 10, Window: "minute", Per: "ip", Message: "Too many login attempts"},
	{PathPattern: "/register", Requests: 10, Window: "minute", Per: "ip", Message: "Too many registration attempts"},
	{PathPattern: "/forgot-password", Requests: 10, Window: "minute", Per: "ip", Message: "Too many password reset attempts"},
}

func NewRateLimiter(rules []parser.RateLimit) *RateLimiter {
	rl := &RateLimiter{
		entries: make(map[string]*rateLimitEntry),
		rules:   rules,
	}

	// Apply default auth rate limits unless the developer has explicitly
	// configured limits for those paths.
	for _, def := range defaultAuthRateLimits {
		hasCustom := false
		for _, r := range rules {
			if matchRateLimitPath(r.PathPattern, def.PathPattern) {
				hasCustom = true
				break
			}
		}
		if !hasCustom {
			rl.rules = append(rl.rules, def)
		}
	}

	// Adaptive cleanup: interval based on shortest configured window
	interval := rl.minWindow() / 2
	if interval < time.Second {
		interval = time.Second
	}
	if interval > 60*time.Second {
		interval = 60 * time.Second
	}
	go func() {
		for {
			time.Sleep(interval)
			rl.cleanup()
		}
	}()
	return rl
}

// minWindow returns the shortest window duration across all rules
func (rl *RateLimiter) minWindow() time.Duration {
	min := time.Hour
	for _, rule := range rl.rules {
		w := windowDuration(rule.Window)
		if w < min {
			min = w
		}
	}
	return min
}

// CheckWithRule returns (exceeded bool, matched rule) for the request.
// exceeded is true when the request should be blocked.
func (rl *RateLimiter) CheckWithRule(r *http.Request, session *Session) (bool, *parser.RateLimit) {
	if len(rl.rules) == 0 {
		return false, nil
	}

	for _, rule := range rl.rules {
		if !matchRateLimitPath(rule.PathPattern, r.URL.Path) {
			continue
		}

		var key string
		switch rule.Per {
		case "user":
			if session != nil {
				key = rule.PathPattern + ":user:" + session.UserID
			} else {
				key = rule.PathPattern + ":ip:" + clientIP(r)
			}
		default:
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
			return false, nil
		}

		entry.count++
		if entry.count > rule.Requests {
			rl.mu.Unlock()
			r := rule // copy to avoid loop variable issue
			return true, &r
		}

		rl.mu.Unlock()
		return false, nil
	}

	return false, nil
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
	// Safety valve: prevent unbounded memory growth under DDoS
	if len(rl.entries) > maxRateLimitEntries {
		count := 0
		evictTarget := len(rl.entries) / 10
		for key := range rl.entries {
			if count >= evictTarget {
				break
			}
			delete(rl.entries, key)
			count++
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
