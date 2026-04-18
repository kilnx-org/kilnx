package runtime

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// Logger wraps request handling with logging
type Logger struct {
	config *parser.LogConfig
}

func NewLogger(config *parser.LogConfig) *Logger {
	if config == nil {
		config = &parser.LogConfig{
			Level:       "info",
			SlowQueryMs: 100,
			LogRequests: false,
			LogErrors:   true,
		}
	}
	return &Logger{config: config}
}

// LogRequest logs an HTTP request if request logging is enabled
func (l *Logger) LogRequest(r *http.Request, status int, duration time.Duration) {
	if !l.config.LogRequests {
		return
	}
	fmt.Printf("[%s] %s %s %d %s\n",
		time.Now().Format("15:04:05"),
		r.Method, r.URL.Path, status, duration.Round(time.Millisecond))
}

// LogSlowQuery logs a query if it exceeds the slow query threshold
func (l *Logger) LogSlowQuery(sql string, duration time.Duration) {
	if l.config.SlowQueryMs <= 0 {
		return
	}
	if duration.Milliseconds() > int64(l.config.SlowQueryMs) {
		fmt.Printf("[%s] SLOW QUERY (%s): %s\n",
			time.Now().Format("15:04:05"),
			duration.Round(time.Millisecond),
			truncateSQL(sql))
	}
}

// LogSecurity always emits a security-relevant event regardless of the
// user's `log.errors` config. Routed to stderr so operators never lose
// the signal that a tenant guard, CSRF check, or similar invariant
// fired. Nil-tolerant for tests.
func (l *Logger) LogSecurity(msg string, err error) {
	line := fmt.Sprintf("[%s] SECURITY: %s: %v\n",
		time.Now().Format("15:04:05"), msg, err)
	_, _ = os.Stderr.WriteString(line)
}

// LogError logs an error, optionally with a goroutine stack trace.
// Nil-tolerant: tests and early-boot paths may hold a nil Logger.
func (l *Logger) LogError(msg string, err error) {
	if l == nil || !l.config.LogErrors {
		return
	}
	fmt.Printf("[%s] ERROR: %s: %v\n",
		time.Now().Format("15:04:05"), msg, err)

	if l.config.Stacktrace {
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		fmt.Printf("[%s] STACKTRACE:\n%s\n",
			time.Now().Format("15:04:05"), string(buf[:n]))
	}
}

func truncateSQL(sql string) string {
	if len(sql) > 200 {
		return sql[:200] + "..."
	}
	return sql
}

// LoggingMiddleware wraps an http.Handler with request logging
func (l *Logger) LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w, statusCode: 200}
		next.ServeHTTP(lw, r)
		l.LogRequest(r, lw.statusCode, time.Since(start))
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lw *loggingResponseWriter) WriteHeader(code int) {
	lw.statusCode = code
	lw.ResponseWriter.WriteHeader(code)
}

// Hijack proxies to the underlying ResponseWriter for WebSocket support.
func (lw *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := lw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not support hijacking")
}

// Flush proxies to the underlying ResponseWriter for SSE support.
func (lw *loggingResponseWriter) Flush() {
	if f, ok := lw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
