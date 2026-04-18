package runtime

import (
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

// handleStream serves a Server-Sent Events endpoint.
// It executes the stream's SQL query at the configured interval
// and sends results as SSE events.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request, stream parser.Stream) {
	// Check auth
	if stream.Auth {
		session := s.getSession(r)
		if session == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if stream.RequiresRole != "" && stream.RequiresRole != "auth" {
			app := s.getApp()
			if !s.hasPermission(session.Role, stream.RequiresRole, app.Permissions) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
		}
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Get path params for the query
	params := matchPathParams(stream.Path, r.URL.Path)

	// Add current_user params if authenticated (includes tenant id etc.)
	s.populateCurrentUserParams(params, s.getSession(r))

	ticker := time.NewTicker(time.Duration(stream.IntervalSecs) * time.Second)
	defer ticker.Stop()

	// Send immediately on connect, then on each tick
	s.sendSSEEvent(w, flusher, stream, params)

	for {
		select {
		case <-ticker.C:
			s.sendSSEEvent(w, flusher, stream, params)
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) sendSSEEvent(w http.ResponseWriter, flusher http.Flusher, stream parser.Stream, params map[string]string) {
	if s.db == nil || stream.SQL == "" {
		return
	}

	sql, tErr := RewriteTenantSQL(stream.SQL, s.tenants, params)
	if tErr != nil {
		s.logger.LogError("tenant guard rejected SSE query", tErr)
		fmt.Fprintf(w, "event: error\ndata: query rejected\n\n")
		flusher.Flush()
		return
	}
	rows, err := s.db.QueryRowsWithParams(sql, params)
	if err != nil {
		fmt.Printf("  SSE query error: %v\n", err)
		fmt.Fprintf(w, "event: error\ndata: query failed\n\n")
		flusher.Flush()
		return
	}

	// Build the HTML fragment for htmx SSE
	htmlContent := renderSSERows(rows)

	// Send as SSE event
	eventName := stream.EventName
	if eventName == "" {
		eventName = "message"
	}

	fmt.Fprintf(w, "event: %s\n", eventName)

	// Split data across lines (SSE format requires "data:" prefix per line)
	for _, line := range strings.Split(htmlContent, "\n") {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprintf(w, "\n")

	flusher.Flush()
}

// renderSSERows renders query results as HTML for SSE consumption
func renderSSERows(rows []database.Row) string {
	if len(rows) == 0 {
		return ""
	}

	// If single row with single column, return just the escaped value
	if len(rows) == 1 && len(rows[0]) == 1 {
		for _, v := range rows[0] {
			return html.EscapeString(v)
		}
	}

	var b strings.Builder
	for _, row := range rows {
		b.WriteString("<div class=\"kilnx-sse-item\">")
		for _, val := range row {
			b.WriteString(fmt.Sprintf("<span>%s</span> ", html.EscapeString(val)))
		}
		b.WriteString("</div>")
	}
	return b.String()
}
