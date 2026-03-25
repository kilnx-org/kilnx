package runtime

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/kilnx-org/kilnx/internal/parser"
)

const htmxVersion = "2.0.4"

func Serve(app *parser.App, port int) error {
	mux := http.NewServeMux()

	for _, page := range app.Pages {
		p := page // capture for closure
		mux.HandleFunc(p.Path, func(w http.ResponseWriter, r *http.Request) {
			// Only match exact paths
			if r.URL.Path != p.Path {
				http.NotFound(w, r)
				return
			}

			html := renderPage(p)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(html))
		})
	}

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("kilnx serving on http://localhost%s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func renderPage(p parser.Page) string {
	var body strings.Builder

	for _, node := range p.Body {
		switch node.Type {
		case parser.NodeText:
			body.WriteString(fmt.Sprintf("  <p>%s</p>\n", node.Value))
		}
	}

	title := p.Title
	if title == "" {
		title = p.Path
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s</title>
  <script src="https://unpkg.com/htmx.org@%s"></script>
</head>
<body>
%s</body>
</html>
`, title, htmxVersion, body.String())
}
