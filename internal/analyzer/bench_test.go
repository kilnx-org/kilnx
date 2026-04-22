package analyzer

import (
	"testing"

	"github.com/kilnx-org/kilnx/internal/lexer"
	"github.com/kilnx-org/kilnx/internal/parser"
)

func benchApp(n int) *parser.App {
	src := ""
	for i := 0; i < n; i++ {
		src += "model m" + string(rune('0'+i%10)) + "\n  name: text required\n  email: email unique\n\n"
	}
	src += `page /dashboard requires auth
  query stats: SELECT count(*) as total FROM m0
  query users: SELECT name, email FROM m0
  html
    <p>Total: {stats.total}</p>
    {{each users}}<p>{name}</p>{{end}}
`
	stripped := lexer.StripComments(src)
	tokens := lexer.Tokenize(stripped)
	app, _ := parser.Parse(tokens, stripped)
	return app
}

func BenchmarkAnalyze10(b *testing.B) {
	app := benchApp(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Analyze(app)
	}
}

func BenchmarkAnalyze50(b *testing.B) {
	app := benchApp(50)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Analyze(app)
	}
}
