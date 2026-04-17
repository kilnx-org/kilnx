package runtime

import (
	"embed"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
	"github.com/kilnx-org/kilnx/internal/pdf"
)

//go:embed static/htmx.min.js static/sse.js
var staticFS embed.FS

var interpolateRe = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*(?:\.[a-zA-Z_][a-zA-Z0-9_]*)*)\}`)

type Server struct {
	app          *parser.App
	db           *database.DB
	sessions     *SessionStore
	jobQueue     *JobQueue
	rateLimiter  *RateLimiter
	logger       *Logger
	i18n         *I18n
	mu           sync.RWMutex
	port         int
	scheduleStop chan struct{}
}

func NewServer(app *parser.App, db *database.DB, port int) *Server {
	secret := ""
	if app.Config != nil {
		secret = app.Config.Secret
	}
	s := &Server{app: app, db: db, sessions: NewSessionStore(secret), port: port}
	// Attach DB to session store for persistence
	if db != nil {
		s.sessions.SetDB(db)
	}
	s.jobQueue = NewJobQueue(s)
	s.rateLimiter = NewRateLimiter(app.RateLimits)
	s.logger = NewLogger(app.LogConfig)
	// Wire slow query logging from database to logger
	if db != nil {
		db.OnSlowQuery = s.logger.LogSlowQuery
	}
	defaultLang := "en"
	detectLang := true // detect by default when translations exist
	if app.Config != nil {
		if app.Config.DefaultLanguage != "" {
			defaultLang = app.Config.DefaultLanguage
		}
		if app.Config.DetectLanguage != "" {
			detectLang = app.Config.DetectLanguage != "false" && app.Config.DetectLanguage != "off"
		}
	}
	s.i18n = NewI18n(app.Translations, defaultLang, detectLang)
	return s
}

func (s *Server) Reload(app *parser.App) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.app = app
}

func (s *Server) StartJobQueue() {
	s.jobQueue.Start()
}

func (s *Server) getApp() *parser.App {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.app
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Serve embedded static files
	mux.HandleFunc("/_kilnx/htmx.min.js", func(w http.ResponseWriter, r *http.Request) {
		data, _ := staticFS.ReadFile("static/htmx.min.js")
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.Write(data)
	})
	mux.HandleFunc("/_kilnx/sse.js", func(w http.ResponseWriter, r *http.Request) {
		data, _ := staticFS.ReadFile("static/sse.js")
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.Write(data)
	})

	// Serve uploaded files
	uploadsDir := "uploads"
	if s.app.Config != nil && s.app.Config.UploadsDir != "" {
		uploadsDir = s.app.Config.UploadsDir
	}
	mux.Handle("/_uploads/", http.StripPrefix("/_uploads/", http.FileServer(http.Dir(uploadsDir))))

	// Serve static files directory (validated to prevent path traversal)
	staticDir := "static"
	if s.app.Config != nil && s.app.Config.StaticDir != "" {
		staticDir = s.app.Config.StaticDir
	}
	if absStatic, err := filepath.Abs(staticDir); err == nil {
		cwd, _ := os.Getwd()
		if strings.HasPrefix(absStatic, cwd) {
			if info, err := os.Stat(absStatic); err == nil && info.IsDir() {
				fileServer := http.FileServer(http.Dir(absStatic))
				mux.Handle("/_static/", http.StripPrefix("/_static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Cache-Control", "public, max-age=3600")
					fileServer.ServeHTTP(w, r)
				})))
				fmt.Printf("Serving static files from %s at /_static/\n", staticDir)
			}
		}
	}

	// Health check for PaaS platforms and load balancers (GET only)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Catch-all handler that resolves routes dynamically (supports hot reload)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		app := s.getApp()

		// Rate limiting
		if exceeded, rule := s.rateLimiter.CheckWithRule(r, s.getSession(r)); exceeded {
			msg := "Too many requests"
			retryAfter := "60"
			if rule != nil {
				if rule.Message != "" {
					msg = rule.Message
				}
				if rule.DelaySecs > 0 {
					retryAfter = fmt.Sprintf("%d", rule.DelaySecs)
				}
			}
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set("Retry-After", retryAfter)
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(msg))
			return
		}

		// Webhooks (POST, no CSRF)
		for _, wh := range app.Webhooks {
			if r.URL.Path == wh.Path {
				s.handleWebhook(w, r, wh)
				return
			}
		}

		// WebSockets
		for _, sock := range app.Sockets {
			if matchPath(sock.Path, r.URL.Path) {
				s.handleSocket(w, r, sock)
				return
			}
		}

		// Auth routes (auto-generated when auth block is present)
		if app.Auth != nil {
			if r.URL.Path == app.Auth.LoginPath {
				s.handleLogin(w, r)
				return
			}
			if r.URL.Path == "/logout" {
				s.handleLogout(w, r)
				return
			}
			if r.URL.Path == "/register" {
				s.handleRegister(w, r)
				return
			}
			if r.URL.Path == "/forgot-password" {
				s.handleForgotPassword(w, r)
				return
			}
			if r.URL.Path == "/reset-password" {
				s.handleResetPassword(w, r)
				return
			}
		}

		// Handle POST/PUT/DELETE -> match actions
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
			for _, action := range app.Actions {
				if matchPath(action.Path, r.URL.Path) {
					// Enforce declared HTTP method
					if action.Method != "" && !strings.EqualFold(action.Method, r.Method) {
						continue
					}
					if !s.requireAuth(w, r, action) {
						return
					}
					s.handleAction(w, r, action, app)
					return
				}
			}
			// Also check pages with method POST (legacy)
			for _, page := range app.Pages {
				if page.Method == "POST" && matchPath(page.Path, r.URL.Path) {
					if !s.requireAuth(w, r, page) {
						return
					}
					s.handleAction(w, r, page, app)
					return
				}
			}
		}

		// Match SSE streams
		for _, stream := range app.Streams {
			if matchPath(stream.Path, r.URL.Path) {
				s.handleStream(w, r, stream)
				return
			}
		}

		// Match API endpoints (JSON)
		pathMatched := false
		for _, api := range app.APIs {
			if matchPath(api.Path, r.URL.Path) {
				// Allow CORS preflight even when method is declared
				if r.Method == http.MethodOptions {
					s.handleAPI(w, r, api)
					return
				}
				// Enforce declared HTTP method
				if api.Method != "" && !strings.EqualFold(api.Method, r.Method) {
					pathMatched = true
					continue
				}
				if !s.requireAPIAuth(w, r, api) {
					return
				}
				s.handleAPI(w, r, api)
				return
			}
		}

		// If a path matched but no method matched, return 405
		if pathMatched {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		// Match fragments (partial HTML, no page wrapper)
		for _, frag := range app.Fragments {
			if matchPath(frag.Path, r.URL.Path) {
				if !s.requireAuth(w, r, frag) {
					return
				}
				content := s.renderFragment(frag, r)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write([]byte(content))
				return
			}
		}

		// Find matching page (GET)
		for _, page := range app.Pages {
			if matchPath(page.Path, r.URL.Path) {
				if !s.requireAuth(w, r, page) {
					return
				}
				content := s.renderPage(page, app.Pages, r)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write([]byte(content))
				return
			}
		}

		// 404
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(render404(r.URL.Path, app.Pages)))
	})

	addr := fmt.Sprintf(":%d", s.port)
	fmt.Printf("kilnx serving on http://localhost%s\n", addr)
	return http.ListenAndServe(addr, s.logger.LoggingMiddleware(mux))
}

// PaginateInfo holds pagination state for a query
type PaginateInfo struct {
	Page    int
	PerPage int
	Total   int
	HasPrev bool
	HasNext bool
}

// renderContext holds query results available during template rendering
type renderContext struct {
	queries     map[string][]database.Row
	paginate    map[string]PaginateInfo
	currentUser *Session
	queryParams map[string]string // URL query parameters (?key=value)
}

func (s *Server) renderPage(p parser.Page, allPages []parser.Page, r *http.Request) string {
	ctx := &renderContext{
		queries:     make(map[string][]database.Row),
		paginate:    make(map[string]PaginateInfo),
		currentUser: s.getSession(r),
		queryParams: make(map[string]string),
	}

	pathParams := matchPathParams(p.Path, r.URL.Path)

	// Make current_user available as a query result and as SQL params
	if ctx.currentUser != nil {
		ctx.queries["current_user"] = []database.Row{ctx.currentUser.Data}
		pathParams["current_user.id"] = ctx.currentUser.UserID
		pathParams["current_user.identity"] = ctx.currentUser.Identity
		pathParams["current_user.role"] = ctx.currentUser.Role
		pathParams["current_user_id"] = ctx.currentUser.UserID
		pathParams["current_user_identity"] = ctx.currentUser.Identity
		pathParams["current_user_role"] = ctx.currentUser.Role
	}

	// Expose URL query parameters to templates and SQL
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			ctx.queryParams[key] = values[0]
			pathParams[key] = values[0]
		}
	}

	// Get current page number from query string
	pageNum := 1
	if pg := r.URL.Query().Get("page"); pg != "" {
		fmt.Sscanf(pg, "%d", &pageNum)
		if pageNum < 1 {
			pageNum = 1
		}
	}

	// Execute all queries first (needed for both modes)
	for _, node := range p.Body {
		if node.Type != parser.NodeQuery || s.db == nil {
			continue
		}
		sql := node.SQL

		queryName := node.Name
		if queryName == "" {
			queryName = "_last"
		}
		// Handle pagination
		if node.Paginate > 0 {
			countSQL := fmt.Sprintf("SELECT COUNT(*) as _count FROM (%s)", sql)
			params := make(map[string]string)
			for k, v := range pathParams {
				params[k] = v
			}
			var countRows []database.Row
			var countErr error
			if len(params) > 0 {
				countRows, countErr = s.db.QueryRowsWithParams(countSQL, params)
			} else {
				countRows, countErr = s.db.QueryRows(countSQL)
			}
			total := 0
			if countErr == nil && len(countRows) > 0 {
				fmt.Sscanf(countRows[0]["_count"], "%d", &total)
			}

			offset := (pageNum - 1) * node.Paginate
			sql = fmt.Sprintf("%s LIMIT %d OFFSET %d", sql, node.Paginate, offset)

			name := node.Name
			if name == "" {
				name = "_last"
			}
			ctx.paginate[name] = PaginateInfo{
				Page:    pageNum,
				PerPage: node.Paginate,
				Total:   total,
				HasPrev: pageNum > 1,
				HasNext: offset+node.Paginate < total,
			}
		}

		params := make(map[string]string)
		for k, v := range pathParams {
			params[k] = v
		}
		var rows []database.Row
		var err error
		if len(params) > 0 {
			rows, err = s.db.QueryRowsWithParams(sql, params)
		} else {
			rows, err = s.db.QueryRows(sql)
		}
		if err != nil {
			continue
		}
		ctx.queries[queryName] = rows
	}

	// Execute fetch nodes
	for _, node := range p.Body {
		if node.Type != parser.NodeFetch {
			continue
		}
		fetchName := node.Name
		if fetchName == "" {
			fetchName = "_fetch"
		}
		rows, err := executeFetch(node, pathParams)
		if err != nil {
			fmt.Printf("  fetch error: %v\n", err)
			continue
		}
		ctx.queries[fetchName] = rows
	}

	var body strings.Builder

	for _, node := range p.Body {
		switch node.Type {
		case parser.NodeQuery:
			// Already executed above
		case parser.NodeHTML:
			htmlContent := s.i18n.TranslateAll(node.HTMLContent, r)
			htmlContent = renderHTML(htmlContent, ctx)
			body.WriteString(htmlContent + "\n")
		case parser.NodeText:
			text := s.i18n.TranslateAll(node.Value, r)
			text = interpolate(text, ctx)
			body.WriteString(fmt.Sprintf("<p>%s</p>\n", html.EscapeString(text)))
		}
	}

	title := p.Title
	if title == "" {
		title = "kilnx"
	}
	title = interpolate(title, ctx)

	bodyContent := body.String()

	appName := ""
	app := s.getApp()
	if app.Config != nil {
		appName = app.Config.Name
	}
	nav := renderNav(allPages, p.Path, ctx.currentUser, appName)
	content := bodyContent
	if p.Layout != "" {
		for _, layout := range app.Layouts {
			if layout.Name == p.Layout {
				var layoutCtx *renderContext
				if len(layout.Queries) > 0 && s.db != nil {
					layoutCtx = &renderContext{
						queries:  make(map[string][]database.Row),
						paginate: make(map[string]PaginateInfo),
					}
					// Build params from path and current_user (same as page queries)
					layoutParams := make(map[string]string)
					for k, v := range pathParams {
						layoutParams[k] = v
					}
					if ctx.currentUser != nil {
						layoutParams["current_user.id"] = ctx.currentUser.UserID
						layoutParams["current_user.identity"] = ctx.currentUser.Identity
						layoutParams["current_user.role"] = ctx.currentUser.Role
						layoutParams["current_user_id"] = ctx.currentUser.UserID
						layoutParams["current_user_identity"] = ctx.currentUser.Identity
						layoutParams["current_user_role"] = ctx.currentUser.Role
					}
					for _, q := range layout.Queries {
						if q.SQL == "" {
							continue
						}
						name := q.Name
						if name == "" {
							name = "_last"
						}
						var rows []database.Row
						var err error
						if len(layoutParams) > 0 {
							rows, err = s.db.QueryRowsWithParams(q.SQL, layoutParams)
						} else {
							rows, err = s.db.QueryRows(q.SQL)
						}
						if err == nil {
							layoutCtx.queries[name] = rows
						}
					}
				}
				// Propagate current_user to layout context
				if layoutCtx == nil {
					layoutCtx = &renderContext{
						queries:  make(map[string][]database.Row),
						paginate: make(map[string]PaginateInfo),
					}
				}
				layoutCtx.currentUser = ctx.currentUser
				if ctx.currentUser != nil {
					layoutCtx.queries["current_user"] = ctx.queries["current_user"]
				}
				return renderWithLayout(layout, title, nav, content, layoutCtx)
			}
		}
	}

	// Default layout
	return renderDefaultLayout(title, nav, content)
}

// interpolate replaces {name.field} patterns with query result values
// Supports:
//   - {queryName.field} -> first row of named query, specific column
//   - {queryName.count} -> number of rows in named query (built-in)
func interpolate(text string, ctx *renderContext) string {
	return interpolateRe.ReplaceAllStringFunc(text, func(match string) string {
		// Strip braces
		expr := match[1 : len(match)-1]

		parts := strings.SplitN(expr, ".", 2)

		if len(parts) == 2 {
			queryName := parts[0]
			field := parts[1]

			rows, ok := ctx.queries[queryName]
			if !ok {
				return match // leave unchanged if query not found
			}

			// Built-in: .count returns number of rows
			if field == "count" {
				return fmt.Sprintf("%d", len(rows))
			}

			// Return first row's field value
			if len(rows) > 0 {
				if val, ok := rows[0][field]; ok {
					return val
				}
			}
			return ""
		}

		// Single name: check all queries for a matching column in first row
		for _, rows := range ctx.queries {
			if len(rows) > 0 {
				if val, ok := rows[0][expr]; ok {
					return val
				}
			}
		}

		return match
	})
}

func renderNav(pages []parser.Page, currentPath string, session *Session, appName string) string {
	var nav strings.Builder
	nav.WriteString("  <header class=\"kilnx-topbar\">\n")
	nav.WriteString("    <div class=\"kilnx-topbar-left\">\n")
	if appName != "" {
		nav.WriteString(fmt.Sprintf("      <span class=\"kilnx-app-name\">%s</span>\n", html.EscapeString(appName)))
	}
	nav.WriteString("      <nav class=\"kilnx-nav\">\n")
	for _, p := range pages {
		// Skip pages with :params in the path (they're not navigable directly)
		if strings.Contains(p.Path, ":") {
			continue
		}
		// Skip auth-required pages if not logged in
		if p.Auth && session == nil {
			continue
		}
		class := ""
		if p.Path == currentPath {
			class = ` class="active"`
		}
		label := p.Title
		if label == "" {
			if p.Path == "/" {
				label = "Home"
			} else {
				label = strings.TrimPrefix(p.Path, "/")
				label = strings.ToUpper(label[:1]) + label[1:]
			}
		}
		nav.WriteString(fmt.Sprintf("        <a href=\"%s\"%s>%s</a>\n", p.Path, class, html.EscapeString(label)))
	}
	nav.WriteString("      </nav>\n")
	nav.WriteString("    </div>\n")
	// Auth links
	if session != nil {
		nav.WriteString("    <div class=\"kilnx-topbar-right\">\n")
		nav.WriteString(fmt.Sprintf("      <span class=\"kilnx-user\">%s</span>\n",
			html.EscapeString(session.Identity)))
		csrf := generateCSRFToken()
		nav.WriteString(fmt.Sprintf("      <form method=\"POST\" action=\"/logout\" style=\"display:inline;margin:0\"><input type=\"hidden\" name=\"_csrf\" value=\"%s\"><button type=\"submit\" class=\"kilnx-logout\">Logout</button></form>\n", csrf))
		nav.WriteString("    </div>\n")
	}
	nav.WriteString("  </header>\n")
	return nav.String()
}

func render404(path string, pages []parser.Page) string {
	nav := renderNav(pages, "", nil, "")
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>404 - Not Found</title>
  <script src="/_kilnx/htmx.min.js"></script>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { font-family: system-ui, -apple-system, sans-serif; line-height: 1.6; color: #1a1a1a; max-width: 800px; margin: 0 auto; padding: 1rem; }
    nav { display: flex; gap: 1rem; padding: 0.75rem 0; border-bottom: 1px solid #e0e0e0; margin-bottom: 1.5rem; }
    nav a { text-decoration: none; color: #555; font-size: 0.9rem; }
    nav a:hover { color: #1a1a1a; }
    main { padding: 2rem 0; text-align: center; }
    h1 { font-size: 3rem; color: #ccc; margin-bottom: 0.5rem; }
    p { color: #888; }
    code { background: #f5f5f5; padding: 0.15rem 0.4rem; border-radius: 3px; font-size: 0.9rem; }
  </style>
</head>
<body>
%s  <main>
    <h1>404</h1>
    <p>No page declared for <code>%s</code></p>
  </main>
</body>
</html>
`, nav, html.EscapeString(path))
}

// renderFragment renders a fragment (partial HTML, no page wrapper)
func (s *Server) renderFragment(frag parser.Page, r *http.Request) string {
	ctx := &renderContext{
		queries:     make(map[string][]database.Row),
		paginate:    make(map[string]PaginateInfo),
		currentUser: s.getSession(r),
		queryParams: make(map[string]string),
	}

	// Make current_user available in fragments
	if ctx.currentUser != nil {
		ctx.queries["current_user"] = []database.Row{ctx.currentUser.Data}
	}

	// Get path params and merge query params
	pathParams := matchPathParams(frag.Path, r.URL.Path)
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			ctx.queryParams[key] = values[0]
			pathParams[key] = values[0]
		}
	}

	var body strings.Builder

	// Get current page number for pagination
	pageNum := 1
	if pg := r.URL.Query().Get("page"); pg != "" {
		fmt.Sscanf(pg, "%d", &pageNum)
		if pageNum < 1 {
			pageNum = 1
		}
	}

	for _, node := range frag.Body {
		switch node.Type {
		case parser.NodeQuery:
			if s.db != nil {
				sql := node.SQL
				queryName := node.Name
				if queryName == "" {
					queryName = "_last"
				}
				// Handle pagination in fragments
				if node.Paginate > 0 {
					countSQL := fmt.Sprintf("SELECT COUNT(*) as _count FROM (%s)", sql)
					countRows, countErr := s.db.QueryRowsWithParams(countSQL, pathParams)
					total := 0
					if countErr == nil && len(countRows) > 0 {
						fmt.Sscanf(countRows[0]["_count"], "%d", &total)
					}
					offset := (pageNum - 1) * node.Paginate
					sql = fmt.Sprintf("%s LIMIT %d OFFSET %d", sql, node.Paginate, offset)
					ctx.paginate[queryName] = PaginateInfo{
						Page:    pageNum,
						PerPage: node.Paginate,
						Total:   total,
						HasPrev: pageNum > 1,
						HasNext: offset+node.Paginate < total,
					}
				}
				rows, err := s.db.QueryRowsWithParams(sql, pathParams)
				if err != nil {
					s.logger.LogError("fragment query failed", err)
					body.WriteString("<p style=\"color:red\">Query error</p>")
					continue
				}
				ctx.queries[queryName] = rows
			}

		case parser.NodeText:
			text := s.i18n.TranslateAll(node.Value, r)
			text = interpolate(text, ctx)
			body.WriteString(fmt.Sprintf("<p>%s</p>\n", html.EscapeString(text)))

		case parser.NodeHTML:
			htmlContent := s.i18n.TranslateAll(node.HTMLContent, r)
			htmlContent = renderHTML(htmlContent, ctx)
			body.WriteString(htmlContent + "\n")
		}
	}

	return body.String()
}

// renderFragmentWithParams renders a fragment using provided params (for WebSocket broadcast)
func (s *Server) renderFragmentWithParams(frag parser.Page, params map[string]string) string {
	ctx := &renderContext{
		queries:  make(map[string][]database.Row),
		paginate: make(map[string]PaginateInfo),
	}

	var body strings.Builder

	for _, node := range frag.Body {
		switch node.Type {
		case parser.NodeQuery:
			if s.db != nil {
				rows, err := s.db.QueryRowsWithParams(node.SQL, params)
				if err != nil {
					s.logger.LogError("fragment query failed", err)
					body.WriteString("<p style=\"color:red\">Query error</p>")
					continue
				}
				name := node.Name
				if name == "" {
					name = "_last"
				}
				ctx.queries[name] = rows
			}

		case parser.NodeText:
			text := interpolate(node.Value, ctx)
			body.WriteString(fmt.Sprintf("<p>%s</p>\n", html.EscapeString(text)))

		case parser.NodeHTML:
			htmlContent := renderHTML(node.HTMLContent, ctx)
			body.WriteString(htmlContent + "\n")
		}
	}

	return body.String()
}

// handleAction processes a POST/PUT/DELETE request.
// All mutation queries within an action are wrapped in an implicit transaction.
func (s *Server) handleAction(w http.ResponseWriter, r *http.Request, action parser.Page, app *parser.App) {
	formData := extractFormData(r, app.Config)

	// Verify CSRF token
	csrfToken := formData["_csrf"]
	if !validateCSRFToken(csrfToken) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	delete(formData, "_csrf")

	// Merge path params into form data
	pathParams := matchPathParams(action.Path, r.URL.Path)
	for k, v := range pathParams {
		formData[k] = v
	}

	// Add current_user fields (both dot and underscore formats)
	session := s.getSession(r)
	if session != nil {
		formData["current_user.id"] = session.UserID
		formData["current_user.identity"] = session.Identity
		formData["current_user.role"] = session.Role
		formData["current_user_id"] = session.UserID
		formData["current_user_identity"] = session.Identity
		formData["current_user_role"] = session.Role
	}

	// Start implicit transaction for atomicity
	var tx *database.TxHandle
	if s.db != nil {
		var err error
		tx, err = s.db.BeginTxHandle()
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback() // no-op if already committed
	}

	// Track last query result for `on` branching
	lastQueryOk := true
	lastQueryNotFound := false

	// Process body nodes
	for _, node := range action.Body {
		switch node.Type {
		case parser.NodeValidate:
			modelName := node.ModelName
			if modelName == "" {
				// Inline validation rules
				if len(node.Validations) > 0 {
					errors := validateInlineRules(node.Validations, formData)
					if len(errors) > 0 {
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
						w.WriteHeader(http.StatusUnprocessableEntity)
						var errorHTML strings.Builder
						for _, e := range errors {
							errorHTML.WriteString(fmt.Sprintf("<div class=\"kilnx-alert kilnx-alert-error\">%s</div>", html.EscapeString(e)))
						}
						w.Write([]byte(errorHTML.String()))
						return // tx.Rollback via defer
					}
				}
				continue
			}
			errors := validateFormData(modelName, app, formData)
			if len(errors) > 0 {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusUnprocessableEntity)
				referer := r.Header.Get("Referer")
				if referer == "" {
					referer = "/"
				}
				var errorHTML strings.Builder
				for _, e := range errors {
					errorHTML.WriteString(fmt.Sprintf("<div class=\"kilnx-alert kilnx-alert-error\">%s</div>", html.EscapeString(e)))
				}
				w.Write([]byte(errorHTML.String()))
				return // tx.Rollback via defer
			}

		case parser.NodeQuery:
			if tx != nil {
				trimmed := strings.TrimSpace(strings.ToUpper(node.SQL))
				if strings.HasPrefix(trimmed, "SELECT") {
					rows, err := tx.QueryRowsWithParams(node.SQL, formData)
					if err != nil {
						lastQueryOk = false
						lastQueryNotFound = false
						continue
					}
					lastQueryOk = true
					lastQueryNotFound = len(rows) == 0
					name := node.Name
					if name == "" {
						name = "_last"
					}
					for _, row := range rows {
						for k, v := range row {
							formData[name+"."+k] = v
						}
					}
				} else {
					err := tx.ExecWithParams(node.SQL, formData)
					if err != nil {
						lastQueryOk = false
						lastQueryNotFound = false
						// Check if there's an on error handler in remaining nodes
						hasOnError := false
						for _, remaining := range action.Body {
							if remaining.Type == parser.NodeOn {
								if cond, ok := remaining.Props["condition"]; ok && cond == "error" {
									hasOnError = true
									break
								}
							}
						}
						if !hasOnError {
							s.logger.LogError("action query failed", err)
							http.Error(w, "Internal server error", http.StatusInternalServerError)
							return // tx.Rollback via defer
						}
						continue
					}
					lastQueryOk = true
					lastQueryNotFound = false
				}
			}

		case parser.NodeOn:
			condition := node.Props["condition"]
			shouldExecute := false
			switch condition {
			case "success":
				shouldExecute = lastQueryOk && !lastQueryNotFound
			case "error":
				shouldExecute = !lastQueryOk
			case "not found":
				shouldExecute = lastQueryNotFound
			case "forbidden":
				sess := s.getSession(r)
				if sess == nil {
					shouldExecute = true
				} else if action.RequiresRole != "" && action.RequiresRole != "auth" {
					shouldExecute = sess.Role != action.RequiresRole &&
						!s.hasPermission(sess.Role, action.RequiresRole, app.Permissions)
				}
			}
			if shouldExecute {
				if tx != nil {
					tx.Commit()
				}
				s.handleActionNodes(w, r, node.Children, formData, app)
				return
			}

		case parser.NodeSendEmail:
			recipient := resolveEmailRecipient(node.EmailTo, formData)
			// Resolve recipient from SQL query if specified
			if toQuery, ok := node.Props["to_query"]; ok && toQuery != "" && s.db != nil {
				rows, err := s.db.QueryRowsWithParams(toQuery, formData)
				if err == nil && len(rows) > 0 {
					for _, v := range rows[0] {
						recipient = v
						break
					}
				}
			}
			subject := node.EmailSubject
			emailBody := node.Props["body"]
			if emailBody == "" {
				emailBody = subject
			}
			templateName := node.EmailTemplate
			paramsCopy := make(map[string]string, len(formData))
			for k, v := range formData {
				paramsCopy[k] = v
			}
			go func(to, subj, body, tmpl string, params map[string]string) {
				if err := SendEmailWithTemplate(to, subj, body, tmpl, params); err != nil {
					fmt.Printf("  email error: %v\n", err)
				}
			}(recipient, subject, emailBody, templateName, paramsCopy)

		case parser.NodeEnqueue:
			if s.jobQueue != nil {
				resolvedParams := make(map[string]string)
				for k, v := range node.JobParams {
					if strings.HasPrefix(v, ":") {
						paramName := strings.TrimPrefix(v, ":")
						if val, ok := formData[paramName]; ok {
							resolvedParams[k] = val
							continue
						}
					}
					resolvedParams[k] = v
				}
				if err := s.jobQueue.Enqueue(node.JobName, resolvedParams); err != nil {
					fmt.Printf("  enqueue error: %v\n", err)
				}
			}

		case parser.NodeGeneratePDF:
			doc := pdf.NewDocument()
			doc.SetTitle(node.TemplateName)
			doc.SetFooter("Page {page} of {pages}")
			pdfPage := doc.AddPage()
			pdfPage.AddHeading(node.TemplateName)
			pdfPage.AddSpace(10)

			dataName := node.DataQueryName
			if dataName != "" && tx != nil {
				// Re-run the named query to get full rows for the PDF
				for _, prevNode := range action.Body {
					if prevNode.Type == parser.NodeQuery && prevNode.Name == dataName {
						rows, err := tx.QueryRowsWithParams(prevNode.SQL, formData)
						if err == nil && len(rows) > 0 {
							var headers []string
							for key := range rows[0] {
								headers = append(headers, key)
							}
							var tableRows [][]string
							for _, row := range rows {
								var tr []string
								for _, h := range headers {
									tr = append(tr, row[h])
								}
								tableRows = append(tableRows, tr)
							}
							pdfPage.AddTable(headers, tableRows)
						}
						break
					}
				}
			}

			pdfBytes := doc.Render()
			tmpFile, err := os.CreateTemp("", "kilnx-*.pdf")
			if err != nil {
				fmt.Printf("  pdf generation error: %v\n", err)
				http.Error(w, "PDF generation error", http.StatusInternalServerError)
				return
			}
			if _, err := tmpFile.Write(pdfBytes); err != nil {
				fmt.Printf("  pdf write error: %v\n", err)
				tmpFile.Close()
				os.Remove(tmpFile.Name())
				http.Error(w, "PDF write error", http.StatusInternalServerError)
				return
			}
			tmpFile.Close()
			formData["_generated_pdf"] = tmpFile.Name()
			fmt.Printf("  generated pdf: %s\n", tmpFile.Name())

		case parser.NodeFetch:
			fetchName := node.Name
			if fetchName == "" {
				fetchName = "_fetch"
			}
			rows, err := executeFetch(node, formData)
			if err != nil {
				fmt.Printf("  fetch error in action: %v\n", err)
			} else if len(rows) > 0 {
				for k, v := range rows[0] {
					formData["fetch."+k] = v
				}
			}

		case parser.NodeRedirect:
			path := node.Value
			for k, v := range formData {
				path = strings.ReplaceAll(path, ":"+k, url.PathEscape(v))
			}
			if tx != nil {
				tx.Commit()
			}
			if r.Header.Get("HX-Request") == "true" {
				// For htmx: find a fragment under the redirect path
				// e.g., redirect to /channel/6 finds fragment /channel/:id/messages
				rendered := false
				for _, frag := range app.Fragments {
					// Check if fragment path starts with same prefix as redirect
					fragParts := strings.Split(frag.Path, "/")
					pathParts := strings.Split(path, "/")
					if len(fragParts) > len(pathParts) {
						match := true
						for i, pp := range pathParts {
							fp := fragParts[i]
							if fp != pp && !strings.HasPrefix(fp, ":") {
								match = false
								break
							}
						}
						if match {
							// Build the actual fragment URL using redirect path values
							actualPath := path + "/" + strings.Join(fragParts[len(pathParts):], "/")
							fakeReq := r.Clone(r.Context())
							fakeReq.URL.Path = actualPath
							content := s.renderFragment(frag, fakeReq)
							w.Header().Set("Content-Type", "text/html; charset=utf-8")
							w.Write([]byte(content))
							rendered = true
							break
						}
					}
				}
				if !rendered {
					w.Header().Set("HX-Redirect", path)
					w.WriteHeader(http.StatusOK)
				}
			} else {
				http.Redirect(w, r, path, http.StatusSeeOther)
			}
			return

		case parser.NodeRespond:
			if node.StatusCode > 0 {
				if tx != nil {
					tx.Commit()
				}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(node.StatusCode)
				return
			}

			if node.RespondSwap == "delete" {
				if tx != nil {
					tx.Commit()
				}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Header().Set("HX-Reswap", "delete")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(""))
				return
			}

			if node.QuerySQL != "" && tx != nil {
				rows, err := tx.QueryRowsWithParams(node.QuerySQL, formData)
				if err != nil {
					s.logger.LogError("respond query failed", err)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}
				tx.Commit()
				ctx := &renderContext{
					queries: map[string][]database.Row{"_result": rows},
				}
				if node.RespondTarget != "" {
					w.Header().Set("HX-Retarget", node.RespondTarget)
				}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				// Try to find and render the matching fragment
				if node.RespondTarget != "" {
					for _, frag := range app.Fragments {
						fragName := strings.TrimPrefix(frag.Path, "/")
						if fragName == node.RespondTarget {
							fragParams := make(map[string]string)
							for k, v := range formData {
								fragParams[k] = v
							}
							// Add query result fields to params
							if len(rows) > 0 {
								for k, v := range rows[0] {
									fragParams[k] = v
								}
							}
							rendered := s.renderFragmentWithParams(frag, fragParams)
							w.Write([]byte(rendered))
							return
						}
					}
				}
				// Fallback: render rows as structured HTML using the template engine
				var result strings.Builder
				result.WriteString("{{each _result}}")
				for _, row := range rows {
					for key := range row {
						result.WriteString(fmt.Sprintf("<span class=\"%s\">{%s}</span> ", key, key))
					}
					break // only need first row for template structure
				}
				result.WriteString("{{end}}")
				rendered := renderHTML(result.String(), ctx)
				w.Write([]byte(rendered))
				return
			}

			if tx != nil {
				tx.Commit()
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	// Commit transaction before default redirect
	if tx != nil {
		tx.Commit()
	}

	// Default: redirect back to referer (validated) or /
	referer := r.Header.Get("Referer")
	redirectTo := "/"
	if referer != "" {
		if parsed, err := url.Parse(referer); err == nil && isLocalPath(parsed.Path) {
			redirectTo = parsed.Path
			if parsed.RawQuery != "" {
				redirectTo += "?" + parsed.RawQuery
			}
		}
	}
	http.Redirect(w, r, redirectTo, http.StatusSeeOther)
}

// handleActionNodes processes action nodes inside `on` branches.
// Supports redirect, respond, send email, query execution, and job enqueue.
func (s *Server) handleActionNodes(w http.ResponseWriter, r *http.Request, nodes []parser.Node, formData map[string]string, app *parser.App) {
	for _, node := range nodes {
		switch node.Type {
		case parser.NodeRedirect:
			path := node.Value
			for k, v := range formData {
				path = strings.ReplaceAll(path, ":"+k, url.PathEscape(v))
			}
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", path)
				w.WriteHeader(http.StatusOK)
			} else {
				http.Redirect(w, r, path, http.StatusSeeOther)
			}
			return
		case parser.NodeRespond:
			if node.StatusCode > 0 {
				w.WriteHeader(node.StatusCode)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			return
		case parser.NodeSendEmail:
			recipient := resolveEmailRecipient(node.EmailTo, formData)
			if toQuery, ok := node.Props["to_query"]; ok && toQuery != "" && s.db != nil {
				rows, err := s.db.QueryRowsWithParams(toQuery, formData)
				if err == nil && len(rows) > 0 {
					for _, v := range rows[0] {
						recipient = v
						break
					}
				}
			}
			subject := node.EmailSubject
			emailBody := node.Props["body"]
			if emailBody == "" {
				emailBody = subject
			}
			templateName := node.EmailTemplate
			paramsCopy := make(map[string]string, len(formData))
			for k, v := range formData {
				paramsCopy[k] = v
			}
			go func(to, subj, body, tmpl string, params map[string]string) {
				if err := SendEmailWithTemplate(to, subj, body, tmpl, params); err != nil {
					fmt.Printf("  email error: %v\n", err)
				}
			}(recipient, subject, emailBody, templateName, paramsCopy)
		case parser.NodeQuery:
			if s.db != nil {
				trimmed := strings.TrimSpace(strings.ToUpper(node.SQL))
				if strings.HasPrefix(trimmed, "SELECT") {
					rows, err := s.db.QueryRowsWithParams(node.SQL, formData)
					if err != nil {
						continue
					}
					name := node.Name
					if name == "" {
						name = "_last"
					}
					for _, row := range rows {
						for k, v := range row {
							formData[name+"."+k] = v
						}
					}
				} else {
					if err := s.db.ExecWithParams(node.SQL, formData); err != nil {
						fmt.Printf("  on-branch query error: %v\n", err)
					}
				}
			}
		case parser.NodeValidate:
			modelName := node.ModelName
			var errors []string
			if modelName == "" {
				if len(node.Validations) > 0 {
					errors = validateInlineRules(node.Validations, formData)
				}
			} else {
				errors = validateFormData(modelName, app, formData)
			}
			if len(errors) > 0 {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusUnprocessableEntity)
				var errorHTML strings.Builder
				for _, e := range errors {
					errorHTML.WriteString(fmt.Sprintf("<div class=\"kilnx-alert kilnx-alert-error\">%s</div>", html.EscapeString(e)))
				}
				w.Write([]byte(errorHTML.String()))
				return
			}
		case parser.NodeEnqueue:
			if s.jobQueue != nil {
				resolvedParams := make(map[string]string)
				for k, v := range node.JobParams {
					if strings.HasPrefix(v, ":") {
						paramName := strings.TrimPrefix(v, ":")
						if val, ok := formData[paramName]; ok {
							resolvedParams[k] = val
							continue
						}
					}
					resolvedParams[k] = v
				}
				if err := s.jobQueue.Enqueue(node.JobName, resolvedParams); err != nil {
					fmt.Printf("  enqueue error: %v\n", err)
				}
			}
		}
	}
}

// matchPath checks if a route pattern matches a URL path.
// Supports :param segments: /users/:id matches /users/5
func matchPath(pattern, urlPath string) bool {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	urlParts := strings.Split(strings.Trim(urlPath, "/"), "/")

	if len(patternParts) != len(urlParts) {
		return false
	}

	for i, pp := range patternParts {
		if strings.HasPrefix(pp, ":") {
			continue // param segment matches anything
		}
		if pp != urlParts[i] {
			return false
		}
	}

	return true
}

// matchPathParams extracts :param values from a URL given a pattern
func matchPathParams(pattern, urlPath string) map[string]string {
	params := make(map[string]string)
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	urlParts := strings.Split(strings.Trim(urlPath, "/"), "/")

	for i, pp := range patternParts {
		if strings.HasPrefix(pp, ":") && i < len(urlParts) {
			params[strings.TrimPrefix(pp, ":")] = urlParts[i]
		}
	}

	return params
}
