package parser

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/kilnx-org/kilnx/internal/lexer"
)

type App struct {
	Models       []Model
	Pages        []Page
	Actions      []Page       // actions share the same structure as pages but handle POST/PUT/DELETE
	Fragments    []Page       // fragments return partial HTML (no page wrapper)
	APIs         []Page       // api endpoints return JSON instead of HTML
	Streams      []Stream     // SSE stream endpoints
	Schedules    []Schedule   // timed tasks
	Jobs         []Job        // async background jobs
	Webhooks     []Webhook    // external event receivers
	Sockets      []Socket     // bidirectional websockets
	RateLimits   []RateLimit  // rate limiting rules
	Config       *AppConfig   // nil if no config block defined
	Auth         *AuthConfig  // nil if no auth block defined
	Permissions  []Permission // role-based access rules
	Layouts      []Layout
	Tests        []Test
	LogConfig    *LogConfig
	Translations map[string]map[string]string // lang -> key -> value
	NamedQueries map[string]string            // name -> SQL
}

type Test struct {
	Name  string
	Steps []TestStep
}

type TestStep struct {
	Action string // "visit", "fill", "submit", "expect", "as"
	Target string // field name, URL, or selector
	Value  string // value to fill or expected value
}

type AppConfig struct {
	Name            string   // app display name for the topbar
	Database        string   // env var or default path
	Port            int      // server port (default 8080)
	Secret          string   // env var for session secret
	StaticDir       string   // static files directory (served at /_static/)
	UploadsDir      string   // upload directory
	UploadsMaxMB    int      // max upload size in MB
	DefaultLanguage string   // default i18n language
	DetectLanguage  string   // "header accept-language" or empty
	CORSOrigins     []string // allowed CORS origins (empty = same-origin only)
}

type LogConfig struct {
	Level       string // "debug", "info", "warn", "error"
	SlowQueryMs int    // log queries slower than this (ms)
	LogRequests bool
	LogErrors   bool
	Stacktrace  bool // log errors with stacktrace
}

type Webhook struct {
	Path      string
	SecretEnv string         // env var name for signature verification
	Events    []WebhookEvent // on event handlers
}

type WebhookEvent struct {
	Name string // e.g., "payment_intent.succeeded"
	Body []Node // actions to execute
}

type Socket struct {
	Path         string
	Auth         bool
	RequiresRole string
	OnConnect    []Node
	OnMessage    []Node
	OnDisconnect []Node
}

type RateLimit struct {
	PathPattern string // e.g., "/api/*", "/login"
	Requests    int    // max requests
	Window      string // "minute", "hour"
	Per         string // "user", "ip"
	DelaySecs   int    // cooldown on exceeded
	Message     string // custom message on exceeded
}

type Schedule struct {
	Name         string
	IntervalSecs int    // parsed from "every 24h", "every 1m", etc.
	Cron         string // raw cron expression like "every monday at 9:00"
	Body         []Node // query, send email, etc.
}

type Job struct {
	Name       string
	Body       []Node // query, send email, etc.
	MaxRetries int    // from "retry N" declaration (default 3)
}

type Stream struct {
	Path         string
	Auth         bool
	RequiresRole string
	SQL          string // query to execute on each tick
	IntervalSecs int    // polling interval in seconds
	EventName    string // SSE event name (default: "message")
}

type Permission struct {
	Role  string   // e.g., "admin", "editor", "viewer"
	Rules []string // e.g., "all", "read post", "write post where author = current_user"
}

type Layout struct {
	Name        string
	HTMLContent string // raw HTML with {page.title}, {page.content}, {nav}
	Queries     []Node // queries to execute when rendering the layout
}

type AuthConfig struct {
	Table        string // user table name (default: "user")
	Identity     string // identity field (default: "email")
	Password     string // password field (default: "password")
	LoginPath    string // login page path (default: "/login")
	LogoutPath   string // logout POST path (default: "/logout")
	RegisterPath string // registration page path (default: "/register")
	ForgotPath   string // forgot-password page path (default: "/forgot-password")
	ResetPath    string // reset-password page path (default: "/reset-password")
	AfterLogin   string // redirect after login (default: "/")
}

type Model struct {
	Name string
	// Tenant references another model. Rows of this model are scoped to
	// that tenant. The compiler auto-synthesizes a required reference
	// field named after the tenant model (e.g. `tenant: org` adds an
	// `org_id` column) and the runtime injects a WHERE filter on SELECT
	// queries against this table.
	Tenant string
	Fields []Field
}

type FieldType string

const (
	FieldText      FieldType = "text"
	FieldEmail     FieldType = "email"
	FieldBool      FieldType = "bool"
	FieldTimestamp FieldType = "timestamp"
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
	Name      string
	Type      FieldType
	Required  bool
	Unique    bool
	Default   string
	Auto      bool
	Min       string
	Max       string
	Options   []string // for option type: [admin, editor, viewer]
	Reference string   // for reference type: model name
}

type Page struct {
	Path         string
	Layout       string
	Title        string
	Auth         bool
	RequiresRole string // "auth" = any logged in, "admin"/"editor" = specific role
	Method       string
	Body         []Node
}

type NodeType int

const (
	NodeText        NodeType = iota
	NodeQuery                // query users: select name, email from user
	NodeRedirect             // redirect /users
	NodeValidate             // validate { name: required, email: required }
	NodeRespond              // respond fragment ".selector" with query: SQL
	NodeHTML                 // html { raw html content }
	NodeSendEmail            // send email to :email { subject: "...", body: "..." }
	NodeEnqueue              // enqueue job-name with param: value
	NodeOn                   // on success/error/not found branching
	NodeBroadcast            // broadcast to :room
	NodeGeneratePDF          // generate pdf from template X with data Y
	NodeFetch                // fetch data: GET https://api.example.com/endpoint
)

type Node struct {
	Type          NodeType
	Value         string
	Name          string            // for query: result var name
	SQL           string            // for query: the raw SQL
	Props         map[string]string // for on: condition; for send email: body
	Paginate      int               // for query: items per page (0 = no pagination)
	ModelName     string            // for validate: which model to validate against
	QuerySQL      string            // for validate with query: pre-fill data
	Validations   []Validation      // for validate block
	RespondTarget string            // for respond: CSS selector target
	RespondSwap   string            // for respond: htmx swap strategy
	HTMLContent   string            // for html: raw HTML content
	EmailTo       string            // for send email: recipient (:email or query result)
	EmailSubject  string            // for send email: subject line
	EmailTemplate string            // for send email: template name
	JobName       string            // for enqueue: which job to run
	JobParams     map[string]string // for enqueue: params to pass to job
	Children      []Node            // for on: child nodes to execute
	StatusCode    int               // for respond: HTTP status code
	BroadcastRoom string            // for broadcast: room name
	BroadcastFrag string            // for broadcast: fragment reference
	TemplateName  string            // for generate pdf: template name
	DataQueryName string            // for generate pdf: data query name
	EmailAttach   string            // for send email: attachment file path or param
	FetchURL      string            // for fetch: the URL to request
	FetchMethod   string            // for fetch: GET, POST, PUT, DELETE
	FetchHeaders  map[string]string // for fetch: request headers
	FetchBody     map[string]string // for fetch: POST body params
}

type Validation struct {
	Field string
	Rules []string // required, is email, min N, max N
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
				p.addError(err)
				p.synchronize()
				continue
			}
			app.Models = append(app.Models, model)
		case "page":
			page, err := p.parsePage()
			if err != nil {
				p.addError(err)
				p.synchronize()
				continue
			}
			app.Pages = append(app.Pages, page)
		case "action":
			action, err := p.parseAction()
			if err != nil {
				p.addError(err)
				p.synchronize()
				continue
			}
			app.Actions = append(app.Actions, action)
		case "fragment":
			frag, err := p.parseFragment()
			if err != nil {
				p.addError(err)
				p.synchronize()
				continue
			}
			app.Fragments = append(app.Fragments, frag)
		case "api":
			apiEndpoint, err := p.parseAPI()
			if err != nil {
				p.addError(err)
				p.synchronize()
				continue
			}
			app.APIs = append(app.APIs, apiEndpoint)
		case "stream":
			stream, err := p.parseStream()
			if err != nil {
				p.addError(err)
				p.synchronize()
				continue
			}
			app.Streams = append(app.Streams, stream)
		case "schedule":
			sched := p.parseSchedule()
			app.Schedules = append(app.Schedules, sched)
		case "job":
			job := p.parseJob()
			app.Jobs = append(app.Jobs, job)
		case "webhook":
			wh := p.parseWebhook()
			app.Webhooks = append(app.Webhooks, wh)
		case "socket":
			sock := p.parseSocket()
			app.Sockets = append(app.Sockets, sock)
		case "limit":
			rl := p.parseRateLimit()
			app.RateLimits = append(app.RateLimits, rl)
		case "config":
			cfg := p.parseConfig()
			app.Config = &cfg
		case "auth":
			authCfg, err := p.parseAuth()
			if err != nil {
				p.addError(err)
				p.synchronize()
				continue
			}
			app.Auth = &authCfg
		case "permissions":
			perms := p.parsePermissions()
			app.Permissions = append(app.Permissions, perms...)
		case "layout":
			layout := p.parseLayout()
			app.Layouts = append(app.Layouts, layout)
		case "test":
			t := p.parseTest()
			app.Tests = append(app.Tests, t)
		case "log":
			cfg := p.parseLogConfig()
			app.LogConfig = &cfg
		case "translations":
			trans := p.parseTranslations()
			if app.Translations == nil {
				app.Translations = make(map[string]map[string]string)
			}
			for lang, entries := range trans {
				if app.Translations[lang] == nil {
					app.Translations[lang] = make(map[string]string)
				}
				for k, v := range entries {
					app.Translations[lang][k] = v
				}
			}
		case "queries":
			nq := p.parseNamedQueries()
			if app.NamedQueries == nil {
				app.NamedQueries = make(map[string]string)
			}
			for k, v := range nq {
				app.NamedQueries[k] = v
			}
		default:
			p.advance()
		}
	}

	// Resolve named query references in page/action/fragment/api bodies
	if len(app.NamedQueries) > 0 {
		resolveNamedQueries := func(nodes []Node) {
			for i := range nodes {
				if nodes[i].Type == NodeQuery && nodes[i].SQL != "" {
					trimmed := strings.TrimSpace(nodes[i].SQL)
					if sql, ok := app.NamedQueries[trimmed]; ok {
						nodes[i].SQL = sql
					}
				}
			}
		}
		for i := range app.Pages {
			resolveNamedQueries(app.Pages[i].Body)
		}
		for i := range app.Actions {
			resolveNamedQueries(app.Actions[i].Body)
		}
		for i := range app.Fragments {
			resolveNamedQueries(app.Fragments[i].Body)
		}
		for i := range app.APIs {
			resolveNamedQueries(app.APIs[i].Body)
		}
		for i := range app.Schedules {
			resolveNamedQueries(app.Schedules[i].Body)
		}
		for i := range app.Jobs {
			resolveNamedQueries(app.Jobs[i].Body)
		}
		for i := range app.Webhooks {
			for j := range app.Webhooks[i].Events {
				resolveNamedQueries(app.Webhooks[i].Events[j].Body)
			}
		}
		for i := range app.Sockets {
			resolveNamedQueries(app.Sockets[i].OnConnect)
			resolveNamedQueries(app.Sockets[i].OnMessage)
			resolveNamedQueries(app.Sockets[i].OnDisconnect)
		}
	}

	if len(p.errors) > 0 {
		return app, multiError(p.errors)
	}
	return app, nil
}

type parserState struct {
	tokens   []lexer.Token
	pos      int
	lines    []string // original source lines for raw text extraction
	errors   []error  // collected parse errors for multi-error reporting
	recovery bool     // true when skipping tokens to synchronize
}

// addError records a parse error and enters recovery mode
func (p *parserState) addError(err error) {
	p.errors = append(p.errors, err)
	p.recovery = true
}

// synchronize advances to the next top-level keyword (indent level 0).
// This allows the parser to recover from errors and continue parsing.
func (p *parserState) synchronize() {
	depth := 0
	for !p.isEOF() {
		tok := p.current()
		if tok.Type == lexer.TokenIndent {
			depth++
			p.advance()
			continue
		}
		if tok.Type == lexer.TokenDedent {
			depth--
			if depth < 0 {
				depth = 0
			}
			p.advance()
			continue
		}
		if tok.Type == lexer.TokenNewline {
			p.advance()
			continue
		}
		// A keyword at depth 0 is a synchronization point
		if depth == 0 && tok.Type == lexer.TokenKeyword {
			p.recovery = false
			return
		}
		p.advance()
	}
	p.recovery = false
}

// multiError joins multiple errors into a single error
type multiError []error

func (me multiError) Error() string {
	var msgs []string
	for _, e := range me {
		msgs = append(msgs, e.Error())
	}
	return strings.Join(msgs, "\n")
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
//
//	model user
//	  name: text required min 2 max 100
//	  email: email unique
//	  role: option [admin, editor, viewer] default viewer
//	  active: bool default true
//	  created: timestamp auto
//	  author: user required        (reference to another model)
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

		// tenant: <model> is a model-level meta directive, not a field.
		// Must appear before any field declaration.
		if (tok.Type == lexer.TokenIdentifier || tok.Type == lexer.TokenKeyword) &&
			tok.Value == "tenant" && p.peekIsTenantDirective() {
			tenantModel, err := p.parseTenantDirective()
			if err != nil {
				return model, err
			}
			if model.Tenant != "" {
				return model, fmt.Errorf("line %d: model '%s' already has a tenant directive", tok.Line, model.Name)
			}
			if len(model.Fields) > 0 {
				return model, fmt.Errorf("line %d: tenant directive must appear before field declarations in model '%s'", tok.Line, model.Name)
			}
			model.Tenant = tenantModel
			// Auto-synthesize a required reference field so the schema
			// generates the <tenant>_id foreign key column.
			model.Fields = append(model.Fields, Field{
				Name:      tenantModel,
				Type:      FieldReference,
				Reference: tenantModel,
				Required:  true,
			})
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

// peekIsTenantDirective returns true if the tokens at the current position
// match the pattern `tenant : <identifier>` (the meta directive), so we can
// distinguish it from a regular field named "tenant".
func (p *parserState) peekIsTenantDirective() bool {
	if p.pos+2 >= len(p.tokens) {
		return false
	}
	if p.tokens[p.pos+1].Type != lexer.TokenColon {
		return false
	}
	third := p.tokens[p.pos+2]
	if third.Type != lexer.TokenIdentifier && third.Type != lexer.TokenKeyword {
		return false
	}
	// A regular field like `tenant: text required` would have a built-in
	// field type here. Only treat as directive when the third token is a
	// model-name identifier (not a built-in type).
	return !lexer.IsFieldType(third.Value)
}

// parseTenantDirective parses `tenant: <model>` and returns the referenced
// model name. Caller has already verified with peekIsTenantDirective.
func (p *parserState) parseTenantDirective() (string, error) {
	p.advance() // consume 'tenant'
	p.advance() // consume ':'
	name := p.advance().Value
	// Consume to end of line (trailing garbage ignored, keeps grammar forgiving).
	for !p.isEOF() && p.current().Type != lexer.TokenNewline && p.current().Type != lexer.TokenDedent {
		p.advance()
	}
	return name, nil
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
				titleTok := p.advance()
				if p.current().Type == lexer.TokenString {
					page.Title = p.advance().Value
				} else {
					page.Title = p.extractTitleFromSourceLine(titleTok.Line, titleTok.Column)
					p.skipToEndOfModifier()
				}
			case "requires":
				p.advance()
				page.Auth = true
				if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
					page.RequiresRole = p.advance().Value
				} else {
					page.RequiresRole = "auth"
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
			case "redirect":
				node := p.parseRedirectNode()
				nodes = append(nodes, node)
				continue
			case "validate":
				node := p.parseValidateNode()
				nodes = append(nodes, node)
				continue
			case "send":
				node := p.parseSendEmailNode()
				nodes = append(nodes, node)
				continue
			case "enqueue":
				node := p.parseEnqueueNode()
				nodes = append(nodes, node)
				continue
			case "on":
				node := p.parseOnNode()
				nodes = append(nodes, node)
				continue
			case "broadcast":
				node := p.parseBroadcastNode()
				nodes = append(nodes, node)
				continue
			case "fetch":
				node := p.parseFetchNode()
				nodes = append(nodes, node)
				continue
			}
		}

		// "respond", "html", "generate", "card", "modal", "chart" are identifiers, not keywords
		if tok.Type == lexer.TokenIdentifier {
			switch tok.Value {
			case "respond":
				node := p.parseRespondNode()
				nodes = append(nodes, node)
				continue
			case "html":
				node := p.parseHTMLNode()
				nodes = append(nodes, node)
				continue
			case "generate":
				node := p.parseGeneratePDFNode()
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

// extractTitleFromSourceLine extracts a dynamic title expression from the source line.
// For "page /docs/:slug title {doc.title} layout main", it returns "{doc.title}".
func (p *parserState) extractTitleFromSourceLine(lineNum, titleCol int) string {
	if lineNum < 1 || lineNum > len(p.lines) {
		return ""
	}
	line := p.lines[lineNum-1]

	// Find start: skip past "title " from the title keyword column
	start := titleCol + len("title")
	if start >= len(line) {
		return ""
	}
	// Skip whitespace after "title"
	for start < len(line) && line[start] == ' ' {
		start++
	}

	rest := line[start:]

	// Trim at known page modifier keywords that appear as standalone words
	modifiers := []string{" layout ", " requires ", " method "}
	for _, mod := range modifiers {
		if idx := strings.Index(rest, mod); idx >= 0 {
			rest = rest[:idx]
		}
	}

	return strings.TrimSpace(rest)
}

// skipToEndOfModifier advances past tokens until hitting a page modifier keyword or newline
func (p *parserState) skipToEndOfModifier() {
	for !p.isEOF() && p.current().Type != lexer.TokenNewline && p.current().Type != lexer.TokenDedent {
		if p.current().Type == lexer.TokenKeyword {
			v := p.current().Value
			if v == "layout" || v == "requires" || v == "method" {
				break
			}
		}
		p.advance()
	}
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
				if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
					page.RequiresRole = p.advance().Value
				} else {
					page.RequiresRole = "auth"
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

// parseFragment parses:
//
//	fragment /users/:id/card
//	  query user: SELECT name, email FROM user WHERE id = :id
//	  html
//	    <div class="card">{user.name}</div>
func (p *parserState) parseFragment() (Page, error) {
	page := Page{Method: "GET"}

	// consume "fragment"
	p.advance()

	// expect path
	if p.current().Type != lexer.TokenPath {
		return page, fmt.Errorf("line %d: expected path after 'fragment'", p.current().Line)
	}
	page.Path = p.advance().Value

	// parse optional modifiers
	for !p.isEOF() && p.current().Type != lexer.TokenNewline {
		tok := p.current()
		if (tok.Type == lexer.TokenKeyword || tok.Type == lexer.TokenIdentifier) && tok.Value == "requires" {
			p.advance()
			page.Auth = true
			if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
				page.RequiresRole = p.advance().Value
			} else {
				page.RequiresRole = "auth"
			}
		} else {
			p.advance()
		}
	}

	p.skipNewlines()

	if p.current().Type == lexer.TokenIndent {
		p.advance()
		page.Body = p.parseBody()
	}

	return page, nil
}

// parseRespondNode parses:
//
//	respond fragment ".user-row" with query: SELECT * FROM user WHERE id = :id
//	respond fragment delete
func (p *parserState) parseRespondNode() Node {
	node := Node{Type: NodeRespond}

	respondLine := p.current().Line

	// consume "respond"
	p.advance()

	// Check for "respond status N"
	if (p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword) && p.current().Value == "status" {
		p.advance()
		if p.current().Type == lexer.TokenNumber {
			fmt.Sscanf(p.advance().Value, "%d", &node.StatusCode)
		}
		p.skipToEndOfLine()
		return node
	}

	// expect "fragment"
	if (p.current().Type == lexer.TokenKeyword || p.current().Type == lexer.TokenIdentifier) && p.current().Value == "fragment" {
		p.advance()
	}

	// "delete" = empty response (htmx will remove the element)
	if p.current().Type == lexer.TokenIdentifier && p.current().Value == "delete" {
		node.RespondSwap = "delete"
		p.advance()
		p.skipToEndOfLine()
		return node
	}

	// target selector (string like ".user-row" or identifier)
	if p.current().Type == lexer.TokenString {
		node.RespondTarget = p.advance().Value
	} else if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
		node.RespondTarget = p.advance().Value
	}

	// "with query: SQL" or "with" followed by body
	if p.current().Type == lexer.TokenKeyword && p.current().Value == "with" {
		p.advance()
		if p.current().Type == lexer.TokenKeyword && p.current().Value == "query" {
			p.advance()
			if p.current().Type == lexer.TokenColon {
				p.advance()
			}
			// Extract SQL from the line
			if respondLine >= 1 && respondLine <= len(p.lines) {
				line := p.lines[respondLine-1]
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

// parseHTMLNode parses an html block with raw HTML content.
// The content is extracted from source lines to preserve all characters.
// Supports nested indentation within the html block.
//
//	html
//	  <div class="card">{user.name}</div>
func (p *parserState) parseHTMLNode() Node {
	node := Node{Type: NodeHTML}

	// consume "html"
	p.advance()

	p.skipToEndOfLine()
	p.skipNewlines()

	// Collect indented lines as raw HTML
	if p.current().Type == lexer.TokenIndent {
		p.advance()
		depth := 1 // track nested indent/dedent within the html block
		var htmlLines []string
		for !p.isEOF() {
			if p.current().Type == lexer.TokenIndent {
				depth++
				p.advance()
				continue
			}
			if p.current().Type == lexer.TokenDedent {
				depth--
				if depth <= 0 {
					p.advance()
					break
				}
				p.advance()
				continue
			}
			if p.current().Type == lexer.TokenNewline {
				p.advance()
				continue
			}
			// Get raw line from source
			lineNum := p.current().Line
			if lineNum >= 1 && lineNum <= len(p.lines) {
				htmlLines = append(htmlLines, strings.TrimSpace(p.lines[lineNum-1]))
			}
			// Skip all tokens on this line
			currentLine := p.current().Line
			for !p.isEOF() && p.current().Line == currentLine &&
				p.current().Type != lexer.TokenNewline &&
				p.current().Type != lexer.TokenDedent &&
				p.current().Type != lexer.TokenIndent {
				p.advance()
			}
		}
		node.HTMLContent = strings.Join(htmlLines, "\n")
	}

	return node
}

// parseAuth parses:
//
//	auth
//	  table: user
//	  identity: email
//	  password: password
//	  login: /login
//	  after login: /dashboard
func (p *parserState) parseAuth() (AuthConfig, error) {
	cfg := AuthConfig{
		Table:        "user",
		Identity:     "email",
		Password:     "password",
		LoginPath:    "/login",
		LogoutPath:   "/logout",
		RegisterPath: "/register",
		ForgotPath:   "/forgot-password",
		ResetPath:    "/reset-password",
		AfterLogin:   "/",
	}

	// consume "auth"
	p.advance()

	p.skipToEndOfLine()
	p.skipNewlines()

	if p.current().Type != lexer.TokenIndent {
		return cfg, nil
	}
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

		if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
			key := p.advance().Value

			// "after login" is two words
			if key == "after" && (p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword) && p.current().Value == "login" {
				p.advance()
				key = "after login"
			}

			if p.current().Type == lexer.TokenColon {
				p.advance()
			}

			var val string
			if p.current().Type == lexer.TokenPath {
				val = p.advance().Value
			} else if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
				val = p.advance().Value
			} else if p.current().Type == lexer.TokenString {
				val = p.advance().Value
			}

			switch key {
			case "table":
				cfg.Table = val
			case "identity":
				cfg.Identity = val
			case "password":
				cfg.Password = val
			case "login":
				cfg.LoginPath = val
			case "logout":
				cfg.LogoutPath = val
			case "register":
				cfg.RegisterPath = val
			case "forgot", "forgot_password", "forgot-password":
				cfg.ForgotPath = val
			case "reset", "reset_password", "reset-password":
				cfg.ResetPath = val
			case "after login":
				cfg.AfterLogin = val
			}

			p.skipToEndOfLine()
		} else {
			p.advance()
		}
	}

	return cfg, nil
}

// parseLayout parses:
//
//	layout main
//	  html
//	    <html>
//	    <head><title>{page.title}</title></head>
//	    <body>{nav}{page.content}</body>
//	    </html>
func (p *parserState) parseLayout() Layout {
	layout := Layout{}

	// consume "layout"
	p.advance()

	// layout name
	if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
		layout.Name = p.advance().Value
	}

	p.skipToEndOfLine()
	p.skipNewlines()

	// The layout body is an html block
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
			// Look for "html" keyword/identifier to start raw HTML capture
			if (p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword) && p.current().Value == "html" {
				node := p.parseHTMLNode()
				layout.HTMLContent = node.HTMLContent
				continue
			}
			// Parse query nodes inside layout
			if p.current().Type == lexer.TokenKeyword && p.current().Value == "query" {
				node, err := p.parseQueryNode()
				if err == nil {
					layout.Queries = append(layout.Queries, node)
				}
				continue
			}
			p.advance()
		}
	}

	return layout
}

// parsePermissions parses:
//
//	permissions
//	  admin: all
//	  editor: read post, write post where author = current_user
//	  viewer: read post where status = published
func (p *parserState) parsePermissions() []Permission {
	var perms []Permission

	// consume "permissions"
	p.advance()

	p.skipToEndOfLine()
	p.skipNewlines()

	if p.current().Type != lexer.TokenIndent {
		return perms
	}
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

		// role: rules
		if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
			lineNum := p.current().Line
			role := p.advance().Value

			if p.current().Type == lexer.TokenColon {
				p.advance()
			}

			// Extract the full rule text from the raw source line
			var ruleText string
			if lineNum >= 1 && lineNum <= len(p.lines) {
				line := p.lines[lineNum-1]
				idx := strings.Index(line, ":")
				if idx >= 0 {
					ruleText = strings.TrimSpace(line[idx+1:])
				}
			}

			var rules []string
			if ruleText != "" {
				// Split by comma to get individual rules
				for _, r := range strings.Split(ruleText, ",") {
					r = strings.TrimSpace(r)
					if r != "" {
						rules = append(rules, r)
					}
				}
			}

			perms = append(perms, Permission{Role: role, Rules: rules})
			p.skipToEndOfLine()
		} else {
			p.advance()
		}
	}

	return perms
}

// parseAPI parses:
//
//	api /api/v1/users requires auth
//	  query users: SELECT id, name, email FROM user ORDER BY id
//	  paginate 20
func (p *parserState) parseAPI() (Page, error) {
	page := Page{Method: "GET"}

	// consume "api"
	p.advance()

	// expect path
	if p.current().Type != lexer.TokenPath {
		return page, fmt.Errorf("line %d: expected path after 'api'", p.current().Line)
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
				if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
					page.RequiresRole = p.advance().Value
				} else {
					page.RequiresRole = "auth"
				}
			default:
				p.advance()
			}
		} else {
			p.advance()
		}
	}

	p.skipNewlines()

	if p.current().Type == lexer.TokenIndent {
		p.advance()
		page.Body = p.parseBody()
	}

	return page, nil
}

// parseStream parses:
//
//	stream /notifications requires auth
//	  query: SELECT message, created_at FROM notifications WHERE seen = false
//	  every 5s
func (p *parserState) parseStream() (Stream, error) {
	stream := Stream{
		IntervalSecs: 5,
		EventName:    "message",
	}

	// consume "stream"
	p.advance()

	// expect path
	if p.current().Type != lexer.TokenPath {
		return stream, fmt.Errorf("line %d: expected path after 'stream'", p.current().Line)
	}
	stream.Path = p.advance().Value

	// parse modifiers: requires
	for !p.isEOF() && p.current().Type != lexer.TokenNewline {
		tok := p.current()
		if (tok.Type == lexer.TokenKeyword || tok.Type == lexer.TokenIdentifier) && tok.Value == "requires" {
			p.advance()
			stream.Auth = true
			if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
				stream.RequiresRole = p.advance().Value
			} else {
				stream.RequiresRole = "auth"
			}
		} else {
			p.advance()
		}
	}

	p.skipNewlines()

	if p.current().Type != lexer.TokenIndent {
		return stream, nil
	}
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

		// query: SELECT ...
		if tok.Type == lexer.TokenKeyword && tok.Value == "query" {
			queryLine := tok.Line
			p.advance()
			if p.current().Type == lexer.TokenColon {
				p.advance()
			}
			stream.SQL = p.extractSQLFromLine(queryLine)
			p.skipToEndOfLine()
			cont := p.extractContinuationSQL()
			if cont != "" {
				stream.SQL += cont
			}
			continue
		}

		// every 5s / every 10s
		if (tok.Type == lexer.TokenIdentifier || tok.Type == lexer.TokenKeyword) && tok.Value == "every" {
			p.advance()
			stream.IntervalSecs = p.parseDurationTokens()
			p.skipToEndOfLine()
			continue
		}

		// event: name
		if (tok.Type == lexer.TokenIdentifier || tok.Type == lexer.TokenKeyword) && tok.Value == "event" {
			p.advance()
			if p.current().Type == lexer.TokenColon {
				p.advance()
			}
			if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
				stream.EventName = p.advance().Value
			}
			p.skipToEndOfLine()
			continue
		}

		p.advance()
	}

	return stream, nil
}

// parseDurationTokens reads duration from token stream.
// Handles both "5s" as single token and "5" + "s" as split tokens.
func (p *parserState) parseDurationTokens() int {
	if p.current().Type == lexer.TokenNumber {
		num := p.advance().Value
		// Check if next token is the unit suffix
		if p.current().Type == lexer.TokenIdentifier &&
			(p.current().Value == "s" || p.current().Value == "m" || p.current().Value == "h") {
			unit := p.advance().Value
			return parseDuration(num + unit)
		}
		return parseDuration(num)
	}
	if p.current().Type == lexer.TokenIdentifier {
		return parseDuration(p.advance().Value)
	}
	return 5
}

// parseDuration parses "5s", "10s", "1m", "5m" into seconds
func parseDuration(val string) int {
	n := 0
	unit := ""
	for i, c := range val {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			unit = val[i:]
			break
		}
	}
	if n == 0 {
		n = 5
	}
	switch unit {
	case "m":
		return n * 60
	case "h":
		return n * 3600
	default: // "s" or empty
		return n
	}
}

// parseSchedule parses:
//
//	schedule cleanup every 24h
//	  query: DELETE FROM session WHERE expires_at < now()
//
//	schedule report every monday at 9:00
//	  query stats: SELECT count(*) as new_users FROM user
//	  send email to admin@test.com
//	    subject: "Weekly report"
func (p *parserState) parseSchedule() Schedule {
	sched := Schedule{IntervalSecs: 3600}

	// consume "schedule"
	p.advance()

	// schedule name
	if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
		sched.Name = p.advance().Value
	}

	// "every" interval: "every 1h", "every 24h", "every 5m", "every 30s"
	// or cron: "every monday at 9:00"
	if (p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword) && p.current().Value == "every" {
		// Check if next token is a day name (cron expression)
		schedLine := p.current().Line
		p.advance()
		nextVal := ""
		if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
			nextVal = strings.ToLower(p.current().Value)
		}
		days := map[string]bool{
			"monday": true, "tuesday": true, "wednesday": true, "thursday": true,
			"friday": true, "saturday": true, "sunday": true,
		}
		if days[nextVal] {
			// It's a cron expression, extract from raw line
			if schedLine >= 1 && schedLine <= len(p.lines) {
				line := p.lines[schedLine-1]
				idx := strings.Index(strings.ToLower(line), "every ")
				if idx >= 0 {
					sched.Cron = strings.TrimSpace(line[idx:])
				}
			}
			p.skipToEndOfLine()
		} else {
			sched.IntervalSecs = p.parseDurationTokens()
		}
	}

	p.skipToEndOfLine()
	p.skipNewlines()

	if p.current().Type == lexer.TokenIndent {
		p.advance()
		sched.Body = p.parseBody()
	}

	return sched
}

// parseJob parses:
//
//	job generate-report
//	  query data: SELECT * FROM order WHERE created > :start_date
//	  send email to :requested_by
//	    subject: "Your report is ready"
func (p *parserState) parseJob() Job {
	job := Job{MaxRetries: 3}

	// consume "job"
	p.advance()

	// job name
	if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
		job.Name = p.advance().Value
	}

	p.skipToEndOfLine()
	p.skipNewlines()

	if p.current().Type == lexer.TokenIndent {
		p.advance()

		// Check for "retry N" as first line in job body
		if p.current().Type == lexer.TokenIdentifier && p.current().Value == "retry" {
			p.advance()
			if p.current().Type == lexer.TokenNumber {
				if n, err := strconv.Atoi(p.advance().Value); err == nil && n > 0 {
					job.MaxRetries = n
				}
			}
			p.skipToEndOfLine()
			p.skipNewlines()
		}

		job.Body = p.parseBody()
	}

	return job
}

// parseSendEmailNode parses:
//
//	send email to :email
//	  subject: "Welcome"
//	  template: welcome
//
// or inline: send email to admin@test.com subject "Report ready"
func (p *parserState) parseSendEmailNode() Node {
	node := Node{Type: NodeSendEmail, Props: make(map[string]string)}

	// consume "send"
	p.advance()

	// expect "email"
	if (p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword) && p.current().Value == "email" {
		p.advance()
	}

	// expect "to"
	if p.current().Type == lexer.TokenIdentifier && p.current().Value == "to" {
		p.advance()
	}

	// recipient: query: SQL, :param, identifier, or string
	if (p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword) && p.current().Value == "query" {
		// "send email to query: SELECT email FROM ..."
		queryLine := p.current().Line
		p.advance() // consume "query"
		if p.current().Type == lexer.TokenColon {
			p.advance() // consume ":"
		}
		// Extract SQL from the source line (may be multi-line)
		if queryLine >= 1 && queryLine <= len(p.lines) {
			line := p.lines[queryLine-1]
			idx := strings.Index(line, "query:")
			if idx >= 0 {
				sql := strings.TrimSpace(line[idx+len("query:"):])
				// Collect continuation lines
				for queryLine < len(p.lines) {
					nextLine := p.lines[queryLine]
					trimmed := strings.TrimSpace(nextLine)
					if trimmed == "" || (!strings.HasPrefix(nextLine, " ") && !strings.HasPrefix(nextLine, "\t")) {
						break
					}
					// Stop if next line starts a new keyword block (subject:, template:, etc.)
					if strings.Contains(trimmed, ":") && !strings.HasPrefix(strings.ToUpper(trimmed), "SELECT") &&
						!strings.HasPrefix(strings.ToUpper(trimmed), "WHERE") &&
						!strings.HasPrefix(strings.ToUpper(trimmed), "AND") &&
						!strings.HasPrefix(strings.ToUpper(trimmed), "OR") &&
						!strings.HasPrefix(strings.ToUpper(trimmed), "JOIN") &&
						!strings.HasPrefix(strings.ToUpper(trimmed), "LEFT") &&
						!strings.HasPrefix(strings.ToUpper(trimmed), "ORDER") &&
						!strings.HasPrefix(strings.ToUpper(trimmed), "GROUP") {
						break
					}
					sql += " " + trimmed
					queryLine++
				}
				node.Props["to_query"] = sql
			}
		}
	} else if p.current().Type == lexer.TokenColon {
		p.advance() // consume ':'
		if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
			node.EmailTo = ":" + p.advance().Value
		}
	} else if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
		node.EmailTo = p.advance().Value
	} else if p.current().Type == lexer.TokenString {
		node.EmailTo = p.advance().Value
	}

	p.skipToEndOfLine()
	p.skipNewlines()

	// Parse indented block: subject, template, body
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

			if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
				key := p.advance().Value
				if p.current().Type == lexer.TokenColon {
					p.advance()
				}
				if p.current().Type == lexer.TokenString {
					node.Props[key] = p.advance().Value
				} else if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
					node.Props[key] = p.advance().Value
				}
				p.skipToEndOfLine()
			} else {
				p.advance()
			}
		}
	}

	node.EmailSubject = node.Props["subject"]
	node.EmailTemplate = node.Props["template"]
	node.EmailAttach = node.Props["attach"]

	return node
}

// parseEnqueueNode parses:
//
//	enqueue generate-report with
//	  start_date: :start_date
//	  requested_by: :current_user_email
func (p *parserState) parseEnqueueNode() Node {
	node := Node{Type: NodeEnqueue, JobParams: make(map[string]string)}

	// consume "enqueue"
	p.advance()

	// job name
	if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
		node.JobName = p.advance().Value
	}

	// optional "with"
	if p.current().Type == lexer.TokenKeyword && p.current().Value == "with" {
		p.advance()
	}

	p.skipToEndOfLine()
	p.skipNewlines()

	// Parse indented params
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

			if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
				key := p.advance().Value
				if p.current().Type == lexer.TokenColon {
					p.advance()
				}
				var val string
				if p.current().Type == lexer.TokenColon {
					p.advance()
					if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
						val = ":" + p.advance().Value
					}
				} else if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
					val = p.advance().Value
				} else if p.current().Type == lexer.TokenString {
					val = p.advance().Value
				}
				node.JobParams[key] = val
				p.skipToEndOfLine()
			} else {
				p.advance()
			}
		}
	}

	return node
}

// parseWebhook parses:
//
//	webhook /stripe/payment secret env STRIPE_SECRET
//	  on event payment_intent.succeeded
//	    query: UPDATE order SET status = 'paid' WHERE stripe_id = :event_id
func (p *parserState) parseWebhook() Webhook {
	wh := Webhook{}

	// consume "webhook"
	p.advance()

	if p.current().Type == lexer.TokenPath {
		wh.Path = p.advance().Value
	}

	// "secret env VAR_NAME"
	for !p.isEOF() && p.current().Type != lexer.TokenNewline {
		if (p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword) && p.current().Value == "secret" {
			p.advance()
			if p.current().Type == lexer.TokenIdentifier && p.current().Value == "env" {
				p.advance()
			}
			if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
				wh.SecretEnv = p.advance().Value
			}
		} else {
			p.advance()
		}
	}

	p.skipNewlines()

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

			// "on event eventName"
			if p.current().Type == lexer.TokenKeyword && p.current().Value == "on" {
				p.advance()
				if p.current().Type == lexer.TokenIdentifier && p.current().Value == "event" {
					p.advance()
				}
				event := WebhookEvent{}
				eventLine := p.current().Line
				if eventLine >= 1 && eventLine <= len(p.lines) {
					line := p.lines[eventLine-1]
					idx := strings.Index(strings.ToLower(line), "event ")
					if idx >= 0 {
						event.Name = strings.TrimSpace(line[idx+len("event "):])
					}
				}
				p.skipToEndOfLine()
				p.skipNewlines()

				if p.current().Type == lexer.TokenIndent {
					p.advance()
					event.Body = p.parseBody()
				}

				wh.Events = append(wh.Events, event)
				continue
			}
			p.advance()
		}
	}

	return wh
}

// parseSocket parses:
//
//	socket /chat/:room requires auth
//	  on connect
//	    query: SELECT ...
//	  on message
//	    query: INSERT ...
func (p *parserState) parseSocket() Socket {
	sock := Socket{}

	p.advance()

	if p.current().Type == lexer.TokenPath {
		sock.Path = p.advance().Value
	}

	for !p.isEOF() && p.current().Type != lexer.TokenNewline {
		if (p.current().Type == lexer.TokenKeyword || p.current().Type == lexer.TokenIdentifier) && p.current().Value == "requires" {
			p.advance()
			sock.Auth = true
			if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
				sock.RequiresRole = p.advance().Value
			} else {
				sock.RequiresRole = "auth"
			}
		} else {
			p.advance()
		}
	}

	p.skipNewlines()

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

			if p.current().Type == lexer.TokenKeyword && p.current().Value == "on" {
				p.advance()
				eventType := ""
				if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
					eventType = p.advance().Value
				}
				p.skipToEndOfLine()
				p.skipNewlines()

				var body []Node
				if p.current().Type == lexer.TokenIndent {
					p.advance()
					body = p.parseBody()
				}

				switch eventType {
				case "connect":
					sock.OnConnect = body
				case "message":
					sock.OnMessage = body
				case "disconnect":
					sock.OnDisconnect = body
				}
				continue
			}
			p.advance()
		}
	}

	return sock
}

// parseRateLimit parses:
//
//	limit /api/*
//	  requests: 100 per minute per user
func (p *parserState) parseRateLimit() RateLimit {
	rl := RateLimit{Requests: 100, Window: "minute", Per: "ip"}

	p.advance()

	if p.current().Type == lexer.TokenPath {
		rl.PathPattern = p.advance().Value
	}

	p.skipToEndOfLine()
	p.skipNewlines()

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

			if (tok.Type == lexer.TokenIdentifier || tok.Type == lexer.TokenKeyword) && tok.Value == "requests" {
				p.advance()
				if p.current().Type == lexer.TokenColon {
					p.advance()
				}
				if p.current().Type == lexer.TokenNumber {
					fmt.Sscanf(p.advance().Value, "%d", &rl.Requests)
				}
				for !p.isEOF() && p.current().Type != lexer.TokenNewline && p.current().Type != lexer.TokenDedent {
					if p.current().Type == lexer.TokenIdentifier && p.current().Value == "per" {
						p.advance()
						if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
							val := p.advance().Value
							if val == "minute" || val == "hour" || val == "second" {
								rl.Window = val
							} else if val == "user" || val == "ip" {
								rl.Per = val
							}
						}
					} else {
						p.advance()
					}
				}
				continue
			}

			if (tok.Type == lexer.TokenIdentifier || tok.Type == lexer.TokenKeyword) && tok.Value == "delay" {
				p.advance()
				rl.DelaySecs = p.parseDurationTokens()
				p.skipToEndOfLine()
				continue
			}

			// message: "Custom exceeded message"
			if (tok.Type == lexer.TokenIdentifier || tok.Type == lexer.TokenKeyword) && tok.Value == "message" {
				p.advance()
				if p.current().Type == lexer.TokenColon {
					p.advance()
				}
				if p.current().Type == lexer.TokenString {
					rl.Message = p.advance().Value
				}
				p.skipToEndOfLine()
				continue
			}

			// on exceeded: status 429 message "Too many requests"
			if tok.Type == lexer.TokenKeyword && tok.Value == "on" {
				p.advance()
				if (p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword) && p.current().Value == "exceeded" {
					p.advance()
					if p.current().Type == lexer.TokenColon {
						p.advance()
					}
					// Parse inline: message "text" (status is always 429)
					for !p.isEOF() && p.current().Type != lexer.TokenNewline && p.current().Type != lexer.TokenDedent {
						cur := p.current()
						if (cur.Type == lexer.TokenIdentifier || cur.Type == lexer.TokenKeyword) && cur.Value == "message" {
							p.advance()
							if p.current().Type == lexer.TokenString {
								rl.Message = p.advance().Value
							}
						} else {
							p.advance()
						}
					}
				}
				p.skipToEndOfLine()
				continue
			}

			p.skipToEndOfLine()
		}
	}

	return rl
}

// parseTest parses:
//
//	test "user can create post"
//	  as user with role editor
//	  visit /posts/new
//	  fill title with "Test Post"
//	  submit
//	  expect page /posts contains "Test Post"
func (p *parserState) parseTest() Test {
	t := Test{}

	// consume "test"
	p.advance()

	// test name (string)
	if p.current().Type == lexer.TokenString {
		t.Name = p.advance().Value
	}

	p.skipToEndOfLine()
	p.skipNewlines()

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

			// Read the raw line for the step
			lineNum := p.current().Line
			rawLine := ""
			if lineNum >= 1 && lineNum <= len(p.lines) {
				rawLine = strings.TrimSpace(p.lines[lineNum-1])
			}

			step := parseTestStep(rawLine)
			if step.Action != "" {
				t.Steps = append(t.Steps, step)
			}

			p.skipToEndOfLine()
		}
	}

	return t
}

func parseTestStep(line string) TestStep {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return TestStep{}
	}

	step := TestStep{Action: parts[0]}

	switch parts[0] {
	case "visit":
		if len(parts) > 1 {
			step.Target = parts[1]
		}
	case "fill":
		// fill fieldName with "value"
		if len(parts) > 1 {
			step.Target = parts[1]
		}
		// Extract quoted value after "with"
		idx := strings.Index(line, "with ")
		if idx >= 0 {
			rest := strings.TrimSpace(line[idx+5:])
			step.Value = strings.Trim(rest, "\"")
		}
	case "submit":
		// no args needed
	case "expect":
		// expect page /path contains "text"
		// expect query: SQL returns N
		// expect status 200
		// expect redirect to /path
		step.Target = strings.Join(parts[1:], " ")
		idx := strings.Index(line, "contains ")
		if idx >= 0 {
			rest := strings.TrimSpace(line[idx+9:])
			step.Value = strings.Trim(rest, "\"")
		}
		idx = strings.Index(line, "returns ")
		if idx >= 0 {
			step.Value = strings.TrimSpace(line[idx+8:])
		}
	case "as":
		// as user with role editor
		step.Target = strings.Join(parts[1:], " ")
	}

	return step
}

// parseLogConfig parses:
//
//	log
//	  level: info
//	  queries: slow > 100ms
//	  requests: all
//	  errors: all
func (p *parserState) parseLogConfig() LogConfig {
	cfg := LogConfig{
		Level:       "info",
		SlowQueryMs: 100,
		LogRequests: true,
		LogErrors:   true,
	}

	// consume "log"
	p.advance()

	p.skipToEndOfLine()
	p.skipNewlines()

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

			if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
				key := p.advance().Value
				if p.current().Type == lexer.TokenColon {
					p.advance()
				}

				// Read the raw value from source line
				lineNum := p.current().Line
				rawVal := ""
				if lineNum >= 1 && lineNum <= len(p.lines) {
					line := p.lines[lineNum-1]
					idx := strings.Index(line, ":")
					if idx >= 0 {
						rawVal = strings.TrimSpace(line[idx+1:])
					}
				}

				switch key {
				case "level":
					cfg.Level = resolveEnvValue(rawVal)
				case "queries":
					// "slow > 100ms"
					if strings.Contains(rawVal, ">") {
						ms := 0
						fmt.Sscanf(rawVal, "slow > %dms", &ms)
						if ms > 0 {
							cfg.SlowQueryMs = ms
						}
					}
				case "requests":
					cfg.LogRequests = rawVal == "all"
				case "errors":
					cfg.LogErrors = rawVal == "all" || rawVal == "all with stacktrace"
					if strings.Contains(rawVal, "stacktrace") {
						cfg.Stacktrace = true
					}
				}

				p.skipToEndOfLine()
			} else {
				p.advance()
			}
		}
	}

	return cfg
}

// parseTranslations parses:
//
//	translations
//	  en
//	    welcome: "Welcome back"
//	    users: "Users"
//	  pt
//	    welcome: "Bem vindo de volta"
//	    users: "Usuários"
func (p *parserState) parseTranslations() map[string]map[string]string {
	result := make(map[string]map[string]string)

	// consume "translations"
	p.advance()

	p.skipToEndOfLine()
	p.skipNewlines()

	if p.current().Type != lexer.TokenIndent {
		return result
	}
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

		// Language code: en, pt, es, etc.
		if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
			lang := p.advance().Value
			result[lang] = make(map[string]string)

			p.skipToEndOfLine()
			p.skipNewlines()

			// Parse key-value pairs
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

					if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
						key := p.advance().Value
						if p.current().Type == lexer.TokenColon {
							p.advance()
						}
						if p.current().Type == lexer.TokenString {
							result[lang][key] = p.advance().Value
						}
						p.skipToEndOfLine()
					} else {
						p.advance()
					}
				}
			}
		} else {
			p.advance()
		}
	}

	return result
}

// parseConfig parses:
//
//	config
//	  database: env DATABASE_URL default "sqlite://app.db"
//	  port: env PORT default 8080
//	  secret: env SECRET_KEY required
func (p *parserState) parseConfig() AppConfig {
	cfg := AppConfig{
		Port: 8080,
	}

	// consume "config"
	p.advance()

	p.skipToEndOfLine()
	p.skipNewlines()

	if p.current().Type != lexer.TokenIndent {
		return cfg
	}
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

		if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
			lineNum := p.current().Line
			key := p.advance().Value

			// Handle compound keys
			if key == "default" && (p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword) && p.current().Value == "language" {
				key = "default_language"
				p.advance()
			} else if key == "detect" && (p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword) && p.current().Value == "language" {
				key = "detect_language"
				p.advance()
			}

			if p.current().Type == lexer.TokenColon {
				p.advance()
			}

			rawVal := ""
			if lineNum >= 1 && lineNum <= len(p.lines) {
				line := p.lines[lineNum-1]
				idx := strings.Index(line, ":")
				if idx >= 0 {
					rawVal = strings.TrimSpace(line[idx+1:])
				}
			}

			switch key {
			case "name":
				cfg.Name = strings.Trim(rawVal, "\"")
			case "database":
				cfg.Database = resolveEnvValue(rawVal)
			case "port":
				resolved := resolveEnvValue(rawVal)
				fmt.Sscanf(resolved, "%d", &cfg.Port)
			case "secret":
				cfg.Secret = resolveEnvValue(rawVal)
				if strings.Contains(rawVal, "required") && cfg.Secret == "" {
					fmt.Fprintf(os.Stderr, "kilnx: config secret is required but not set\n")
					os.Exit(1)
				}
			case "static":
				cfg.StaticDir = strings.Trim(rawVal, "\"' ")
			case "uploads":
				parts := strings.Fields(rawVal)
				if len(parts) > 0 {
					cfg.UploadsDir = parts[0]
				}
				for i, pt := range parts {
					if pt == "max" && i+1 < len(parts) {
						fmt.Sscanf(parts[i+1], "%d", &cfg.UploadsMaxMB)
					}
				}
			case "default_language":
				cfg.DefaultLanguage = strings.TrimSpace(rawVal)
			case "detect_language":
				cfg.DetectLanguage = strings.TrimSpace(rawVal)
			case "cors":
				for _, origin := range strings.Split(rawVal, ",") {
					origin = strings.TrimSpace(origin)
					if origin != "" {
						cfg.CORSOrigins = append(cfg.CORSOrigins, origin)
					}
				}
			}

			p.skipToEndOfLine()
		} else {
			p.advance()
		}
	}

	return cfg
}

// parseOnNode parses:
//
//	on success
//	  redirect /users
//	on error
//	  alert error "Something went wrong"
//	on not found
//	  redirect /404
func (p *parserState) parseOnNode() Node {
	node := Node{Type: NodeOn, Props: make(map[string]string)}

	// consume "on"
	p.advance()

	// condition: success, error, forbidden, "not found"
	if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
		cond := p.advance().Value
		// Handle "not found" as two words
		if cond == "not" && (p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword) && p.current().Value == "found" {
			p.advance()
			cond = "not found"
		}
		node.Props["condition"] = cond
	}

	p.skipToEndOfLine()
	p.skipNewlines()

	// Parse children (indented block)
	if p.current().Type == lexer.TokenIndent {
		p.advance()
		node.Children = p.parseBody()
	}

	return node
}

// parseBroadcastNode parses:
//
//	broadcast to :room
//	broadcast to :room fragment chat-message
func (p *parserState) parseBroadcastNode() Node {
	node := Node{Type: NodeBroadcast, Props: make(map[string]string)}

	// consume "broadcast"
	p.advance()

	// expect "to"
	if p.current().Type == lexer.TokenIdentifier && p.current().Value == "to" {
		p.advance()
	}

	// room reference: :room or identifier
	if p.current().Type == lexer.TokenColon {
		p.advance()
		if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
			node.BroadcastRoom = ":" + p.advance().Value
		}
	} else if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
		node.BroadcastRoom = p.advance().Value
	}

	// optional "fragment name"
	if (p.current().Type == lexer.TokenKeyword || p.current().Type == lexer.TokenIdentifier) && p.current().Value == "fragment" {
		p.advance()
		if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
			node.BroadcastFrag = p.advance().Value
		}
	}

	p.skipToEndOfLine()
	return node
}

// parseNamedQueries parses:
//
//	queries
//	  active-users: SELECT u.name FROM users u WHERE u.active = true
//	  recent-posts: SELECT * FROM post ORDER BY created DESC LIMIT 10
func (p *parserState) parseNamedQueries() map[string]string {
	result := make(map[string]string)

	// consume "queries"
	p.advance()

	p.skipToEndOfLine()
	p.skipNewlines()

	if p.current().Type != lexer.TokenIndent {
		return result
	}
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

		if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
			lineNum := p.current().Line
			name := p.advance().Value

			if p.current().Type == lexer.TokenColon {
				p.advance()
			}

			// Extract SQL from source line
			if lineNum >= 1 && lineNum <= len(p.lines) {
				line := p.lines[lineNum-1]
				idx := strings.Index(line, ":")
				if idx >= 0 {
					result[name] = strings.TrimSpace(line[idx+1:])
				}
			}

			p.skipToEndOfLine()
			// Check for continuation SQL
			cont := p.extractContinuationSQL()
			if cont != "" {
				result[name] += cont
			}
		} else {
			p.advance()
		}
	}

	return result
}

// resolveEnvValue handles "env VAR_NAME default VALUE" syntax.
// Returns the resolved value.
func resolveEnvValue(raw string) string {
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return raw
	}

	if parts[0] == "env" && len(parts) >= 2 {
		envVar := parts[1]
		val := os.Getenv(envVar)
		if val != "" {
			return val
		}
		// Check for "default" fallback
		for i, p := range parts {
			if p == "default" && i+1 < len(parts) {
				return strings.Trim(strings.Join(parts[i+1:], " "), "\"")
			}
		}
		return ""
	}

	return strings.Trim(raw, "\"")
}

// parseGeneratePDFNode parses: generate pdf from template X with data Y
func (p *parserState) parseGeneratePDFNode() Node {
	node := Node{Type: NodeGeneratePDF}

	// consume "generate"
	p.advance()

	// expect "pdf"
	if (p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword) && p.current().Value == "pdf" {
		p.advance()
	}

	// expect "from"
	if (p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword) && p.current().Value == "from" {
		p.advance()
	}

	// expect "template"
	if (p.current().Type == lexer.TokenKeyword || p.current().Type == lexer.TokenIdentifier) && p.current().Value == "template" {
		p.advance()
	}

	// template name
	if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword || p.current().Type == lexer.TokenString {
		node.TemplateName = p.advance().Value
	}

	// expect "with"
	if p.current().Type == lexer.TokenKeyword && p.current().Value == "with" {
		p.advance()
	}

	// "data" keyword (optional)
	if (p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword) && p.current().Value == "data" {
		p.advance()
	}

	// data query name
	if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
		node.DataQueryName = p.advance().Value
	}

	p.skipToEndOfLine()
	return node
}

// parseFetchNode parses:
//
//	fetch weather: GET https://api.weather.com/v1?city=:city
//	  header Authorization: env API_KEY
//	  body amount: :amount
func (p *parserState) parseFetchNode() Node {
	node := Node{
		Type:         NodeFetch,
		FetchMethod:  "GET",
		FetchHeaders: make(map[string]string),
		FetchBody:    make(map[string]string),
	}

	fetchLine := p.current().Line

	// consume "fetch"
	p.advance()

	// fetch name (optional): "fetch weather: GET ..."
	if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
		next := p.peek()
		if next.Type == lexer.TokenColon {
			node.Name = p.advance().Value
			p.advance() // consume ':'
		} else {
			val := strings.ToUpper(p.current().Value)
			if val == "GET" || val == "POST" || val == "PUT" || val == "DELETE" || val == "PATCH" {
				node.FetchMethod = val
				p.advance()
			}
		}
	}

	// HTTP method (if name was consumed, method comes next)
	if p.current().Type == lexer.TokenIdentifier || p.current().Type == lexer.TokenKeyword {
		val := strings.ToUpper(p.current().Value)
		if val == "GET" || val == "POST" || val == "PUT" || val == "DELETE" || val == "PATCH" {
			node.FetchMethod = val
			p.advance()
		}
	}

	// URL - extract from raw source line to preserve special chars
	if fetchLine >= 1 && fetchLine <= len(p.lines) {
		line := p.lines[fetchLine-1]
		methods := []string{" GET ", " POST ", " PUT ", " DELETE ", " PATCH "}
		for _, m := range methods {
			if idx := strings.Index(strings.ToUpper(line), m); idx >= 0 {
				node.FetchURL = strings.TrimSpace(line[idx+len(m):])
				break
			}
		}
	}

	p.skipToEndOfLine()
	p.skipNewlines()

	// Parse indented body: header and body declarations
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

			if (tok.Type == lexer.TokenIdentifier || tok.Type == lexer.TokenKeyword) && tok.Value == "header" {
				headerLine := tok.Line
				p.advance()
				if headerLine >= 1 && headerLine <= len(p.lines) {
					line := strings.TrimSpace(p.lines[headerLine-1])
					rest := strings.TrimPrefix(line, "header ")
					if colonIdx := strings.Index(rest, ":"); colonIdx > 0 {
						key := strings.TrimSpace(rest[:colonIdx])
						val := strings.TrimSpace(rest[colonIdx+1:])
						if strings.HasPrefix(val, "env ") {
							node.FetchHeaders[key] = "env:" + strings.TrimSpace(strings.TrimPrefix(val, "env "))
						} else {
							node.FetchHeaders[key] = strings.Trim(val, "\"'")
						}
					}
				}
				p.skipToEndOfLine()
				continue
			}

			if (tok.Type == lexer.TokenIdentifier || tok.Type == lexer.TokenKeyword) && tok.Value == "body" {
				bodyLine := tok.Line
				p.advance()
				if bodyLine >= 1 && bodyLine <= len(p.lines) {
					line := strings.TrimSpace(p.lines[bodyLine-1])
					rest := strings.TrimPrefix(line, "body ")
					if colonIdx := strings.Index(rest, ":"); colonIdx > 0 {
						key := strings.TrimSpace(rest[:colonIdx])
						val := strings.TrimSpace(rest[colonIdx+1:])
						node.FetchBody[key] = strings.Trim(val, "\"'")
					}
				}
				p.skipToEndOfLine()
				continue
			}

			p.advance()
		}
	}

	return node
}

// peek returns the next token without advancing
func (p *parserState) peek() lexer.Token {
	if p.pos+1 < len(p.tokens) {
		return p.tokens[p.pos+1]
	}
	return lexer.Token{Type: lexer.TokenEOF}
}
