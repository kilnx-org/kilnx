package parser

import (
	"testing"

	"github.com/kilnx-org/kilnx/internal/lexer"
)

func benchSource(n int) string {
	var b []byte
	for i := 0; i < n; i++ {
		b = append(b, "model m"...)
		b = append(b, byte('0'+i%10))
		b = append(b, "\n  name: text required\n  email: email unique\n  active: bool default true\n"...)
	}
	b = append(b, `page /dashboard requires auth
  query stats: SELECT count(*) as total FROM m0
  html
    <p>Total: {stats.total}</p>
`...)
	return string(b)
}

func BenchmarkParse100(b *testing.B) {
	src := benchSource(10)
	stripped := lexer.StripComments(src)
	tokens := lexer.Tokenize(stripped)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Parse(tokens, stripped)
	}
}

func BenchmarkParse1k(b *testing.B) {
	src := benchSource(50)
	stripped := lexer.StripComments(src)
	tokens := lexer.Tokenize(stripped)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Parse(tokens, stripped)
	}
}
