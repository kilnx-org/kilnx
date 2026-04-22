package runtime

import (
	"testing"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/lexer"
	"github.com/kilnx-org/kilnx/internal/parser"
)

func benchRenderApp() *parser.App {
	src := `model user
  name: text required
  email: email unique

page /users
  query users: SELECT name, email FROM user
  html
    <ul>
    {{each users}}<li>{name} — {email}</li>{{end}}
    </ul>
`
	stripped := lexer.StripComments(src)
	tokens := lexer.Tokenize(stripped)
	app, _ := parser.Parse(tokens, stripped)
	return app
}

func BenchmarkRenderPage(b *testing.B) {
	app := benchRenderApp()
	ctx := &renderContext{
		queries: map[string][]database.Row{
			"users": {
				{"name": "Alice", "email": "alice@test.com"},
				{"name": "Bob", "email": "bob@test.com"},
				{"name": "Carol", "email": "carol@test.com"},
			},
		},
	}
	node := app.Pages[0].Body[1] // html node
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = renderHTML(node.HTMLContent, ctx)
	}
}
