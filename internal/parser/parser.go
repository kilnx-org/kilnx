package parser

import (
	"fmt"

	"github.com/kilnx-org/kilnx/internal/lexer"
)

type App struct {
	Pages []Page
}

type Page struct {
	Path    string
	Layout  string
	Title   string
	Auth    bool
	Method  string
	Body    []Node
}

type NodeType int

const (
	NodeText NodeType = iota
)

type Node struct {
	Type  NodeType
	Value string
}

func Parse(tokens []lexer.Token) (*App, error) {
	p := &parserState{tokens: tokens, pos: 0}
	app := &App{}

	for !p.isEOF() {
		p.skipNewlines()
		if p.isEOF() {
			break
		}

		tok := p.current()
		if tok.Type == lexer.TokenKeyword && tok.Value == "page" {
			page, err := p.parsePage()
			if err != nil {
				return nil, err
			}
			app.Pages = append(app.Pages, page)
		} else {
			p.advance()
		}
	}

	return app, nil
}

type parserState struct {
	tokens []lexer.Token
	pos    int
}

func (p *parserState) current() lexer.Token {
	if p.pos >= len(p.tokens) {
		return lexer.Token{Type: lexer.TokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *parserState) advance() lexer.Token {
	tok := p.current()
	p.pos++
	return tok
}

func (p *parserState) isEOF() bool {
	return p.pos >= len(p.tokens) || p.current().Type == lexer.TokenEOF
}

func (p *parserState) skipNewlines() {
	for !p.isEOF() && p.current().Type == lexer.TokenNewline {
		p.advance()
	}
}

func (p *parserState) parsePage() (Page, error) {
	page := Page{Method: "GET"}

	// consume "page"
	p.advance()

	// expect path
	if p.current().Type != lexer.TokenPath {
		return page, fmt.Errorf("line %d: expected path after 'page', got '%s'", p.current().Line, p.current().Value)
	}
	page.Path = p.advance().Value

	// parse optional modifiers on the same line: layout, title, requires, method
	for !p.isEOF() && p.current().Type != lexer.TokenNewline {
		tok := p.current()
		if tok.Type == lexer.TokenKeyword {
			switch tok.Value {
			case "layout":
				p.advance()
				if p.current().Type == lexer.TokenIdentifier {
					page.Layout = p.advance().Value
				}
			case "title":
				p.advance()
				if p.current().Type == lexer.TokenString {
					page.Title = p.advance().Value
				}
			case "requires":
				p.advance()
				page.Auth = true
				if p.current().Type == lexer.TokenIdentifier {
					p.advance() // consume the role name (auth, admin, etc.)
				}
			case "method":
				p.advance()
				if p.current().Type == lexer.TokenIdentifier {
					page.Method = p.advance().Value
				}
			default:
				p.advance()
			}
		} else {
			p.advance()
		}
	}

	// skip newline
	p.skipNewlines()

	// parse body (indented block)
	if p.current().Type == lexer.TokenIndent {
		p.advance() // consume indent
		page.Body = p.parseBody()
	}

	return page, nil
}

func (p *parserState) parseBody() []Node {
	var nodes []Node

	for !p.isEOF() {
		tok := p.current()

		if tok.Type == lexer.TokenDedent {
			p.advance()
			break
		}

		if tok.Type == lexer.TokenNewline {
			p.advance()
			continue
		}

		if tok.Type == lexer.TokenString {
			nodes = append(nodes, Node{Type: NodeText, Value: tok.Value})
			p.advance()
			continue
		}

		// Skip tokens we don't handle yet
		p.advance()
	}

	return nodes
}
