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

