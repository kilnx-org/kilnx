package lexer

import (
	"strings"
	"testing"
)

func TestTokenKeyword(t *testing.T) {
	allKeywords := []string{
		"page", "action", "fragment", "stream", "socket", "api",
		"webhook", "schedule", "job", "model", "config", "auth",
		"permissions", "layout", "query", "validate",
		"redirect", "on", "limit", "log", "test", "translations",
		"enqueue", "broadcast", "send", "requires", "method",
		"title",
	}

	for _, kw := range allKeywords {
		tokens := Tokenize(kw)
		found := false
		for _, tok := range tokens {
			if tok.Type == TokenKeyword && tok.Value == kw {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%q should tokenize as TokenKeyword", kw)
		}
	}
}

func TestTokenIdentifier(t *testing.T) {
	tests := []string{"foo", "myvar", "user_name", "hello", "abc123", "my-var"}

	for _, id := range tests {
		tokens := Tokenize(id)
		found := false
		for _, tok := range tokens {
			if tok.Type == TokenIdentifier && tok.Value == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%q should tokenize as TokenIdentifier", id)
		}
	}
}

func TestTokenString(t *testing.T) {
	tokens := Tokenize(`"Hello World"`)
	found := false
	for _, tok := range tokens {
		if tok.Type == TokenString && tok.Value == "Hello World" {
			found = true
			break
		}
	}
	if !found {
		t.Error(`"Hello World" should tokenize as TokenString with value "Hello World"`)
	}
}

func TestTokenStringEmpty(t *testing.T) {
	tokens := Tokenize(`""`)
	found := false
	for _, tok := range tokens {
		if tok.Type == TokenString && tok.Value == "" {
			found = true
			break
		}
	}
	if !found {
		t.Error(`"" should tokenize as TokenString with empty value`)
	}
}

func TestTokenNumberInteger(t *testing.T) {
	tokens := Tokenize("42")
	found := false
	for _, tok := range tokens {
		if tok.Type == TokenNumber && tok.Value == "42" {
			found = true
			break
		}
	}
	if !found {
		t.Error("42 should tokenize as TokenNumber")
	}
}

func TestTokenNumberFloat(t *testing.T) {
	tokens := Tokenize("3.14")
	found := false
	for _, tok := range tokens {
		if tok.Type == TokenNumber && tok.Value == "3.14" {
			found = true
			break
		}
	}
	if !found {
		t.Error("3.14 should tokenize as TokenNumber")
	}
}

func TestTokenPath(t *testing.T) {
	tokens := Tokenize("/users/create")
	found := false
	for _, tok := range tokens {
		if tok.Type == TokenPath && tok.Value == "/users/create" {
			found = true
			break
		}
	}
	if !found {
		t.Error("/users/create should tokenize as TokenPath")
	}
}

func TestTokenColon(t *testing.T) {
	tokens := Tokenize("name:")
	found := false
	for _, tok := range tokens {
		if tok.Type == TokenColon && tok.Value == ":" {
			found = true
			break
		}
	}
	if !found {
		t.Error(": should tokenize as TokenColon")
	}
}

func TestTokenComma(t *testing.T) {
	tokens := Tokenize("a, b")
	found := false
	for _, tok := range tokens {
		if tok.Type == TokenComma && tok.Value == "," {
			found = true
			break
		}
	}
	if !found {
		t.Error(", should tokenize as TokenComma")
	}
}

func TestTokenBrackets(t *testing.T) {
	tokens := Tokenize("[admin, editor]")
	var hasOpen, hasClose bool
	for _, tok := range tokens {
		if tok.Type == TokenBracketOpen {
			hasOpen = true
		}
		if tok.Type == TokenBracketClose {
			hasClose = true
		}
	}
	if !hasOpen {
		t.Error("[ should tokenize as TokenBracketOpen")
	}
	if !hasClose {
		t.Error("] should tokenize as TokenBracketClose")
	}
}

func TestTokenNewline(t *testing.T) {
	tokens := Tokenize("page /\nfragment /card")
	newlineCount := 0
	for _, tok := range tokens {
		if tok.Type == TokenNewline {
			newlineCount++
		}
	}
	if newlineCount < 2 {
		t.Errorf("expected at least 2 newline tokens, got %d", newlineCount)
	}
}

func TestTokenEOF(t *testing.T) {
	tokens := Tokenize("page /")
	last := tokens[len(tokens)-1]
	if last.Type != TokenEOF {
		t.Error("last token should be TokenEOF")
	}
}

func TestIndentEmit(t *testing.T) {
	src := "page /\n  \"Hello\""
	tokens := Tokenize(src)
	found := false
	for _, tok := range tokens {
		if tok.Type == TokenIndent {
			found = true
			break
		}
	}
	if !found {
		t.Error("increasing indent should emit TokenIndent")
	}
}

func TestDedentEmit(t *testing.T) {
	src := "page /\n  \"Hello\"\nfragment /card"
	tokens := Tokenize(src)
	found := false
	for _, tok := range tokens {
		if tok.Type == TokenDedent {
			found = true
			break
		}
	}
	if !found {
		t.Error("decreasing indent should emit TokenDedent")
	}
}

func TestMultipleDedent(t *testing.T) {
	src := "page /\n  query: SELECT\n    name\nfragment /card"
	tokens := Tokenize(src)
	dedentCount := 0
	for _, tok := range tokens {
		if tok.Type == TokenDedent {
			dedentCount++
		}
	}
	if dedentCount < 2 {
		t.Errorf("expected at least 2 dedents for double indent reduction, got %d", dedentCount)
	}
}

func TestIndentStackBehavior(t *testing.T) {
	src := "a\n  b\n    c\n  d\ne"
	tokens := Tokenize(src)

	indents := 0
	dedents := 0
	for _, tok := range tokens {
		if tok.Type == TokenIndent {
			indents++
		}
		if tok.Type == TokenDedent {
			dedents++
		}
	}

	if indents != dedents {
		t.Errorf("indent count (%d) should equal dedent count (%d)", indents, dedents)
	}
}

func TestEmptyInput(t *testing.T) {
	tokens := Tokenize("")
	// Should produce only EOF
	if len(tokens) != 1 {
		t.Errorf("empty input should produce only EOF, got %d tokens", len(tokens))
	}
	if tokens[0].Type != TokenEOF {
		t.Error("empty input should produce TokenEOF")
	}
}

func TestBlankLines(t *testing.T) {
	tokens := Tokenize("\n\n\n")
	// Blank lines are skipped, so only EOF
	if len(tokens) != 1 {
		t.Errorf("blank lines should produce only EOF, got %d tokens", len(tokens))
	}
}

func TestMinimalPage(t *testing.T) {
	src := "page /\n  \"Hello World\""
	tokens := Tokenize(src)

	expected := []struct {
		typ TokenType
		val string
	}{
		{TokenKeyword, "page"},
		{TokenPath, "/"},
		{TokenNewline, "\n"},
		{TokenIndent, ""},
		{TokenString, "Hello World"},
		{TokenNewline, "\n"},
		{TokenDedent, ""},
		{TokenEOF, ""},
	}

	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d\ntokens: %v", len(expected), len(tokens), tokens)
	}

	for i, exp := range expected {
		if tokens[i].Type != exp.typ {
			t.Errorf("token[%d]: expected type %d, got %d (value=%q)", i, exp.typ, tokens[i].Type, tokens[i].Value)
		}
		if tokens[i].Value != exp.val {
			t.Errorf("token[%d]: expected value %q, got %q", i, exp.val, tokens[i].Value)
		}
	}
}

func TestLineNumbers(t *testing.T) {
	src := "page /\n  \"Hello\""
	tokens := Tokenize(src)

	for _, tok := range tokens {
		if tok.Type == TokenKeyword && tok.Value == "page" {
			if tok.Line != 1 {
				t.Errorf("page keyword should be on line 1, got %d", tok.Line)
			}
		}
		if tok.Type == TokenString && tok.Value == "Hello" {
			if tok.Line != 2 {
				t.Errorf("string should be on line 2, got %d", tok.Line)
			}
		}
	}
}

func TestFieldTypeRecognition(t *testing.T) {
	fieldTypes := []string{
		"text", "email", "bool", "timestamp", "richtext",
		"option", "int", "float", "password", "image", "phone",
	}

	for _, ft := range fieldTypes {
		if !IsFieldType(ft) {
			t.Errorf("IsFieldType(%q) should return true", ft)
		}
	}

	if IsFieldType("nonexistent") {
		t.Error("IsFieldType(\"nonexistent\") should return false")
	}
}

func TestFieldConstraintRecognition(t *testing.T) {
	constraints := []string{"required", "unique", "default", "auto", "min", "max"}

	for _, c := range constraints {
		if !IsFieldConstraint(c) {
			t.Errorf("IsFieldConstraint(%q) should return true", c)
		}
	}

	if IsFieldConstraint("nonexistent") {
		t.Error("IsFieldConstraint(\"nonexistent\") should return false")
	}
}

func TestModelTokenization(t *testing.T) {
	src := "model user\n  name: text required\n  email: email unique"
	tokens := Tokenize(src)

	// Verify we get model keyword, user identifier, indent, field tokens
	types := make([]TokenType, 0)
	for _, tok := range tokens {
		types = append(types, tok.Type)
	}

	// First token must be keyword "model"
	if tokens[0].Type != TokenKeyword || tokens[0].Value != "model" {
		t.Errorf("first token should be keyword 'model', got type=%d value=%q", tokens[0].Type, tokens[0].Value)
	}

	// Second token must be identifier "user"
	if tokens[1].Type != TokenIdentifier || tokens[1].Value != "user" {
		t.Errorf("second token should be identifier 'user', got type=%d value=%q", tokens[1].Type, tokens[1].Value)
	}
}

func TestTabIndent(t *testing.T) {
	src := "page /\n\t\"Hello\""
	tokens := Tokenize(src)
	found := false
	for _, tok := range tokens {
		if tok.Type == TokenIndent {
			found = true
			break
		}
	}
	if !found {
		t.Error("tab indentation should emit TokenIndent")
	}
}

func TestPathWithParams(t *testing.T) {
	tokens := Tokenize("/users/:id/edit")
	found := false
	for _, tok := range tokens {
		if tok.Type == TokenPath && tok.Value == "/users/:id/edit" {
			found = true
			break
		}
	}
	if !found {
		t.Error("/users/:id/edit should tokenize as a single TokenPath")
	}
}

func TestMultipleTokensOnLine(t *testing.T) {
	src := "role: option [admin, editor, viewer] default viewer"
	tokens := Tokenize(src)

	// Should contain: identifier(role), colon, identifier(option), bracket_open,
	// identifiers and commas, bracket_close, identifier(default), identifier(viewer)
	hasOpen := false
	hasClose := false
	hasComma := false
	hasColon := false
	for _, tok := range tokens {
		switch tok.Type {
		case TokenBracketOpen:
			hasOpen = true
		case TokenBracketClose:
			hasClose = true
		case TokenComma:
			hasComma = true
		case TokenColon:
			hasColon = true
		}
	}

	if !hasColon {
		t.Error("should have TokenColon")
	}
	if !hasOpen {
		t.Error("should have TokenBracketOpen")
	}
	if !hasClose {
		t.Error("should have TokenBracketClose")
	}
	if !hasComma {
		t.Error("should have TokenComma")
	}
}

// --- Comment tests ---

func TestStripComments_FullLineComment(t *testing.T) {
	src := "page /\n# this is a comment\n  \"Hello\""
	result := StripComments(src)
	if strings.Contains(result, "this is a comment") {
		t.Error("full-line comment should be stripped")
	}
	// Line should be blank to preserve line numbers
	lines := strings.Split(result, "\n")
	if lines[1] != "" {
		t.Errorf("comment line should be empty, got %q", lines[1])
	}
}

func TestStripComments_InlineComment(t *testing.T) {
	src := `page / # serves the home page`
	result := StripComments(src)
	if strings.Contains(result, "serves the home page") {
		t.Error("inline comment should be stripped")
	}
	if !strings.Contains(result, "page /") {
		t.Error("code before comment should be preserved")
	}
}

func TestStripComments_HashInsideString(t *testing.T) {
	src := `"color: #ff0000"`
	result := StripComments(src)
	if !strings.Contains(result, "#ff0000") {
		t.Error("# inside string should NOT be treated as comment")
	}
}

func TestStripComments_PreservesLineNumbers(t *testing.T) {
	src := "line1\n# comment\nline3"
	result := StripComments(src)
	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Errorf("should preserve 3 lines, got %d", len(lines))
	}
	if lines[2] != "line3" {
		t.Errorf("line 3 should be 'line3', got %q", lines[2])
	}
}

func TestStripComments_NoComments(t *testing.T) {
	src := "page /\n  \"Hello World\""
	result := StripComments(src)
	if result != src {
		t.Errorf("source without comments should be unchanged")
	}
}

func TestCommentedLineNotTokenized(t *testing.T) {
	src := StripComments("page /\n# model user\nfragment /card")
	tokens := Tokenize(src)

	for _, tok := range tokens {
		if tok.Value == "model" {
			t.Error("commented out 'model' keyword should not be tokenized")
		}
	}
}

func TestStripCommentsPreservesHexColorsInHTML(t *testing.T) {
	source := `layout main
  html
    <style>
      body { background: #09090b; color: #fafafa; }
      a { color: #ff6e40; }
    </style>
    <div class="test">#not-a-comment</div>`

	result := StripComments(source)

	if !strings.Contains(result, "#09090b") {
		t.Error("hex color #09090b inside html block was stripped")
	}
	if !strings.Contains(result, "#fafafa") {
		t.Error("hex color #fafafa inside html block was stripped")
	}
	if !strings.Contains(result, "#ff6e40") {
		t.Error("hex color #ff6e40 inside html block was stripped")
	}
	if !strings.Contains(result, "#not-a-comment") {
		t.Error("text with # inside html block was stripped")
	}
}

func TestStripCommentsStillWorksOutsideHTML(t *testing.T) {
	source := `page /
  html
    <style>body { color: #fafafa; }</style>

# this is a comment
page /about
  "About" # inline comment`

	result := StripComments(source)

	if !strings.Contains(result, "#fafafa") {
		t.Error("hex color inside html block was stripped")
	}
	if strings.Contains(result, "this is a comment") {
		t.Error("full-line comment outside html should be stripped")
	}
	if strings.Contains(result, "inline comment") {
		t.Error("inline comment outside html should be stripped")
	}
}

func TestStripCommentsNestedHTMLBlock(t *testing.T) {
	source := `page /test
  html
    <div>#hash-in-html</div>
page /other # comment here`

	result := StripComments(source)

	if !strings.Contains(result, "#hash-in-html") {
		t.Error("hash inside html block was stripped")
	}
	if strings.Contains(result, "comment here") {
		t.Error("inline comment after html block should be stripped")
	}
}
