package parser

import (
	"fmt"
	"strings"

	"github.com/kilnx-org/kilnx/internal/lexer"
)

type App struct {
	Models  []Model
	Pages   []Page
	Actions []Page // actions share the same structure as pages but handle POST/PUT/DELETE
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
	NodeText  NodeType = iota
	NodeQuery          // query users: select name, email from user
	NodeList           // list users { title: name, subtitle: email }
	NodeTable          // table users { columns: name, email }
	NodeAlert          // alert success "message"
	NodeForm           // form user
	NodeRedirect       // redirect /users
	NodeValidate       // validate { name: required, email: required }
)

type Node struct {
	Type        NodeType
	Value       string
	Name        string            // for query: result var name; for list/table: query name
	SQL         string            // for query: the raw SQL
	Props       map[string]string // for list: title, subtitle; for alert: level
	Columns     []TableColumn     // for table: column definitions
	RowActions  []RowAction       // for table: row-level actions
	Paginate    int               // for query: items per page (0 = no pagination)
	ModelName   string            // for form: which model to generate form from
	QuerySQL    string            // for form with query: pre-fill data
	Validations []Validation      // for validate block
}

type Validation struct {
	Field string
	Rules []string // required, is email, min N, max N
}

type TableColumn struct {
	Field string // column field name from query result
	Label string // display label (optional, defaults to field name)
}

type RowAction struct {
	Label string // e.g., "edit", "delete", "view"
	Path  string // e.g., /users/:id/edit
}

func Parse(tokens []lexer.Token, source string) (*App, error) {
	lines := strings.Split(source, "\n")
	p := &parserState{tokens: tokens, pos: 0, lines: lines}
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
		case "action":
			action, err := p.parseAction()
			if err != nil {
				return nil, err
			}
			app.Actions = append(app.Actions, action)
		default:
			p.advance()
		}
	}

	return app, nil
}

type parserState struct {
	tokens []lexer.Token
	pos    int
	lines  []string // original source lines for raw text extraction
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

		if tok.Type == lexer.TokenKeyword {
			switch tok.Value {
			case "query":
				node, err := p.parseQueryNode()
				if err == nil {
					nodes = append(nodes, node)
				}
				continue
			case "list":
				node := p.parseListNode()
				nodes = append(nodes, node)
				continue
			case "table":
				node := p.parseTableNode()
				nodes = append(nodes, node)
				continue
			case "alert":
				node := p.parseAlertNode()
				nodes = append(nodes, node)
				continue
			case "form":
				node := p.parseFormNode()
				nodes = append(nodes, node)
				continue
			case "redirect":
				node := p.parseRedirectNode()
				nodes = append(nodes, node)
				continue
			case "validate":
				node := p.parseValidateNode()
				nodes = append(nodes, node)
				continue
			}
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

// parseQueryNode parses: query name: SELECT ... (rest of line and continuation lines)
// Extracts SQL directly from source lines to preserve special chars like (), *, etc.
func (p *parserState) parseQueryNode() (Node, error) {
	node := Node{Type: NodeQuery}

	queryLine := p.current().Line

	// consume "query"
	p.advance()

	// query name (or inline: "query: SELECT ...")
	if p.current().Type == lexer.TokenColon {
		// unnamed query: query: SELECT ...
		p.advance() // consume ':'
		node.SQL = p.extractSQLFromLine(queryLine)
		p.skipToEndOfLine()
		node.SQL += p.extractContinuationSQL()
		node.SQL, node.Paginate = extractPaginate(node.SQL)
		return node, nil
	}

	// named query: query users: SELECT ...
	if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
		node.Name = p.advance().Value
	}

	// expect colon
	if p.current().Type == lexer.TokenColon {
		p.advance()
	}

	node.SQL = p.extractSQLFromLine(queryLine)
	p.skipToEndOfLine()
	node.SQL += p.extractContinuationSQL()

	// Check for "paginate N" suffix in SQL
	node.SQL, node.Paginate = extractPaginate(node.SQL)

	return node, nil
}

// extractSQLFromLine extracts the SQL portion from a source line.
// For "  query stats: SELECT count(*) as total FROM user",
// it returns "SELECT count(*) as total FROM user"
func (p *parserState) extractSQLFromLine(lineNum int) string {
	if lineNum < 1 || lineNum > len(p.lines) {
		return ""
	}
	line := p.lines[lineNum-1]

	// Find the colon after "query" or "query name"
	idx := strings.Index(line, ":")
	if idx < 0 {
		return strings.TrimSpace(line)
	}
	return strings.TrimSpace(line[idx+1:])
}

// skipToEndOfLine advances past all tokens on the current line
func (p *parserState) skipToEndOfLine() {
	for !p.isEOF() && p.current().Type != lexer.TokenNewline && p.current().Type != lexer.TokenDedent {
		p.advance()
	}
}

// extractContinuationSQL collects indented continuation lines for multi-line SQL
func (p *parserState) extractContinuationSQL() string {
	var sqlParts []string

	// Skip the newline
	if p.current().Type == lexer.TokenNewline {
		p.advance()
	}

	// Check for indented continuation
	if p.current().Type == lexer.TokenIndent {
		p.advance()
		for !p.isEOF() {
			if p.current().Type == lexer.TokenDedent {
				p.advance()
				break
			}
			if p.current().Type == lexer.TokenNewline {
				p.advance()
				continue
			}
			// Get the raw line
			lineNum := p.current().Line
			if lineNum >= 1 && lineNum <= len(p.lines) {
				rawLine := strings.TrimSpace(p.lines[lineNum-1])
				if rawLine != "" {
					sqlParts = append(sqlParts, rawLine)
				}
			}
			// Skip all tokens on this line
			currentLine := p.current().Line
			for !p.isEOF() && p.current().Line == currentLine &&
				p.current().Type != lexer.TokenNewline &&
				p.current().Type != lexer.TokenDedent {
				p.advance()
			}
		}
	}

	if len(sqlParts) == 0 {
		return ""
	}
	return " " + strings.Join(sqlParts, " ")
}

// extractPaginate checks for "paginate N" at the end of SQL and strips it.
// Returns the cleaned SQL and the page size (0 if no pagination).
func extractPaginate(sql string) (string, int) {
	trimmed := strings.TrimSpace(sql)
	lower := strings.ToLower(trimmed)

	idx := strings.LastIndex(lower, "paginate ")
	if idx < 0 {
		return sql, 0
	}

	after := strings.TrimSpace(trimmed[idx+len("paginate "):])
	n := 0
	fmt.Sscanf(after, "%d", &n)
	if n <= 0 {
		return sql, 0
	}

	return strings.TrimSpace(trimmed[:idx]), n
}

// parseListNode parses:
//
//	list queryName
//	  title: fieldName
//	  subtitle: fieldName
func (p *parserState) parseListNode() Node {
	node := Node{Type: NodeList, Props: make(map[string]string)}

	// consume "list"
	p.advance()

	// query name to iterate over
	if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
		node.Name = p.advance().Value
	}

	// skip to newline
	p.skipToEndOfLine()
	p.skipNewlines()

	// parse indented props
	if p.current().Type == lexer.TokenIndent {
		p.advance()
		for !p.isEOF() {
			if p.current().Type == lexer.TokenDedent {
				p.advance()
				break
			}
			if p.current().Type == lexer.TokenNewline {
				p.advance()
				continue
			}

			// prop: value
			if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
				key := p.advance().Value
				if p.current().Type == lexer.TokenColon {
					p.advance()
				}
				if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
					node.Props[key] = p.advance().Value
				} else if p.current().Type == lexer.TokenString {
					node.Props[key] = p.advance().Value
				}
				p.skipToEndOfLine()
			} else {
				p.advance()
			}
		}
	}

	return node
}

// parseTableNode parses:
//
//	table queryName
//	  columns: name, email, created
//	  row action: edit /users/:id/edit
//	  row action: delete /users/:id/delete
func (p *parserState) parseTableNode() Node {
	node := Node{Type: NodeTable, Props: make(map[string]string)}

	// consume "table"
	p.advance()

	// query name
	if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
		node.Name = p.advance().Value
	}

	p.skipToEndOfLine()
	p.skipNewlines()

	// parse indented block
	if p.current().Type == lexer.TokenIndent {
		p.advance()
		for !p.isEOF() {
			if p.current().Type == lexer.TokenDedent {
				p.advance()
				break
			}
			if p.current().Type == lexer.TokenNewline {
				p.advance()
				continue
			}

			tok := p.current()

			// columns: name, email as "Email", created
			if (tok.Type == lexer.TokenIdentifier || tok.Type == lexer.TokenKeyword) && tok.Value == "columns" {
				p.advance()
				if p.current().Type == lexer.TokenColon {
					p.advance()
				}
				node.Columns = p.parseColumnList()
				continue
			}

			// row action: label /path
			if (tok.Type == lexer.TokenIdentifier || tok.Type == lexer.TokenKeyword) && tok.Value == "row" {
				p.advance()
				// expect "action" (can be keyword or identifier)
				if (p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword) && p.current().Value == "action" {
					p.advance()
				}
				if p.current().Type == lexer.TokenColon {
					p.advance()
				}
				action := RowAction{}
				if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
					action.Label = p.advance().Value
				}
				if p.current().Type == lexer.TokenPath {
					action.Path = p.advance().Value
				}
				node.RowActions = append(node.RowActions, action)
				p.skipToEndOfLine()
				continue
			}

			p.advance()
		}
	}

	return node
}

// parseColumnList parses: name, email as "Email Address", created
func (p *parserState) parseColumnList() []TableColumn {
	var cols []TableColumn

	for !p.isEOF() && p.current().Type != lexer.TokenNewline && p.current().Type != lexer.TokenDedent {
		if p.current().Type == lexer.TokenComma {
			p.advance()
			continue
		}

		if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
			col := TableColumn{Field: p.advance().Value}

			// Check for "as" alias
			if p.current().Type == lexer.TokenIdentifier && p.current().Value == "as" {
				p.advance()
				if p.current().Type == lexer.TokenString {
					col.Label = p.advance().Value
				}
			}

			cols = append(cols, col)
		} else {
			p.advance()
		}
	}

	return cols
}

// parseAlertNode parses: alert success "message"
func (p *parserState) parseAlertNode() Node {
	node := Node{Type: NodeAlert, Props: make(map[string]string)}

	// consume "alert"
	p.advance()

	// level: success, error, warning, info
	if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
		node.Props["level"] = p.advance().Value
	}

	// message
	if p.current().Type == lexer.TokenString {
		node.Value = p.advance().Value
	}

	p.skipToEndOfLine()
	return node
}

// parseAction parses:
//
//	action /users/create method POST
//	  validate user
//	  query: INSERT INTO user (name, email) VALUES (:name, :email)
//	  redirect /users
func (p *parserState) parseAction() (Page, error) {
	page := Page{Method: "POST"}

	// consume "action"
	p.advance()

	// expect path
	if p.current().Type != lexer.TokenPath {
		return page, fmt.Errorf("line %d: expected path after 'action'", p.current().Line)
	}
	page.Path = p.advance().Value

	// parse modifiers: method, requires
	for !p.isEOF() && p.current().Type != lexer.TokenNewline {
		tok := p.current()
		if tok.Type == lexer.TokenKeyword || tok.Type == lexer.TokenIdentifier {
			switch tok.Value {
			case "method":
				p.advance()
				if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
					page.Method = p.advance().Value
				}
			case "requires":
				p.advance()
				page.Auth = true
				if p.current().Type == lexer.TokenIdentifier {
					p.advance()
				}
			default:
				p.advance()
			}
		} else {
			p.advance()
		}
	}

	p.skipNewlines()

	// parse body
	if p.current().Type == lexer.TokenIndent {
		p.advance()
		page.Body = p.parseBody()
	}

	return page, nil
}

// parseFormNode parses:
//
//	form user
//	form user with query: SELECT * FROM user WHERE id = :id
func (p *parserState) parseFormNode() Node {
	node := Node{Type: NodeForm}

	formLine := p.current().Line

	// consume "form"
	p.advance()

	// model name
	if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
		node.ModelName = p.advance().Value
	}

	// optional "with query: SQL"
	if p.current().Type == lexer.TokenKeyword && p.current().Value == "with" {
		p.advance()
		if p.current().Type == lexer.TokenKeyword && p.current().Value == "query" {
			p.advance()
			if p.current().Type == lexer.TokenColon {
				p.advance()
			}
			node.QuerySQL = p.extractSQLFromLine(formLine)
			// extractSQLFromLine finds the first colon; we need the SQL after "with query:"
			// Re-extract: find "query:" in the line
			if formLine >= 1 && formLine <= len(p.lines) {
				line := p.lines[formLine-1]
				idx := strings.Index(line, "query:")
				if idx >= 0 {
					node.QuerySQL = strings.TrimSpace(line[idx+len("query:"):])
				}
			}
		}
	}

	p.skipToEndOfLine()
	return node
}

// parseValidateNode parses:
//
//	validate user
//	  (uses model constraints automatically)
//
// or:
//
//	validate
//	  name: required
//	  email: required, is email
func (p *parserState) parseValidateNode() Node {
	node := Node{Type: NodeValidate}

	// consume "validate"
	p.advance()

	// optional model name (validate user = use model constraints)
	if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
		node.ModelName = p.advance().Value
	}

	p.skipToEndOfLine()
	p.skipNewlines()

	// optional indented block with explicit rules
	if p.current().Type == lexer.TokenIndent {
		p.advance()
		for !p.isEOF() {
			if p.current().Type == lexer.TokenDedent {
				p.advance()
				break
			}
			if p.current().Type == lexer.TokenNewline {
				p.advance()
				continue
			}

			// field: rules
			if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
				v := Validation{Field: p.advance().Value}
				if p.current().Type == lexer.TokenColon {
					p.advance()
				}
				// collect rules until end of line
				var rules []string
				for !p.isEOF() && p.current().Type != lexer.TokenNewline && p.current().Type != lexer.TokenDedent {
					if p.current().Type == lexer.TokenComma {
						p.advance()
						continue
					}
					rules = append(rules, p.advance().Value)
				}
				v.Rules = rules
				node.Validations = append(node.Validations, v)
			} else {
				p.advance()
			}
		}
	}

	return node
}

// parseRedirectNode parses: redirect /users
func (p *parserState) parseRedirectNode() Node {
	node := Node{Type: NodeRedirect}

	// consume "redirect"
	p.advance()

	// path
	if p.current().Type == lexer.TokenPath {
		node.Value = p.advance().Value
	}

	p.skipToEndOfLine()
	return node
}
