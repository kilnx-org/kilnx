# `internal/parser`

> Package parser builds the Kilnx AST from a token stream produced by internal/lexer.

| | |
|---|---|
| **Import path** | `github.com/kilnx-org/kilnx/internal/parser` |
| **Source last touched** | `5da8498` (2026-05-08) |
| **Doc last touched** | `5da8498` (2026-05-08) |


## Overview

To regenerate the reference documentation under docs/devs/reference
after editing any *_spec.go file, run:

	go generate ./...

## Files

| File | Summary |
|------|---------|
| [`action_spec.go`](../../../internal/parser/action_spec.go) | _no file-level doc_ |
| [`api_spec.go`](../../../internal/parser/api_spec.go) | _no file-level doc_ |
| [`attrs_spec.go`](../../../internal/parser/attrs_spec.go) | _no file-level doc_ |
| [`auth_spec.go`](../../../internal/parser/auth_spec.go) | _no file-level doc_ |
| [`body_nodes_spec.go`](../../../internal/parser/body_nodes_spec.go) | _no file-level doc_ |
| [`config_spec.go`](../../../internal/parser/config_spec.go) | _no file-level doc_ |
| [`field_attrs_spec.go`](../../../internal/parser/field_attrs_spec.go) | _no file-level doc_ |
| [`fragment_spec.go`](../../../internal/parser/fragment_spec.go) | _no file-level doc_ |
| [`job_spec.go`](../../../internal/parser/job_spec.go) | _no file-level doc_ |
| [`layout_spec.go`](../../../internal/parser/layout_spec.go) | _no file-level doc_ |
| [`limit_spec.go`](../../../internal/parser/limit_spec.go) | _no file-level doc_ |
| [`log_spec.go`](../../../internal/parser/log_spec.go) | _no file-level doc_ |
| [`model_spec.go`](../../../internal/parser/model_spec.go) | _no file-level doc_ |
| [`page_spec.go`](../../../internal/parser/page_spec.go) | _no file-level doc_ |
| [`parser.go`](../../../internal/parser/parser.go) | _no file-level doc_ |
| [`permissions_spec.go`](../../../internal/parser/permissions_spec.go) | _no file-level doc_ |
| [`query_spec.go`](../../../internal/parser/query_spec.go) | _no file-level doc_ |
| [`schedule_spec.go`](../../../internal/parser/schedule_spec.go) | _no file-level doc_ |
| [`socket_spec.go`](../../../internal/parser/socket_spec.go) | _no file-level doc_ |
| [`stream_spec.go`](../../../internal/parser/stream_spec.go) | _no file-level doc_ |
| [`test_spec.go`](../../../internal/parser/test_spec.go) | _no file-level doc_ |
| [`translations_spec.go`](../../../internal/parser/translations_spec.go) | _no file-level doc_ |
| [`webhook_spec.go`](../../../internal/parser/webhook_spec.go) | _no file-level doc_ |

## Types

### `App`

```go
type App struct {
	Models		[]Model
	Pages		[]Page
	Actions		[]Page		// actions share the same structure as pages but handle POST/PUT/DELETE
	Fragments	[]Page		// fragments return partial HTML (no page wrapper)
	APIs		[]Page		// api endpoints return JSON instead of HTML
	Streams		[]Stream	// SSE stream endpoints
	Schedules	[]Schedule	// timed tasks
	Jobs		[]Job		// async background jobs
	Webhooks	[]Webhook	// external event receivers
	Sockets		[]Socket	// bidirectional websockets
	RateLimits	[]RateLimit	// rate limiting rules
	Config		*AppConfig	// nil if no config block defined
	Auth		*AuthConfig	// nil if no auth block defined
	Permissions	[]Permission	// role-based access rules
	Layouts		[]Layout
	Tests		[]Test
	LogConfig	*LogConfig
	Translations	map[string]map[string]string	// lang -> key -> value
	NamedQueries	map[string]string		// name -> SQL
	CustomManifests	map[string]*CustomFieldManifest	// model name -> custom field definitions
}
```

App is the root AST node holding every top-level declaration parsed
from a .kilnx source file.

### `AppConfig`

```go
type AppConfig struct {
	Name		string		// app display name for the topbar
	Database	string		// env var or default path
	Port		int		// server port (default 8080)
	Secret		string		// env var for session secret
	StaticDir	string		// static files directory (served at /_static/)
	UploadsDir	string		// upload directory
	UploadsMaxMB	int		// max upload size in MB
	DefaultLanguage	string		// default i18n language
	DetectLanguage	string		// "header accept-language" or empty
	CORSOrigins	[]string	// allowed CORS origins (empty = same-origin only)
}
```

AppConfig holds top-level application settings from a `config` block.

### `AuthConfig`

```go
type AuthConfig struct {
	Table		string	// user table name (default: "user")
	Identity	string	// identity field (default: "email")
	Password	string	// password field (default: "password")
	LoginPath	string	// login page path (default: "/login")
	LogoutPath	string	// logout POST path (default: "/logout")
	RegisterPath	string	// registration page path (default: "/register")
	ForgotPath	string	// forgot-password page path (default: "/forgot-password")
	ResetPath	string	// reset-password page path (default: "/reset-password")
	AfterLogin	string	// redirect after login (default: "/")
	Superuser	string	// identity of the platform operator (bypasses all role checks)
}
```

AuthConfig configures the built-in authentication subsystem from an
`auth` block. All fields have defaults if left empty.

### `CustomFieldDef`

```go
type CustomFieldDef struct {
	Name		string
	Kind		CustomFieldKind
	Label		string
	Required	bool
	Options		[]string
	Mode		CustomFieldMode	// "" = JSON, "column" = dedicated column
	Reference	string		// target model name for kind: reference
}
```

CustomFieldDef describes a single custom field from a manifest file.

### `CustomFieldKind`

```go
type CustomFieldKind string
```

CustomFieldKind is the type of a runtime-extensible custom field.

### `CustomFieldManifest`

```go
type CustomFieldManifest struct {
	ModelName	string
	Fields		[]CustomFieldDef
}
```

CustomFieldManifest holds all custom field definitions for a model.

### `CustomFieldMode`

```go
type CustomFieldMode string
```

CustomFieldMode controls how a custom field is stored in the database.

### `Field`

```go
type Field struct {
	Name		string
	Type		FieldType
	Required	bool
	Unique		bool
	Default		string
	Auto		bool
	AutoUpdate	bool
	Min		string
	Max		string
	Options		[]string	// for option type: [admin, editor, viewer]
	Reference	string		// for reference type: model name
	Computed	bool		// virtual field, not stored in DB
	ComputedExpr	string		// expression for computed field (e.g. "quantity * unit_price")
}
```

Field is one column on a Model, with type and optional modifiers
(required, unique, default, min/max, computed expression, ...).

### `FieldType`

```go
type FieldType string
```

FieldType is the data type of a Field declared on a Model.

### `FragmentArg`

```go
type FragmentArg struct {
	Name		string
	HasDefault	bool	// true if `=` was provided in the source (even with empty string)
	DefaultValue	string	// value when HasDefault is true; ignored otherwise
}
```

FragmentArg is one named argument accepted by a component-style
fragment (e.g. `fragment badge(status="default")`).

### `Job`

```go
type Job struct {
	Name		string
	Body		[]Node	// query, send email, etc.
	MaxRetries	int	// from "retry N" declaration (default 3)
}
```

Job is a `job` declaration: an asynchronous unit of work invoked via
enqueue nodes.

### `Layout`

```go
type Layout struct {
	Name		string
	HTMLContent	string	// raw HTML with {page.title}, {page.content}, {nav}
	Queries		[]Node	// queries to execute when rendering the layout
}
```

Layout is a named HTML shell, referenced by Page.Layout, that wraps
page content with shared chrome.

### `LogConfig`

```go
type LogConfig struct {
	Level		string	// "debug", "info", "warn", "error"
	SlowQueryMs	int	// log queries slower than this (ms)
	LogRequests	bool
	LogErrors	bool
	Stacktrace	bool	// log errors with stacktrace
}
```

LogConfig configures runtime logging behaviour from a `log` block.

### `Model`

```go
type Model struct {
	Name	string
	// Tenant references another model. Rows of this model are scoped to
	// that tenant. The compiler auto-synthesizes a required reference
	// field named after the tenant model (e.g. `tenant: org` adds an
	// `org_id` column) and the runtime injects a WHERE filter on SELECT
	// queries against this table.
	Tenant			string
	Fields			[]Field
	CustomFieldsFile	string	// path to *_fields.kilnx manifest (may contain {placeholder})
	CustomFieldsFallback	string	// fallback manifest path used when dynamic file not found
	DynamicFields		bool	// opts model into DB-backed runtime field definitions
	// UniqueConstraints lists composite UNIQUE groups declared with
	// `unique (a, b, ...)` directives. Each group is the list of field
	// names as written; references resolve to their `<name>_id` column
	// at DDL generation time.
	UniqueConstraints	[][]string
	// Indexes lists non-unique indexes declared with `index (a, b, ...)`
	// directives. Same field-name resolution as UniqueConstraints.
	Indexes	[][]string
}
```

Model is a `model` declaration: a database table with typed fields,
optional tenant scoping, and composite indexes.

### `Node`

```go
type Node struct {
	Type		NodeType
	Value		string
	Name		string			// for query: result var name
	SQL		string			// for query: the raw SQL
	SourceModel	string			// primary model this query targets (set by analyzer)
	Props		map[string]string	// for on: condition; for send email: body
	Paginate	int			// for query: items per page (0 = no pagination)
	ModelName	string			// for validate: which model to validate against
	QuerySQL	string			// for respond ... query: pre-fill data
	Validations	[]Validation		// for validate block
	RespondTarget	string			// for respond: CSS selector target
	RespondSwap	string			// for respond: htmx swap strategy
	HTMLContent	string			// for html: raw HTML content
	EmailTo		string			// for send email: recipient (:email or query result)
	EmailSubject	string			// for send email: subject line
	EmailTemplate	string			// for send email: template name
	JobName		string			// for enqueue: which job to run
	JobParams	map[string]string	// for enqueue: params to pass to job
	Children	[]Node			// for on: child nodes to execute
	StatusCode	int			// for respond: HTTP status code
	BroadcastRoom	string			// for broadcast: room name
	BroadcastFrag	string			// for broadcast: fragment reference
	TemplateName	string			// for generate pdf: template name
	DataQueryName	string			// for generate pdf: data query name
	EmailAttach	string			// for send email: attachment file path or param
	FetchURL	string			// for fetch: the URL to request
	FetchMethod	string			// for fetch: GET, POST, PUT, DELETE
	FetchHeaders	map[string]string	// for fetch: request headers
	FetchBody	map[string]string	// for fetch: POST body params
	LLMModel	string			// for llm: model name (e.g. claude-sonnet-4-6)
	LLMSystem	string			// for llm: system prompt
	LLMHistorySQL	string			// for llm: SQL to fetch message history
}
```

Node is one statement inside a Page, Action, Schedule, or similar
body. NodeType selects which fields are meaningful; the rest are zero.

### `NodeType`

```go
type NodeType int
```

NodeType discriminates which kind of body statement a Node represents.
The active fields of Node depend on this value.

### `Page`

```go
type Page struct {
	Path		string
	Layout		string
	Title		string
	Auth		bool
	RequiresRole	string			// "auth" = any logged in, "admin"/"editor" = specific role
	RequiresClauses	[]RequiresClause	// full clause list; supersedes RequiresRole when non-nil
	Method		string
	Body		[]Node
	FragmentArgs	[]FragmentArg	// for component fragments (e.g. fragment badge(status)); nil for path-based
}
```

Page is a routable handler. The same struct represents pages, actions,
fragments, and API endpoints; the field set used and the App slice it
lives on distinguish them.

### `Permission`

```go
type Permission struct {
	Role	string		// e.g., "admin", "editor", "viewer"
	Rules	[]string	// e.g., "all", "read post", "write post where author = current_user"
}
```

Permission is a single role-based access rule from a `permissions` block.

### `RateLimit`

```go
type RateLimit struct {
	PathPattern	string	// e.g., "/api/*", "/login"
	Requests	int	// max requests
	Window		string	// "minute", "hour"
	Per		string	// "user", "ip"
	DelaySecs	int	// cooldown on exceeded
	Message		string	// custom message on exceeded
}
```

RateLimit is a top-level `limit` rule that throttles requests matching
a path pattern.

### `RequiresClause`

```go
type RequiresClause struct {
	Kind		RequiresClauseKind
	Value		string	// role name for Role; expression text for Expr; flag name for Flag; empty for Auth/Superuser
	LimitCount	int	// for RateLimit: max requests
	LimitPeriod	string	// for RateLimit: "minute", "hour", "day"
	LimitScope	string	// for RateLimit: "ip", "user", "tenant"
}
```

RequiresClause is one predicate in a comma-separated requires list.
All clauses are ANDed: the user must satisfy every clause.

### `RequiresClauseKind`

```go
type RequiresClauseKind int
```

RequiresClauseKind identifies the type of a single requires predicate.

### `Schedule`

```go
type Schedule struct {
	Name		string
	IntervalSecs	int	// parsed from "every 24h", "every 1m", etc.
	Cron		string	// raw cron expression like "every monday at 9:00"
	Body		[]Node	// query, send email, etc.
}
```

Schedule is a `schedule` declaration: a body of nodes executed on a
recurring interval or cron expression.

### `Socket`

```go
type Socket struct {
	Path		string
	Auth		bool
	RequiresRole	string
	RequiresClauses	[]RequiresClause
	OnConnect	[]Node
	OnMessage	[]Node
	OnDisconnect	[]Node
}
```

Socket is a `socket` declaration: a bidirectional WebSocket endpoint
with connect, message, and disconnect handlers.

### `Stream`

```go
type Stream struct {
	Path		string
	Auth		bool
	RequiresRole	string
	RequiresClauses	[]RequiresClause
	SQL		string	// query to execute on each tick
	IntervalSecs	int	// polling interval in seconds
	EventName	string	// SSE event name (default: "message")
}
```

Stream is a `stream` declaration: a polling SSE endpoint that re-runs
a SQL query on each tick and pushes results to clients.

### `Test`

```go
type Test struct {
	Name	string
	Steps	[]TestStep
}
```

Test is a named scenario from a `test` block, executed as an end-to-end
flow against the running app.

### `TestStep`

```go
type TestStep struct {
	Action	string	// "visit", "fill", "submit", "expect", "as"
	Target	string	// field name, URL, or selector
	Value	string	// value to fill or expected value
}
```

TestStep is a single instruction inside a Test, such as visiting a URL,
filling a field, or asserting a value.

### `Validation`

```go
type Validation struct {
	Field	string
	Rules	[]string	// required, is email, min N, max N
}
```

Validation is one field-level rule set inside a `validate` block.

### `Webhook`

```go
type Webhook struct {
	Path		string
	SecretEnv	string		// env var name for signature verification
	Events		[]WebhookEvent	// on event handlers
}
```

Webhook is a `webhook` declaration: an HTTP endpoint that receives
signed events from an external service and dispatches them by name.

### `WebhookEvent`

```go
type WebhookEvent struct {
	Name	string	// e.g., "payment_intent.succeeded"
	Body	[]Node	// actions to execute
}
```

WebhookEvent is one `on <event>` handler inside a Webhook.

### `multiError`

```go
type multiError []error
```

multiError joins multiple errors into a single error

### `parserState`

```go
type parserState struct {
	tokens		[]lexer.Token
	pos		int
	lines		[]string	// original source lines for raw text extraction
	errors		[]error		// collected parse errors for multi-error reporting
	recovery	bool		// true when skipping tokens to synchronize
}
```
## Functions

### `Parse`

```go
func Parse(tokens []lexer.Token, source string) (*App, error)
```

Parse builds an App AST from a token stream and the original source.
The source is retained for error context and for line-based extraction
of SQL, HTML, and computed expressions. A non-nil error is returned
when one or more parse errors are encountered.

### `ParseManifest`

```go
func ParseManifest(source, modelName string) (*CustomFieldManifest, error)
```

ParseManifest parses a custom fields manifest file (`*_fields.kilnx`).
The manifest contains top-level `field <name>` blocks with kind, label,
required, and option properties.

### `extractPaginate`

```go
func extractPaginate(sql string) (string, int)
```

extractPaginate checks for "paginate N" at the end of SQL and strips it.
Returns the cleaned SQL and the page size (0 if no pagination).

### `firstRoleValue`

```go
func firstRoleValue(clauses []RequiresClause) string
```

firstRoleValue returns the first role/auth string from clauses for backward compat.

### `init`

```go
func init()
```

Field-level constraints applied inside a `model` block, e.g.:

	model user
	  email: email required unique
	  age: int min 18 max 120
	  role: option [admin, editor] default "editor"

### `parseDuration`

```go
func parseDuration(val string) int
```

parseDuration parses "5s", "10s", "1m", "5m" into seconds

### `parseOneClause`

```go
func parseOneClause(s string) RequiresClause
```

parseOneClause classifies a single requires segment.

### `parseRateLimitClause`

```go
func parseRateLimitClause(s string) RequiresClause
```

parseRateLimitClause parses "10/hour per user" into a RequiresClause.

### `parseTestStep`

```go
func parseTestStep(line string) TestStep
```
### `requiresClauseEnd`

```go
func requiresClauseEnd(s string) string
```

requiresClauseEnd walks s (text after "requires") and returns the portion
that belongs to the requires clauses, stopping before a modifier keyword
(method, layout, title) at bracket/quote depth 0.

### `resolveEnvValue`

```go
func resolveEnvValue(raw string) string
```

resolveEnvValue handles "env VAR_NAME default VALUE" syntax.
Returns the resolved value.

### `splitClauseText`

```go
func splitClauseText(text string) []string
```

splitClauseText splits a requires text on commas at bracket/quote depth 0.

### `(multiError) Error`

```go
func (me multiError) Error() string
```
### `(parserState) addError`

```go
func (p *parserState) addError(err error)
```

addError records a parse error and enters recovery mode

### `(parserState) advance`

```go
func (p *parserState) advance() lexer.Token
```
### `(parserState) current`

```go
func (p *parserState) current() lexer.Token
```
### `(parserState) extractComputedExprFromLine`

```go
func (p *parserState) extractComputedExprFromLine(lineNum int) string
```

extractComputedExprFromLine extracts the computed expression from a source line.
For "  total: computed quantity * unit_price", it returns "quantity * unit_price".

### `(parserState) extractContinuationSQL`

```go
func (p *parserState) extractContinuationSQL() string
```

extractContinuationSQL collects indented continuation lines for multi-line SQL

### `(parserState) extractRequiresText`

```go
func (p *parserState) extractRequiresText(lineNum int) string
```

extractRequiresText returns the raw text that follows the `requires` keyword
on source line lineNum, stopping before any modifier keyword (method, layout,
title) at bracket depth 0, and before any inline comment.

### `(parserState) extractSQLFromLine`

```go
func (p *parserState) extractSQLFromLine(lineNum int) string
```

extractSQLFromLine extracts the SQL portion from a source line.
For "  query stats: SELECT count(*) as total FROM user",
it returns "SELECT count(*) as total FROM user"

### `(parserState) extractTitleFromSourceLine`

```go
func (p *parserState) extractTitleFromSourceLine(lineNum, titleCol int) string
```

extractTitleFromSourceLine extracts a dynamic title expression from the source line.
For "page /docs/:slug title {doc.title} layout main", it returns "{doc.title}".

### `(parserState) isEOF`

```go
func (p *parserState) isEOF() bool
```
### `(parserState) parseAPI`

```go
func (p *parserState) parseAPI() (Page, error)
```

parseAPI parses:

	api /api/v1/users requires auth
	  query users: SELECT id, name, email FROM user ORDER BY id
	  paginate 20

### `(parserState) parseAction`

```go
func (p *parserState) parseAction() (Page, error)
```

parseAction parses:

	action /users/create method POST
	  validate user
	  query: INSERT INTO user (name, email) VALUES (:name, :email)
	  redirect /users

### `(parserState) parseAuth`

```go
func (p *parserState) parseAuth() (AuthConfig, error)
```

parseAuth parses:

	auth
	  table: user
	  identity: email
	  password: password
	  login: /login
	  after login: /dashboard

### `(parserState) parseBody`

```go
func (p *parserState) parseBody() []Node
```
### `(parserState) parseBroadcastNode`

```go
func (p *parserState) parseBroadcastNode() Node
```

parseBroadcastNode parses:

	broadcast to :room
	broadcast to :room fragment chat-message

### `(parserState) parseConfig`

```go
func (p *parserState) parseConfig() AppConfig
```

parseConfig parses:

	config
	  database: env DATABASE_URL default "sqlite://app.db"
	  port: env PORT default 8080
	  secret: env SECRET_KEY required

### `(parserState) parseCustomFieldsDirective`

```go
func (p *parserState) parseCustomFieldsDirective() (string, string, error)
```

parseCustomFieldsDirective parses `custom fields from "<path>" [or "<fallback>"]`
and returns (path, fallback, error). Caller verified with peekIsCustomFieldsDirective.

### `(parserState) parseDurationTokens`

```go
func (p *parserState) parseDurationTokens() int
```

parseDurationTokens reads duration from token stream.
Handles both "5s" as single token and "5" + "s" as split tokens.

### `(parserState) parseEnqueueNode`

```go
func (p *parserState) parseEnqueueNode() Node
```

parseEnqueueNode parses:

	enqueue generate-report
	  start_date: :start_date
	  requested_by: :current_user_email

### `(parserState) parseFetchNode`

```go
func (p *parserState) parseFetchNode() Node
```

parseFetchNode parses:

	fetch weather: GET https://api.weather.com/v1?city=:city
	  header Authorization: env API_KEY
	  body amount: :amount

### `(parserState) parseField`

```go
func (p *parserState) parseField(app *App) (Field, error)
```
### `(parserState) parseFragment`

```go
func (p *parserState) parseFragment() (Page, error)
```

parseFragment parses:

	fragment /users/:id/card
	  query user: SELECT name, email FROM user WHERE id = :id
	  html
	    <div class="card">{user.name}</div>

### `(parserState) parseGeneratePDFNode`

```go
func (p *parserState) parseGeneratePDFNode() Node
```

parseGeneratePDFNode parses: generate pdf from template X data Y

### `(parserState) parseHTMLNode`

```go
func (p *parserState) parseHTMLNode() Node
```

parseHTMLNode parses an html block with raw HTML content.
The content is extracted from source lines to preserve all characters.
Supports nested indentation within the html block.

	html
	  <div class="card">{user.name}</div>

### `(parserState) parseIndexDirective`

```go
func (p *parserState) parseIndexDirective() ([]string, error)
```

parseIndexDirective consumes `index ( ident (, ident)* )` and returns
the list of field names. Single-column indexes are allowed (they
accelerate non-equality predicates and sorts in ways that a UNIQUE
index would not).

### `(parserState) parseJob`

```go
func (p *parserState) parseJob() Job
```

parseJob parses:

	job generate-report
	  query data: SELECT * FROM order WHERE created > :start_date
	  send email to :requested_by
	    subject: "Your report is ready"

### `(parserState) parseLLMNode`

```go
func (p *parserState) parseLLMNode() Node
```

parseLLMNode parses:

	llm varname: model-name
	  history: SELECT papel, conteudo FROM mensagem WHERE conversa_id = :id ORDER BY criada ASC
	  system: You are a helpful assistant...

### `(parserState) parseLayout`

```go
func (p *parserState) parseLayout() Layout
```

parseLayout parses:

	layout main
	  html
	    <html>
	    <head><title>{page.title}</title></head>
	    <body>{nav}{page.content}</body>
	    </html>

### `(parserState) parseLogConfig`

```go
func (p *parserState) parseLogConfig() LogConfig
```

parseLogConfig parses:

	log
	  level: info
	  slow-query: 100ms
	  requests: all
	  errors: all

### `(parserState) parseManifestField`

```go
func (p *parserState) parseManifestField() (CustomFieldDef, error)
```
### `(parserState) parseModel`

```go
func (p *parserState) parseModel(app *App) (Model, error)
```

parseModel parses:

	model user
	  name: text required min 2 max 100
	  email: email unique
	  role: option [admin, editor, viewer] default viewer
	  active: bool default true
	  created: timestamp auto
	  author: user required        (reference to another model)

### `(parserState) parseOnNode`

```go
func (p *parserState) parseOnNode() Node
```

parseOnNode parses:

	on success
	  redirect /users
	on error
	  alert error "Something went wrong"
	on not found
	  redirect /404

### `(parserState) parsePage`

```go
func (p *parserState) parsePage() (Page, error)
```
### `(parserState) parsePermissions`

```go
func (p *parserState) parsePermissions() []Permission
```

parsePermissions parses:

	permissions
	  admin: all
	  editor: read post, write post where author = current_user
	  viewer: read post where status = published

### `(parserState) parseQueryNode`

```go
func (p *parserState) parseQueryNode() (Node, error)
```

parseQueryNode parses: query name: SELECT ... (rest of line and continuation lines)
Extracts SQL directly from source lines to preserve special chars like (), *, etc.

### `(parserState) parseRateLimit`

```go
func (p *parserState) parseRateLimit() RateLimit
```

parseRateLimit parses:

	limit /api/*
	  requests: 100 per minute per user

### `(parserState) parseRedirectNode`

```go
func (p *parserState) parseRedirectNode() Node
```

parseRedirectNode parses: redirect /users

### `(parserState) parseRequiresClauses`

```go
func (p *parserState) parseRequiresClauses(lineNum int) []RequiresClause
```

parseRequiresClauses reads the raw source line at lineNum to extract the full
requires text (preserving dots and single quotes the lexer would drop), then
parses it into a []RequiresClause.

### `(parserState) parseRespondNode`

```go
func (p *parserState) parseRespondNode() Node
```

parseRespondNode parses:

	respond fragment ".user-row" query: SELECT * FROM user WHERE id = :id
	respond fragment delete

### `(parserState) parseSchedule`

```go
func (p *parserState) parseSchedule() Schedule
```

parseSchedule parses:

	schedule cleanup every 24h
	  query: DELETE FROM session WHERE expires_at < now()

	schedule report every monday at 9:00
	  query stats: SELECT count(*) as new_users FROM user
	  send email to admin@test.com
	    subject: "Weekly report"

### `(parserState) parseSendEmailNode`

```go
func (p *parserState) parseSendEmailNode() Node
```

parseSendEmailNode parses:

	send email to :email
	  subject: "Welcome"
	  template: welcome

or inline: send email to admin@test.com subject "Report ready"

### `(parserState) parseSocket`

```go
func (p *parserState) parseSocket() Socket
```

parseSocket parses:

	socket /chat/:room requires auth
	  on connect
	    query: SELECT ...
	  on message
	    query: INSERT ...

### `(parserState) parseStream`

```go
func (p *parserState) parseStream() (Stream, error)
```

parseStream parses:

	stream /notifications requires auth
	  query: SELECT message, created_at FROM notifications WHERE seen = false
	  every 5s

### `(parserState) parseTenantDirective`

```go
func (p *parserState) parseTenantDirective() (string, error)
```

parseTenantDirective parses `tenant: <model>` and returns the referenced
model name. Caller has already verified with peekIsTenantDirective.

### `(parserState) parseTest`

```go
func (p *parserState) parseTest() Test
```

parseTest parses:

	test "user can create post"
	  as editor
	  visit /posts/new
	  fill title "Test Post"
	  submit
	  expect page /posts contains "Test Post"

### `(parserState) parseTranslations`

```go
func (p *parserState) parseTranslations() map[string]map[string]string
```

parseTranslations parses:

	translations
	  en
	    welcome: "Welcome back"
	    users: "Users"
	  pt
	    welcome: "Bem vindo de volta"
	    users: "Usuários"

### `(parserState) parseUniqueDirective`

```go
func (p *parserState) parseUniqueDirective() ([]string, error)
```

parseUniqueDirective consumes `unique ( ident (, ident)+ )` and returns
the list of field names. Caller has already verified with
peekIsUniqueDirective. Groups with fewer than two fields are rejected
because single-field uniqueness is expressed with the field-level
constraint `field: <type> unique`.

### `(parserState) parseValidateNode`

```go
func (p *parserState) parseValidateNode() Node
```

parseValidateNode parses:

	validate user
	  (uses model constraints automatically)

or:

	validate
	  name: required
	  email: required, is email

### `(parserState) parseWebhook`

```go
func (p *parserState) parseWebhook() Webhook
```

parseWebhook parses:

	webhook /stripe/payment secret env STRIPE_SECRET
	  on event payment_intent.succeeded
	    query: UPDATE order SET status = 'paid' WHERE stripe_id = :event_id

### `(parserState) peek`

```go
func (p *parserState) peek() lexer.Token
```

peek returns the next token without advancing

### `(parserState) peekIsCustomFieldsDirective`

```go
func (p *parserState) peekIsCustomFieldsDirective() bool
```

peekIsCustomFieldsDirective returns true when the next tokens form
`custom fields from "<path>"` rather than a regular field declaration.

### `(parserState) peekIsDynamicFieldsDirective`

```go
func (p *parserState) peekIsDynamicFieldsDirective() bool
```

peekIsDynamicFieldsDirective returns true when the next token is `fields`,
distinguishing `dynamic fields` from a regular field named `dynamic`.

### `(parserState) peekIsIndexDirective`

```go
func (p *parserState) peekIsIndexDirective() bool
```

peekIsIndexDirective reports whether the tokens at the current position
match `index (` (a non-unique index directive).

### `(parserState) peekIsTenantDirective`

```go
func (p *parserState) peekIsTenantDirective() bool
```

peekIsTenantDirective returns true if the tokens at the current position
match the pattern `tenant : <identifier>` (the meta directive), so we can
distinguish it from a regular field named "tenant".

### `(parserState) peekIsUniqueDirective`

```go
func (p *parserState) peekIsUniqueDirective() bool
```

peekIsUniqueDirective reports whether the tokens at the current position
match `unique (` — a composite UNIQUE directive — rather than the field
constraint `unique` appearing as part of `<name>: <type> unique`.

### `(parserState) skipNewlines`

```go
func (p *parserState) skipNewlines()
```
### `(parserState) skipRequiresClauses`

```go
func (p *parserState) skipRequiresClauses()
```

skipRequiresClauses advances the token stream past clause tokens, stopping
before a modifier keyword (method, layout, title) or end of line.

### `(parserState) skipToEndOfLine`

```go
func (p *parserState) skipToEndOfLine()
```

skipToEndOfLine advances past all tokens on the current line

### `(parserState) skipToEndOfModifier`

```go
func (p *parserState) skipToEndOfModifier()
```

skipToEndOfModifier advances past tokens until hitting a page modifier keyword or newline

### `(parserState) synchronize`

```go
func (p *parserState) synchronize()
```

synchronize advances to the next top-level keyword (indent level 0).
This allows the parser to recover from errors and continue parsing.


## Notes

<!-- MANUAL-NOTES START -->
# Package `internal/parser`

Source: [parser.go](../../../internal/parser/parser.go), [doc.go](../../../internal/parser/doc.go).

## Purpose

Build the Kilnx AST from the token stream produced by [`internal/lexer`](../../../internal/lexer). Each top level keyword (`page`, `action`, `model`, ...) has a parse function in this package and a co-located `*_spec.go` file that registers a language spec entry with [`internal/spec`](../../../internal/spec) for documentation generation.

## Pipeline position

```
[]lexer.Token -> parser.Parse -> *parser.App -> analyzer -> optimizer -> runtime
```

The parser is the first stage that sees structure. It also performs one cross referencing pass: after every block has been built, it resolves named query references in page, action, fragment, api, schedule, job, webhook, and socket bodies by substituting the SQL of `app.NamedQueries[name]` when a `query` body node carries only a name.

## Public API

```go
func Parse(tokens []lexer.Token, source string) (*App, error)
func ParseManifest(source, modelName string) (*CustomFieldManifest, error)
```

`Parse` returns the populated `App` even when errors are present. The error value is a `multiError`: parse errors are collected and recovery resumes at the next top level keyword via `parserState.synchronize`. Callers can still inspect the partial AST.

`ParseManifest` parses a `*_fields.kilnx` custom field manifest file and produces a `CustomFieldManifest` independent from the main app.

`App` is the root AST node. Key fields: `Models`, `Pages`, `Actions`, `Fragments`, `APIs`, `Streams`, `Schedules`, `Jobs`, `Webhooks`, `Sockets`, `RateLimits`, `Config`, `Auth`, `Permissions`, `Layouts`, `Tests`, `LogConfig`, `Translations`, `NamedQueries`, `CustomManifests`. Each is documented inline in [parser.go](../../../internal/parser/parser.go).

Per entity parse functions live on `parserState` and are dispatched from the keyword switch in `Parse`: `parseModel`, `parsePage`, `parseAction`, `parseFragment`, `parseAPI`, `parseStream`, `parseSchedule`, `parseJob`, `parseWebhook`, `parseSocket`, `parseRateLimit`, `parseConfig`, `parseAuth`, `parsePermissions`, `parseLayout`, `parseTest`, `parseLogConfig`, `parseTranslations`, `parseQueryNode`.

## The `*_spec.go` registration pattern

Every keyword and shared attribute has a sibling `*_spec.go` file with an `init()` that calls `spec.Register`. The parser package therefore self describes: importing it populates the registry. [`cmd/kilnx-gendocs`](../../../cmd/kilnx-gendocs/main.go) imports the parser to walk this registry and emit the markdown reference under `docs/devs/reference`.

```
//go:generate go run ../../cmd/kilnx-gendocs -o ../../docs/devs/reference
```

Run `go generate ./...` after editing any spec file.

## File map

- [`parser.go`](../../../internal/parser/parser.go): all AST type definitions, the `Parse` entry point, `parserState`, every per-entity parser, named query resolution, error recovery, `ParseManifest`.
- [`doc.go`](../../../internal/parser/doc.go): package doc and `go:generate` directive.
- [`action_spec.go`](../../../internal/parser/action_spec.go): registers `action`, a state changing endpoint (POST/PUT/DELETE).
- [`api_spec.go`](../../../internal/parser/api_spec.go): registers `api`, a JSON returning endpoint.
- [`attrs_spec.go`](../../../internal/parser/attrs_spec.go): shared cross keyword attributes such as `method` and `requires`.
- [`auth_spec.go`](../../../internal/parser/auth_spec.go): registers `auth` plus its child attributes (`table`, `identity`, `password`, `login`, ...).
- [`body_nodes_spec.go`](../../../internal/parser/body_nodes_spec.go): body level keywords (`html`, `validate`, `redirect`, `query`, `respond`, `enqueue`, `broadcast`, `send`, `fetch`, `llm`).
- [`config_spec.go`](../../../internal/parser/config_spec.go): registers `config` and its attributes.
- [`field_attrs_spec.go`](../../../internal/parser/field_attrs_spec.go): per-field model attributes (`required`, `unique`, `default`, `min`, `max`, `auto`, `auto_update`).
- [`fragment_spec.go`](../../../internal/parser/fragment_spec.go): registers `fragment` (route based and component based).
- [`job_spec.go`](../../../internal/parser/job_spec.go): registers `job` and `retry`.
- [`layout_spec.go`](../../../internal/parser/layout_spec.go): registers `layout`.
- [`limit_spec.go`](../../../internal/parser/limit_spec.go): registers `limit` (rate limit rule).
- [`log_spec.go`](../../../internal/parser/log_spec.go): registers `log` configuration.
- [`model_spec.go`](../../../internal/parser/model_spec.go): registers `model`, a table declaration with typed fields and constraints.
- [`page_spec.go`](../../../internal/parser/page_spec.go): registers `page`, an HTTP route and view.
- [`permissions_spec.go`](../../../internal/parser/permissions_spec.go): registers `permissions`, role based access rules.
- [`query_spec.go`](../../../internal/parser/query_spec.go): registers `query`, used both top level (named query) and inside bodies.
- [`schedule_spec.go`](../../../internal/parser/schedule_spec.go): registers `schedule`, interval or cron task.
- [`socket_spec.go`](../../../internal/parser/socket_spec.go): registers `socket`, bidirectional WebSocket endpoint.
- [`stream_spec.go`](../../../internal/parser/stream_spec.go): registers `stream`, SSE endpoint that pushes results on an interval.
- [`test_spec.go`](../../../internal/parser/test_spec.go): registers `test`, end to end browser style scenario.
- [`translations_spec.go`](../../../internal/parser/translations_spec.go): registers `translations`, i18n strings keyed by language and key.
- [`webhook_spec.go`](../../../internal/parser/webhook_spec.go): registers `webhook`, external event receiver at a path.

## Key behaviors and gotchas

**Error recovery.** `parserState.addError` records a `multiError` entry and flips `recovery = true`. `synchronize()` then advances tokens until the next top level keyword (column 1 indent). The keyword switch in `Parse` uses `continue` after each error so unrelated entities still parse.

**Source retention.** `Parse` keeps `lines := strings.Split(source, "\n")` on `parserState`. Parsers extract raw SQL, raw HTML templates, and computed field expressions directly from the original lines rather than reconstructing from tokens. This preserves whitespace, capitalisation, and embedded punctuation.

**Named query resolution lives in `Parse`, not the optimizer.** After all top level entities are parsed, `Parse` walks `Pages`, `Actions`, `Fragments`, `APIs`, `Schedules`, `Jobs`, `Webhooks`, and `Sockets` and rewrites any `NodeQuery` whose `SQL` is just a bare identifier to the registered SQL from `app.NamedQueries`. The optimizer assumes this has already happened.

**`Action` and `Page` share a struct.** `App.Actions`, `App.Fragments`, and `App.APIs` are all `[]Page`. They are distinguished by which slice they live on and by `Page.Method`. This lets layout, `requires`, and body handling be reused.

**Tenant fields are not present in source.** `parseModel` records `model.Tenant`. The `<tenant>_id` reference column is materialised later during DDL generation; the analyzer is aware of this and does not flag the missing field.

**Component fragments versus route fragments.** `parseFragment` switches on whether the next token is a path or `name(args)`. Component fragments populate `Page.FragmentArgs`; route fragments leave it nil. The analyzer's [`checkFragmentComponents`](../../../internal/analyzer/analyzer.go) relies on this distinction.

**Multi error type.** `multiError` is unexported; callers should treat the returned error as opaque or call `Error()` and split on newlines. The CLI prints each line.

**Top level `query` declarations are named queries.** Inside `Parse`, an unnamed top level `query` raises an error: SQL captured at the top level must have a name to be reusable. This is also where `app.NamedQueries` is populated and merged across multiple `query` blocks.

**Translations are merge keyed.** Multiple `translations` blocks are folded together: existing keys for the same language are overwritten by later blocks. Authors who want strict separation should keep one block per language file.

**`Permission` is flattened.** `parsePermissions` returns a slice that is appended to `app.Permissions`. Multiple `permissions` blocks therefore concatenate; ordering follows source order.

**Custom field manifests are loaded out of band.** `parseModel` records `model.CustomFieldsFile` and `model.CustomFieldsFallback` only. The actual manifest parsing happens through `ParseManifest` which is called by the CLI build pipeline once the file has been resolved on disk; the resulting `CustomFieldManifest` is then attached to `app.CustomManifests`.

**Computed fields keep raw source.** `Field.ComputedExpr` is populated from `extractComputedExprFromLine`, which slices the original source rather than rebuilding from tokens, preserving operators and parentheses verbatim.

**Layouts capture raw HTML.** `Layout.HTMLContent` is the indented block from the source file. Placeholders `{page.title}`, `{page.content}`, and `{nav}` are interpreted at render time, not parse time.

## Testing entry points

- Top level parse coverage: [`parser_test.go`](../../../internal/parser/parser_test.go).
- Spec consistency: [`spec_test.go`](../../../internal/parser/spec_test.go).
- Domain specific scenarios: `requires_clauses_test.go`, `requires_clause_test.go`, `dynamic_fields_test.go`, `computed_test.go`, `fragment_component_test.go`.
- Benchmarks and fuzz: `bench_test.go`, `fuzz_test.go`.
- Fixtures live under [`testdata/`](../../../internal/parser/testdata).
<!-- MANUAL-NOTES END -->
