package runtime

import (
	"embed"
	"fmt"
	"html"
	"net/http"
	"strings"
	"sync"

	"github.com/kilnx-org/kilnx/internal/parser"
)

//go:embed static/htmx.min.js
var staticFS embed.FS

type Server struct {
	app  *parser.App
	mu   sync.RWMutex
	port int
}

func NewServer(app *parser.App, port int) *Server {
	return &Server{app: app, port: port}
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

		// Find matching page
		for _, page := range app.Pages {
			if r.URL.Path == page.Path {
				content := renderPage(page, app.Pages)
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

func Serve(app *parser.App, port int) error {
	s := NewServer(app, port)
	return s.Start()
}

func renderPage(p parser.Page, allPages []parser.Page) string {
	var body strings.Builder

	for _, node := range p.Body {
		switch node.Type {
		case parser.NodeText:
			body.WriteString(fmt.Sprintf("    <p>%s</p>\n", html.EscapeString(node.Value)))
		}
	}

	title := p.Title
	if title == "" {
		title = "kilnx"
	}

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
  </style>
</head>
<body>
%s  <main>
%s  </main>
</body>
</html>
`, title, nav, body.String())
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
