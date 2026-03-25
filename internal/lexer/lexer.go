package lexer

import (
	"strings"
	"unicode"
)

type TokenType int

const (
	TokenKeyword    TokenType = iota // page, fragment, action, etc.
	TokenPath                        // /users, /about
	TokenString                      // "Hello World"
	TokenIdentifier                  // variable names, method names
	TokenNewline                     // end of line
	TokenIndent                      // increase in indentation
	TokenDedent                      // decrease in indentation
	TokenEOF                         // end of file
)

type Token struct {
	Type    TokenType
	Value   string
	Line    int
	Column  int
}

var keywords = map[string]bool{
	"page": true, "action": true, "fragment": true,
	"stream": true, "socket": true, "api": true,
	"webhook": true, "schedule": true, "job": true,
	"model": true, "config": true, "auth": true,
	"permissions": true, "layout": true, "component": true,
	"query": true, "queries": true, "validate": true,
	"search": true, "paginate": true, "form": true,
	"redirect": true, "on": true, "limit": true,
	"log": true, "test": true, "translations": true,
	"enqueue": true, "broadcast": true,
	"send": true, "requires": true, "method": true,
	"title": true, "with": true,
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
