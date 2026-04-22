package lexer

import (
	"strings"
	"testing"
)

// buildLargeSource generates a .kilnx source with n models.
func buildLargeSource(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString("model m")
		b.WriteString(string(rune('0' + i%10)))
		b.WriteString("\n  name: text required\n  email: email unique\n  active: bool default true\n")
	}
	b.WriteString(`page /dashboard requires auth
  query stats: SELECT count(*) as total FROM m0
  html
    <p>Total: {stats.total}</p>
`)
	return b.String()
}

func BenchmarkTokenize1k(b *testing.B) {
	src := buildLargeSource(50)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Tokenize(src)
	}
}

func BenchmarkTokenize10k(b *testing.B) {
	src := buildLargeSource(500)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Tokenize(src)
	}
}

func BenchmarkStripComments1k(b *testing.B) {
	src := buildLargeSource(50)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = StripComments(src)
	}
}
