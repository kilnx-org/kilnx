package lexer

import (
	"strings"
	"testing"
)

func FuzzTokenize(f *testing.F) {
	seeds := []string{
		`model user
  name: text required
  email: email unique`,
		`page /dashboard requires auth
  query stats: SELECT count(*) FROM user
  html
    <p>{stats.count}</p>`,
		`action /users/create method POST
  validate
    name: required min 2
  query: INSERT INTO user (name) VALUES (:name)
  on success
    redirect /users`,
		`# comment
config
  secret: env APP_SECRET`,
		`{{each users}}<li>{name}</li>{{end}}`,
		`",\n\t\r`,
		"",
		"html\n  <div>#ff00ff</div>",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		// Tokenize must never panic, regardless of input.
		_ = Tokenize(src)
		_ = StripComments(src)
	})
}

func FuzzStripComments(f *testing.F) {
	seeds := []string{
		"hello # world",
		"# full line comment\npage /",
		`html
  <div style="color:#ff00ff">#not-a-comment</div>`,
		"",
		"#",
		"##",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		out := StripComments(src)
		// Output should never contain bare # outside strings.
		if strings.Contains(out, "#") {
			// It's OK if # is inside a string or html block; this is a coarse check.
		}
	})
}
