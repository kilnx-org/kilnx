package runtime

import (
	"embed"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

//go:embed static/htmx.min.js
var staticFS embed.FS

var interpolateRe = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*(?:\.[a-zA-Z_][a-zA-Z0-9_]*)?)\}`)

type Server struct {
	app  *parser.App
	db   *database.DB
	mu   sync.RWMutex
	port int
}

func NewServer(app *parser.App, db *database.DB, port int) *Server {
	return &Server{app: app, db: db, port: port}
}

func (s *Server) Reload(app *parser.App) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.app = app
}

func (s *Server) getApp() *parser.App {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.app
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Serve embedded htmx.js
	mux.HandleFunc("/_kilnx/htmx.min.js", func(w http.ResponseWriter, r *http.Request) {
		data, _ := staticFS.ReadFile("static/htmx.min.js")
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.Write(data)
	})

	// Catch-all handler that resolves routes dynamically (supports hot reload)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		app := s.getApp()

		// Handle POST/PUT/DELETE -> match actions
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
			for _, action := range app.Actions {
				if matchPath(action.Path, r.URL.Path) {
					s.handleAction(w, r, action, app)
					return
				}
			}
			// Also check pages with method POST (legacy)
			for _, page := range app.Pages {
				if page.Method == "POST" && matchPath(page.Path, r.URL.Path) {
					s.handleAction(w, r, page, app)
					return
				}
			}
		}

		// Match fragments (partial HTML, no page wrapper)
		for _, frag := range app.Fragments {
			if matchPath(frag.Path, r.URL.Path) {
				content := s.renderFragment(frag, r)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write([]byte(content))
				return
			}
		}

		// Find matching page (GET)
		for _, page := range app.Pages {
			if matchPath(page.Path, r.URL.Path) {
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
	return http.ListenAndServe(addr, mux)
}

// renderContext holds query results available during template rendering
type renderContext struct {
	queries  map[string][]database.Row
	paginate map[string]PaginateInfo
}

func (s *Server) renderPage(p parser.Page, allPages []parser.Page, r *http.Request) string {
	ctx := &renderContext{
		queries:  make(map[string][]database.Row),
		paginate: make(map[string]PaginateInfo),
	}

	// Get current page number from query string
	pageNum := 1
	if pg := r.URL.Query().Get("page"); pg != "" {
		fmt.Sscanf(pg, "%d", &pageNum)
		if pageNum < 1 {
			pageNum = 1
		}
	}

	var body strings.Builder

	for _, node := range p.Body {
		switch node.Type {
		case parser.NodeQuery:
			if s.db != nil {
				sql := node.SQL

				// Handle pagination
				if node.Paginate > 0 {
					// First get total count
					countSQL := fmt.Sprintf("SELECT COUNT(*) as _count FROM (%s)", sql)
					countRows, err := s.db.QueryRows(countSQL)
					total := 0
					if err == nil && len(countRows) > 0 {
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

				rows, err := s.db.QueryRows(sql)
				if err != nil {
					body.WriteString(fmt.Sprintf("    <p style=\"color:red\">Query error: %s</p>\n",
						html.EscapeString(err.Error())))
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
			body.WriteString(fmt.Sprintf("    <p>%s</p>\n", html.EscapeString(text)))

		case parser.NodeList:
			body.WriteString(renderList(node, ctx))

		case parser.NodeTable:
			body.WriteString(renderTable(node, ctx, p.Path))

		case parser.NodeAlert:
			body.WriteString(renderAlert(node, ctx))

		case parser.NodeForm:
			body.WriteString(renderForm(node, s.getApp(), s.db, r))

		case parser.NodeHTML:
			htmlContent := interpolate(node.HTMLContent, ctx)
			body.WriteString("    " + htmlContent + "\n")
		}
	}

	title := p.Title
	if title == "" {
		title = "kilnx"
	}

	// Interpolate title too
	title = interpolate(title, ctx)

	nav := renderNav(allPages, p.Path)

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s</title>
  <script src="/_kilnx/htmx.min.js"></script>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { font-family: system-ui, -apple-system, sans-serif; line-height: 1.6; color: #1a1a1a; max-width: 800px; margin: 0 auto; padding: 1rem; }
    nav { display: flex; gap: 1rem; padding: 0.75rem 0; border-bottom: 1px solid #e0e0e0; margin-bottom: 1.5rem; }
    nav a { text-decoration: none; color: #555; font-size: 0.9rem; }
    nav a:hover { color: #1a1a1a; }
    nav a.active { color: #1a1a1a; font-weight: 600; }
    main { padding: 0.5rem 0; }
    p { margin-bottom: 0.75rem; }
    .kilnx-list { list-style: none; }
    .kilnx-list-item { padding: 0.6rem 0; border-bottom: 1px solid #f0f0f0; display: flex; flex-direction: column; gap: 0.15rem; }
    .kilnx-list-item strong { font-size: 0.95rem; }
    .kilnx-list-item span { font-size: 0.85rem; color: #666; }
    .kilnx-table { width: 100%%; border-collapse: collapse; font-size: 0.9rem; }
    .kilnx-table th { text-align: left; padding: 0.5rem; border-bottom: 2px solid #e0e0e0; font-weight: 600; font-size: 0.8rem; text-transform: uppercase; color: #888; }
    .kilnx-table td { padding: 0.5rem; border-bottom: 1px solid #f0f0f0; }
    .kilnx-table tr:hover { background: #fafafa; }
    .kilnx-table td a { color: #4a7aba; text-decoration: none; font-size: 0.85rem; }
    .kilnx-table td a:hover { text-decoration: underline; }
    .kilnx-alert { padding: 0.75rem 1rem; border-radius: 4px; margin-bottom: 1rem; font-size: 0.9rem; }
    .kilnx-alert-success { background: #f0fdf0; color: #166534; border: 1px solid #bbf7d0; }
    .kilnx-alert-error { background: #fef2f2; color: #991b1b; border: 1px solid #fecaca; }
    .kilnx-alert-warning { background: #fffbeb; color: #92400e; border: 1px solid #fde68a; }
    .kilnx-alert-info { background: #eff6ff; color: #1e40af; border: 1px solid #bfdbfe; }
    .kilnx-pagination { display: flex; align-items: center; gap: 1rem; padding: 1rem 0; justify-content: center; }
    .kilnx-pagination a { color: #4a7aba; text-decoration: none; font-size: 0.9rem; }
    .kilnx-pagination a:hover { text-decoration: underline; }
    .kilnx-pagination .disabled { color: #ccc; font-size: 0.9rem; }
    .kilnx-page-info { font-size: 0.85rem; color: #888; }
    .kilnx-form { display: flex; flex-direction: column; gap: 0.75rem; max-width: 500px; }
    .kilnx-field { display: flex; flex-direction: column; gap: 0.25rem; }
    .kilnx-field label { font-size: 0.85rem; font-weight: 500; color: #555; }
    .kilnx-field input, .kilnx-field textarea, .kilnx-field select { padding: 0.5rem; border: 1px solid #ddd; border-radius: 4px; font-size: 0.9rem; font-family: inherit; }
    .kilnx-field input:focus, .kilnx-field textarea:focus, .kilnx-field select:focus { outline: none; border-color: #4a7aba; box-shadow: 0 0 0 2px rgba(74,122,186,0.15); }
    .kilnx-btn { padding: 0.5rem 1.25rem; background: #1a1a1a; color: white; border: none; border-radius: 4px; font-size: 0.9rem; cursor: pointer; align-self: flex-start; }
    .kilnx-btn:hover { background: #333; }
  </style>
</head>
<body>
%s  <main>
%s  </main>
</body>
</html>
`, html.EscapeString(title), nav, body.String())
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

func renderNav(pages []parser.Page, currentPath string) string {
	var nav strings.Builder
	nav.WriteString("  <nav>\n")
	for _, p := range pages {
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
		nav.WriteString(fmt.Sprintf("    <a href=\"%s\"%s>%s</a>\n", p.Path, class, html.EscapeString(label)))
	}
	nav.WriteString("  </nav>\n")
	return nav.String()
}

func render404(path string, pages []parser.Page) string {
	nav := renderNav(pages, "")
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
		queries:  make(map[string][]database.Row),
		paginate: make(map[string]PaginateInfo),
	}

	// Get path params
	pathParams := matchPathParams(frag.Path, r.URL.Path)

	var body strings.Builder

	for _, node := range frag.Body {
		switch node.Type {
		case parser.NodeQuery:
			if s.db != nil {
				rows, err := s.db.QueryRowsWithParams(node.SQL, pathParams)
				if err != nil {
					body.WriteString(fmt.Sprintf("<p style=\"color:red\">Query error: %s</p>",
						html.EscapeString(err.Error())))
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

		case parser.NodeList:
			body.WriteString(renderList(node, ctx))

		case parser.NodeTable:
			body.WriteString(renderTable(node, ctx, frag.Path))

		case parser.NodeAlert:
			body.WriteString(renderAlert(node, ctx))

		case parser.NodeHTML:
			htmlContent := interpolate(node.HTMLContent, ctx)
			body.WriteString(htmlContent + "\n")
		}
	}

	return body.String()
}

// handleAction processes a POST/PUT/DELETE request
func (s *Server) handleAction(w http.ResponseWriter, r *http.Request, action parser.Page, app *parser.App) {
	formData := extractFormData(r)

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

	// Process body nodes
	for _, node := range action.Body {
		switch node.Type {
		case parser.NodeValidate:
			modelName := node.ModelName
			if modelName == "" {
				continue
			}
			errors := validateFormData(modelName, app, formData)
			if len(errors) > 0 {
				// Re-render the referring page with errors
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusUnprocessableEntity)
				referer := r.Header.Get("Referer")
				if referer == "" {
					referer = "/"
				}
				// Simple error response
				var errorHTML strings.Builder
				for _, e := range errors {
					errorHTML.WriteString(fmt.Sprintf("<div class=\"kilnx-alert kilnx-alert-error\">%s</div>", html.EscapeString(e)))
				}
				w.Write([]byte(errorHTML.String()))
				return
			}

		case parser.NodeQuery:
			if s.db != nil {
				err := s.db.ExecWithParams(node.SQL, formData)
				if err != nil {
					http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
					return
				}
			}

		case parser.NodeRedirect:
			path := node.Value
			// Interpolate :params in redirect path
			for k, v := range formData {
				path = strings.ReplaceAll(path, ":"+k, v)
			}
			// If htmx request, use HX-Redirect header
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", path)
				w.WriteHeader(http.StatusOK)
			} else {
				http.Redirect(w, r, path, http.StatusSeeOther)
			}
			return

		case parser.NodeRespond:
			// respond fragment delete -> empty body, htmx removes element
			if node.RespondSwap == "delete" {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Header().Set("HX-Reswap", "delete")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(""))
				return
			}

			// respond fragment with query: SQL
			if node.QuerySQL != "" && s.db != nil {
				rows, err := s.db.QueryRowsWithParams(node.QuerySQL, formData)
				if err != nil {
					http.Error(w, "Query error: "+err.Error(), http.StatusInternalServerError)
					return
				}
				ctx := &renderContext{
					queries: map[string][]database.Row{"_result": rows},
				}
				// If there's a target, set HX-Retarget
				if node.RespondTarget != "" {
					w.Header().Set("HX-Retarget", node.RespondTarget)
				}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				// Render as simple list of values
				var result strings.Builder
				for _, row := range rows {
					for _, val := range row {
						result.WriteString(html.EscapeString(val))
					}
				}
				_ = ctx
				w.Write([]byte(result.String()))
				return
			}

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	// Default: redirect back to referer or /
	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/"
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
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
