package parser

import (
	"fmt"

	"github.com/kilnx-org/kilnx/internal/lexer"
)

type App struct {
	Models []Model
	Pages  []Page
}

type Model struct {
	Name   string
	Fields []Field
}

type FieldType string

const (
	FieldText      FieldType = "text"
	FieldEmail     FieldType = "email"
	FieldBool      FieldType = "bool"
	FieldTimestamp  FieldType = "timestamp"
	FieldRichtext  FieldType = "richtext"
	FieldOption    FieldType = "option"
	FieldInt       FieldType = "int"
	FieldFloat     FieldType = "float"
	FieldPassword  FieldType = "password"
	FieldImage     FieldType = "image"
	FieldPhone     FieldType = "phone"
	FieldReference FieldType = "reference"
)

type Field struct {
	Name       string
	Type       FieldType
	Required   bool
	Unique     bool
	Optional   bool
	Default    string
	Auto       bool
	Min        string
	Max        string
	Options    []string   // for option type: [admin, editor, viewer]
	Reference  string     // for reference type: model name
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
		if tok.Type != lexer.TokenKeyword {
			p.advance()
			continue
		}

		switch tok.Value {
		case "model":
			model, err := p.parseModel(app)
			if err != nil {
				return nil, err
			}
			app.Models = append(app.Models, model)
		case "page":
			page, err := p.parsePage()
			if err != nil {
				return nil, err
			}
			app.Pages = append(app.Pages, page)
		default:
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

// parseModel parses:
//   model user
//     name: text required min 2 max 100
//     email: email unique
//     role: option [admin, editor, viewer] default viewer
//     active: bool default true
//     created: timestamp auto
//     author: user required        (reference to another model)
func (p *parserState) parseModel(app *App) (Model, error) {
	model := Model{}

	// consume "model"
	p.advance()

	// expect model name
	if p.current().Type != lexer.TokenIdentifier {
		return model, fmt.Errorf("line %d: expected model name after 'model'", p.current().Line)
	}
	model.Name = p.advance().Value

	// skip newline
	p.skipNewlines()

	// parse fields (indented block)
	if p.current().Type != lexer.TokenIndent {
		return model, fmt.Errorf("line %d: expected indented fields for model '%s'", p.current().Line, model.Name)
	}
	p.advance() // consume indent

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

		// Field names can be keywords (e.g., "title", "query") when used inside a model
		if tok.Type == lexer.TokenIdentifier || tok.Type == lexer.TokenKeyword {
			field, err := p.parseField(app)
			if err != nil {
				return model, err
			}
			model.Fields = append(model.Fields, field)
			continue
		}

		p.advance()
	}

	return model, nil
}

func (p *parserState) parseField(app *App) (Field, error) {
	field := Field{}

	// field name (can be a keyword like "title" or "query")
	field.Name = p.advance().Value

	// expect colon
	if p.current().Type != lexer.TokenColon {
		return field, fmt.Errorf("line %d: expected ':' after field name '%s'", p.current().Line, field.Name)
	}
	p.advance() // consume ':'

	// field type or reference (can also be a keyword like "text" or an identifier)
	if p.current().Type != lexer.TokenIdentifier && p.current().Type != lexer.TokenKeyword {
		return field, fmt.Errorf("line %d: expected type after '%s:'", p.current().Line, field.Name)
	}

	typeName := p.advance().Value

	if lexer.IsFieldType(typeName) {
		field.Type = FieldType(typeName)
	} else {
		// Check if it's a reference to another model
		field.Type = FieldReference
		field.Reference = typeName
	}

	// Parse options list for option type: [admin, editor, viewer]
	if field.Type == FieldOption && p.current().Type == lexer.TokenBracketOpen {
		p.advance() // consume '['
		for !p.isEOF() && p.current().Type != lexer.TokenBracketClose {
			if p.current().Type == lexer.TokenComma {
				p.advance()
				continue
			}
			if p.current().Type == lexer.TokenIdentifier {
				field.Options = append(field.Options, p.advance().Value)
			} else {
				p.advance()
			}
		}
		if p.current().Type == lexer.TokenBracketClose {
			p.advance() // consume ']'
		}
	}

	// Parse constraints: required, unique, optional, default X, auto, min N, max N
	for !p.isEOF() && p.current().Type != lexer.TokenNewline && p.current().Type != lexer.TokenDedent {
		tok := p.current()

		if tok.Type == lexer.TokenComma {
			p.advance()
			continue
		}

		if tok.Type == lexer.TokenIdentifier || tok.Type == lexer.TokenKeyword {
			switch tok.Value {
			case "required":
				field.Required = true
				p.advance()
			case "unique":
				field.Unique = true
				p.advance()
			case "optional":
				field.Optional = true
				p.advance()
			case "auto":
				field.Auto = true
				p.advance()
			case "default":
				p.advance()
				if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenString || p.current().Type == lexer.TokenNumber {
					field.Default = p.advance().Value
				}
			case "min":
				p.advance()
				if p.current().Type == lexer.TokenNumber {
					field.Min = p.advance().Value
				}
			case "max":
				p.advance()
				if p.current().Type == lexer.TokenNumber || p.current().Type == lexer.TokenIdentifier {
					field.Max = p.advance().Value
				}
			default:
				p.advance()
			}
		} else {
			p.advance()
		}
	}

	return field, nil
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

	// parse optional modifiers on the same line
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
					p.advance()
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
		p.advance()
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

		p.advance()
	}

	return nodes
}
