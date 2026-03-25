package runtime

import (
	"fmt"
	"html"
	"strings"

	"github.com/kilnx-org/kilnx/internal/parser"
)

const kilnxCSS = `    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { font-family: system-ui, -apple-system, sans-serif; line-height: 1.6; color: #1a1a1a; max-width: 800px; margin: 0 auto; padding: 1rem; }
    nav { display: flex; gap: 1rem; padding: 0.75rem 0; border-bottom: 1px solid #e0e0e0; margin-bottom: 1.5rem; flex-wrap: wrap; }
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
    .kilnx-search { margin-bottom: 1rem; }
    .kilnx-search input { width: 100%%; padding: 0.5rem 0.75rem; border: 1px solid #ddd; border-radius: 4px; font-size: 0.9rem; font-family: inherit; }
    .kilnx-search input:focus { outline: none; border-color: #4a7aba; box-shadow: 0 0 0 2px rgba(74,122,186,0.15); }
    .kilnx-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(250px, 1fr)); gap: 1rem; }
    .kilnx-card { border: 1px solid #e0e0e0; border-radius: 8px; overflow: hidden; }
    .kilnx-card-img { width: 100%%; height: 160px; object-fit: cover; }
    .kilnx-card-body { padding: 0.75rem; }
    .kilnx-card-title { font-size: 1rem; margin-bottom: 0.25rem; }
    .kilnx-card-subtitle { font-size: 0.85rem; color: #666; margin-bottom: 0.5rem; }
    .kilnx-card-action { font-size: 0.85rem; color: #4a7aba; text-decoration: none; }
    .kilnx-card-action:hover { text-decoration: underline; }
    .kilnx-modal { position: fixed; top: 0; left: 0; width: 100%%; height: 100%%; z-index: 1000; display: flex; align-items: center; justify-content: center; }
    .kilnx-modal-overlay { position: absolute; top: 0; left: 0; width: 100%%; height: 100%%; background: rgba(0,0,0,0.5); }
    .kilnx-modal-content { position: relative; background: white; border-radius: 8px; max-width: 600px; width: 90%%; max-height: 80vh; overflow-y: auto; }
    .kilnx-modal-header { display: flex; justify-content: space-between; align-items: center; padding: 1rem; border-bottom: 1px solid #e0e0e0; }
    .kilnx-modal-header h3 { margin: 0; }
    .kilnx-modal-close { background: none; border: none; font-size: 1.5rem; cursor: pointer; color: #888; }
    .kilnx-modal-body { padding: 1rem; }`

func renderDefaultLayout(title, nav, content string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s</title>
  <script src="/_kilnx/htmx.min.js"></script>
  <script src="/_kilnx/sse.js"></script>
  <style>
%s
  </style>
</head>
<body>
%s  <main>
%s  </main>
</body>
</html>
`, html.EscapeString(title), kilnxCSS, nav, content)
}

func renderWithLayout(layout parser.Layout, title, nav, content string) string {
	result := layout.HTMLContent

	// Replace placeholders
	result = strings.ReplaceAll(result, "{page.title}", html.EscapeString(title))
	result = strings.ReplaceAll(result, "{page.content}", content)
	result = strings.ReplaceAll(result, "{nav}", nav)
	result = strings.ReplaceAll(result, "{kilnx.css}", kilnxCSS)
	result = strings.ReplaceAll(result, "{kilnx.js}", `<script src="/_kilnx/htmx.min.js"></script>
<script src="/_kilnx/sse.js"></script>`)

	return result
}

// renderComponent renders a custom component with given args
func renderComponent(comp parser.Component, args map[string]string) string {
	var b strings.Builder

	for _, node := range comp.Body {
		switch node.Type {
		case parser.NodeHTML:
			content := node.HTMLContent
			// Replace param placeholders
			for _, param := range comp.Params {
				placeholder := "{" + param + "}"
				if val, ok := args[param]; ok {
					content = strings.ReplaceAll(content, placeholder, html.EscapeString(val))
				} else {
					content = strings.ReplaceAll(content, placeholder, "")
				}
			}
			b.WriteString(content)
			b.WriteString("\n")

		case parser.NodeText:
			text := node.Value
			for _, param := range comp.Params {
				placeholder := "{" + param + "}"
				if val, ok := args[param]; ok {
					text = strings.ReplaceAll(text, placeholder, val)
				}
			}
			b.WriteString(fmt.Sprintf("<p>%s</p>\n", html.EscapeString(text)))
		}
	}

	return b.String()
}
