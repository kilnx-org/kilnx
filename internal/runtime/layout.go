package runtime

import (
	"fmt"
	"html"
	"strings"

	"github.com/kilnx-org/kilnx/internal/parser"
)

func renderDefaultLayout(title, nav, content string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s</title>
  <script src="/_kilnx/htmx.min.js"></script>
  <script src="/_kilnx/sse.js"></script>
</head>
<body>
%s  <main>
%s  </main>
</body>
</html>
`, html.EscapeString(title), nav, content)
}

func renderWithLayout(layout parser.Layout, title, nav, content string) string {
	result := layout.HTMLContent

	result = strings.ReplaceAll(result, "{page.title}", html.EscapeString(title))
	result = strings.ReplaceAll(result, "{page.content}", content)
	result = strings.ReplaceAll(result, "{nav}", nav)
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
