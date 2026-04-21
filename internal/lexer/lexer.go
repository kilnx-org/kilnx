package lexer

import (
	"strings"
	"unicode"
)

type TokenType int

const (
	TokenKeyword      TokenType = iota // page, fragment, action, etc.
	TokenPath                          // /users, /about
	TokenString                        // "Hello World"
	TokenIdentifier                    // variable names, method names
	TokenNumber                        // 2, 100, 8080
	TokenColon                         // :
	TokenComma                         // ,
	TokenBracketOpen                   // [
	TokenBracketClose                  // ]
	TokenParenOpen                     // (
	TokenParenClose                    // )
	TokenNewline                       // end of line
	TokenIndent                        // increase in indentation
	TokenDedent                        // decrease in indentation
	TokenRawLine                       // raw text of a full line (for SQL capture)
	TokenEOF                           // end of file
)

type Token struct {
	Type   TokenType
	Value  string
	Line   int
	Column int
}

var keywords = map[string]bool{
	"page": true, "action": true, "fragment": true,
	"stream": true, "socket": true, "api": true,
	"webhook": true, "schedule": true, "job": true,
	"model": true, "config": true, "auth": true,
	"permissions": true, "layout": true,
	"query": true, "validate": true,
	"redirect": true, "on": true, "limit": true,
	"log": true, "test": true, "translations": true,
	"enqueue": true, "broadcast": true,
	"send": true, "requires": true, "method": true, "fetch": true,
	"title": true,
}

// Field type keywords recognized inside model blocks
var fieldTypes = map[string]bool{
	"text": true, "email": true, "bool": true,
	"timestamp": true, "richtext": true, "option": true,
	"int": true, "float": true, "password": true,
	"image": true, "phone": true,
	"reference": true, "date": true, "url": true,
	"decimal": true, "file": true, "tags": true,
	"json": true, "uuid": true, "bigint": true,
}

// Field constraint keywords
var fieldConstraints = map[string]bool{
	"required": true, "unique": true,
	"default": true, "auto": true,
	"min": true, "max": true,
	"auto_update": true,
}

func IsFieldType(s string) bool {
	return fieldTypes[s]
}

func IsFieldConstraint(s string) bool {
	return fieldConstraints[s]
}

// StripComments removes # comments from source code.
// Full-line comments (lines starting with #) are replaced with blank lines to preserve line numbers.
// Inline comments (# after code) are stripped, respecting quoted strings.
// Lines inside html blocks are never stripped (they may contain CSS hex colors like #fff).
func StripComments(source string) string {
	lines := strings.Split(source, "\n")
	inHTML := false
	htmlIndent := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		indent := 0
		for _, ch := range line {
			if ch == ' ' {
				indent++
			} else if ch == '\t' {
				indent += 2
			} else {
				break
			}
		}

		// Detect entering an html block: a line whose trimmed content is "html"
		if !inHTML && trimmed == "html" {
			inHTML = true
			htmlIndent = indent
			continue
		}

		// Inside html block: skip comment stripping until dedent
		if inHTML {
			if indent <= htmlIndent && trimmed != "" {
				inHTML = false
				// Fall through to normal comment processing for this line
			} else {
				continue
			}
		}

		if trimmed[0] == '#' {
			lines[i] = ""
			continue
		}
		// Strip inline comment, but respect quoted strings
		inQuote := false
		runes := []rune(line)
		for j, ch := range runes {
			if ch == '"' {
				inQuote = !inQuote
			}
			if !inQuote && ch == '#' {
				lines[i] = string(runes[:j])
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}

func Tokenize(source string) []Token {
	var tokens []Token
	lines := strings.Split(source, "\n")
	indentStack := []int{0}

	for lineNum, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		indent := countIndent(line)
		trimmed := strings.TrimSpace(line)

		// Handle indentation changes
		currentIndent := indentStack[len(indentStack)-1]
		if indent > currentIndent {
			indentStack = append(indentStack, indent)
			tokens = append(tokens, Token{Type: TokenIndent, Value: "", Line: lineNum + 1})
		} else if indent < currentIndent {
			for len(indentStack) > 1 && indentStack[len(indentStack)-1] > indent {
				indentStack = indentStack[:len(indentStack)-1]
				tokens = append(tokens, Token{Type: TokenDedent, Value: "", Line: lineNum + 1})
			}
		}

		// Tokenize the line content
		tokens = append(tokens, tokenizeLine(trimmed, lineNum+1)...)
		tokens = append(tokens, Token{Type: TokenNewline, Value: "\n", Line: lineNum + 1})
	}

	// Close remaining indents
	for len(indentStack) > 1 {
		indentStack = indentStack[:len(indentStack)-1]
		tokens = append(tokens, Token{Type: TokenDedent, Value: "", Line: len(lines)})
	}

	tokens = append(tokens, Token{Type: TokenEOF, Value: "", Line: len(lines)})
	return tokens
}

func tokenizeLine(line string, lineNum int) []Token {
	var tokens []Token
	i := 0

	for i < len(line) {
		// Skip whitespace
		if line[i] == ' ' || line[i] == '\t' {
			i++
			continue
		}

		// Single character tokens
		switch line[i] {
		case ':':
			tokens = append(tokens, Token{Type: TokenColon, Value: ":", Line: lineNum, Column: i})
			i++
			continue
		case ',':
			tokens = append(tokens, Token{Type: TokenComma, Value: ",", Line: lineNum, Column: i})
			i++
			continue
		case '[':
			tokens = append(tokens, Token{Type: TokenBracketOpen, Value: "[", Line: lineNum, Column: i})
			i++
			continue
		case ']':
			tokens = append(tokens, Token{Type: TokenBracketClose, Value: "]", Line: lineNum, Column: i})
			i++
			continue
		case '(':
			tokens = append(tokens, Token{Type: TokenParenOpen, Value: "(", Line: lineNum, Column: i})
			i++
			continue
		case ')':
			tokens = append(tokens, Token{Type: TokenParenClose, Value: ")", Line: lineNum, Column: i})
			i++
			continue
		}

		// String literal
		if line[i] == '"' {
			j := i + 1
			for j < len(line) && line[j] != '"' {
				j++
			}
			value := line[i+1 : j]
			tokens = append(tokens, Token{Type: TokenString, Value: value, Line: lineNum, Column: i})
			if j < len(line) {
				j++ // skip closing quote
			}
			i = j
			continue
		}

		// Path (starts with /)
		if line[i] == '/' {
			j := i
			for j < len(line) && line[j] != ' ' && line[j] != '\t' {
				j++
			}
			tokens = append(tokens, Token{Type: TokenPath, Value: line[i:j], Line: lineNum, Column: i})
			i = j
			continue
		}

		// Number
		if unicode.IsDigit(rune(line[i])) {
			j := i
			for j < len(line) && (unicode.IsDigit(rune(line[j])) || line[j] == '.') {
				j++
			}
			tokens = append(tokens, Token{Type: TokenNumber, Value: line[i:j], Line: lineNum, Column: i})
			i = j
			continue
		}

		// Word (keyword or identifier)
		if unicode.IsLetter(rune(line[i])) || line[i] == '_' {
			j := i
			for j < len(line) && (unicode.IsLetter(rune(line[j])) || unicode.IsDigit(rune(line[j])) || line[j] == '_' || line[j] == '-') {
				j++
			}
			word := line[i:j]
			if keywords[word] {
				tokens = append(tokens, Token{Type: TokenKeyword, Value: word, Line: lineNum, Column: i})
			} else {
				tokens = append(tokens, Token{Type: TokenIdentifier, Value: word, Line: lineNum, Column: i})
			}
			i = j
			continue
		}

		// Skip other characters for now
		i++
	}

	return tokens
}

func countIndent(line string) int {
	count := 0
	for _, ch := range line {
		if ch == ' ' {
			count++
		} else if ch == '\t' {
			count += 2
		} else {
			break
		}
	}
	return count
}
