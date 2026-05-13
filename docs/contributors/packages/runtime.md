# `internal/runtime`

> Package runtime is the HTTP server and AST interpreter that backs `kilnx run` and the binaries produced by `kilnx build`.

| | |
|---|---|
| **Import path** | `github.com/kilnx-org/kilnx/internal/runtime` |
| **Source last touched** | `2a440f8` (2026-05-13) |
| **Doc last touched** | `5da8498` (2026-05-08) |


> **Implementation touched after doc.go.** Source changed on `2026-05-13`, but `doc.go` was last edited on `2026-05-08`. The summary above may be out of date.

## Overview

Responsibilities:

  - HTTP routing, request parsing, and response rendering for
    pages, actions, fragments, APIs, websockets, and streams.
  - Built-in authentication (email/password, sessions, CSRF).
  - Form handling and validation.
  - Background jobs, scheduled tasks, and webhook delivery.
  - Email sending, file uploads, and SSR template rendering.
  - Live reload during development.

The runtime executes the parser AST directly: there is no
intermediate code generation step at run time. The optimizer
rewrites the AST before the runtime sees it. For ahead-of-time
builds, package internal/build embeds the runtime alongside the
AST so the resulting binary has no compile-time dependency on the
kilnx CLI.

## Files

| File | Summary |
|------|---------|
| [`api.go`](../../../internal/runtime/api.go) | _no file-level doc_ |
| [`auth.go`](../../../internal/runtime/auth.go) | _no file-level doc_ |
| [`computed.go`](../../../internal/runtime/computed.go) | _no file-level doc_ |
| [`dynamic_fields.go`](../../../internal/runtime/dynamic_fields.go) | _no file-level doc_ |
| [`email.go`](../../../internal/runtime/email.go) | _no file-level doc_ |
| [`expr.go`](../../../internal/runtime/expr.go) | _no file-level doc_ |
| [`fetch.go`](../../../internal/runtime/fetch.go) | _no file-level doc_ |
| [`forms.go`](../../../internal/runtime/forms.go) | _no file-level doc_ |
| [`i18n.go`](../../../internal/runtime/i18n.go) | _no file-level doc_ |
| [`interfaces.go`](../../../internal/runtime/interfaces.go) | _no file-level doc_ |
| [`layout.go`](../../../internal/runtime/layout.go) | _no file-level doc_ |
| [`llm.go`](../../../internal/runtime/llm.go) | _no file-level doc_ |
| [`llm_agent.go`](../../../internal/runtime/llm_agent.go) | _no file-level doc_ |
| [`llm_agent_cwd.go`](../../../internal/runtime/llm_agent_cwd.go) | _no file-level doc_ |
| [`llm_stream.go`](../../../internal/runtime/llm_stream.go) | _no file-level doc_ |
| [`logger.go`](../../../internal/runtime/logger.go) | _no file-level doc_ |
| [`permissions.go`](../../../internal/runtime/permissions.go) | _no file-level doc_ |
| [`query_conditional.go`](../../../internal/runtime/query_conditional.go) | _no file-level doc_ |
| [`ratelimit.go`](../../../internal/runtime/ratelimit.go) | _no file-level doc_ |
| [`render.go`](../../../internal/runtime/render.go) | _no file-level doc_ |
| [`scheduler.go`](../../../internal/runtime/scheduler.go) | _no file-level doc_ |
| [`server.go`](../../../internal/runtime/server.go) | _no file-level doc_ |
| [`stream.go`](../../../internal/runtime/stream.go) | _no file-level doc_ |
| [`tenant.go`](../../../internal/runtime/tenant.go) | _no file-level doc_ |
| [`testing.go`](../../../internal/runtime/testing.go) | _no file-level doc_ |
| [`unfurl.go`](../../../internal/runtime/unfurl.go) | _no file-level doc_ |
| [`watcher.go`](../../../internal/runtime/watcher.go) | _no file-level doc_ |
| [`webhook.go`](../../../internal/runtime/webhook.go) | _no file-level doc_ |
| [`websocket.go`](../../../internal/runtime/websocket.go) | _no file-level doc_ |

## Types

### `AppRuntime`

```go
type AppRuntime interface {
	RenderPage(path string, r *http.Request) (string, error)
	HandleAction(w http.ResponseWriter, r *http.Request, action parser.Page, app *parser.App) error
	HandleAPI(w http.ResponseWriter, r *http.Request, api parser.Page)
}
```

AppRuntime is the interface implemented by *Server.
It exists to allow tests and downstream packages to depend on an abstraction
rather than the concrete Server type.

### `EmailConfig`

```go
type EmailConfig struct {
	Host		string	// KILNX_SMTP_HOST (default: localhost)
	Port		string	// KILNX_SMTP_PORT (default: 25)
	User		string	// KILNX_SMTP_USER
	Password	string	// KILNX_SMTP_PASS
	From		string	// KILNX_SMTP_FROM (default: noreply@localhost)
}
```

EmailConfig holds SMTP settings from environment variables

### `I18n`

```go
type I18n struct {
	translations	map[string]map[string]string	// lang -> key -> value
	defaultLanguage	string
	detectLanguages	bool	// whether to detect language from request headers/params
}
```

I18n manages translations

### `JobQueue`

```go
type JobQueue struct {
	server	*Server
	jobs	map[string]parser.Job
	mu	sync.RWMutex
}
```

JobQueue manages async background jobs with SQLite persistence and retry support

### `Logger`

```go
type Logger struct {
	config *parser.LogConfig
}
```

Logger wraps request handling with logging

### `PaginateInfo`

```go
type PaginateInfo struct {
	Page	int
	PerPage	int
	Total	int
	HasPrev	bool
	HasNext	bool
}
```

PaginateInfo holds pagination state for a query

### `PermissionMap`

```go
type PermissionMap map[string]map[string][]PermissionRule
```

PermissionMap indexes role -> model -> rules for fast runtime lookup.
Model names are lower-cased.  The special key "*" holds wildcard rules
produced by the "all" action.

### `PermissionRule`

```go
type PermissionRule struct {
	Action		string	// "all", "read", "write"
	Resource	string	// lower-cased model name
	Condition	string	// raw condition after "where", or empty
}
```

PermissionRule represents a single parsed rule from the permissions block.

### `RateLimiter`

```go
type RateLimiter struct {
	mu	sync.Mutex
	entries	map[string]*rateLimitEntry
	rules	[]parser.RateLimit
}
```

RateLimiter applies path-scoped fixed-window rate limits keyed by IP or
user. Entries expire on a background sweep; auth endpoints get sane
defaults when the developer hasn't configured them.

### `Room`

```go
type Room struct {
	mu	sync.RWMutex
	clients	map[net.Conn]bool
}
```

Room manages connected WebSocket clients for a path

### `Server`

```go
type Server struct {
	app			*parser.App
	db			database.Executor
	sessions		*SessionStore
	jobQueue		*JobQueue
	rateLimiter		*RateLimiter
	logger			*Logger
	i18n			*I18n
	tenants			TenantMap		// models with a `tenant: <model>` directive
	manifestCache		sync.Map		// resolved path -> *parser.CustomFieldManifest (dynamic manifests)
	superuserIdentity	string			// identity of the platform operator; bypasses all role checks
	fragmentComponents	map[string]*parser.Page	// component name -> fragment (for inline rendering)
	mu			sync.RWMutex
	tenantWarnOnce		sync.Once
	port			int
	scheduleStop		chan struct{}
}
```

Server is the runtime HTTP server hosting a parsed Kilnx App. It owns the
database executor, session store, job queue, rate limiter, logger and
i18n table, and routes requests to page, action, fragment and API
handlers.

### `Session`

```go
type Session struct {
	UserID		string
	Identity	string
	Role		string
	ExpiresAt	time.Time
	Data		database.Row	// full user row
}
```

Session stores authenticated user data

### `SessionStore`

```go
type SessionStore struct {
	mu		sync.RWMutex
	sessions	map[string]*Session
	db		database.Executor
	secret		string	// used for HMAC signing of session cookie values
}
```

SessionStore manages sessions with in-memory fast path and SQLite persistence

### `TenantMap`

```go
type TenantMap map[string]string
```

TenantMap indexes model name (lowercased) to the name of its tenant
reference. e.g. {"quote": "org", "customer": "org"} means rows of
"quote" and "customer" are scoped by org_id.

### `agentResult`

```go
type agentResult struct {
	Text		string
	SessionID	string
	StopReason	string
	CostUSD		float64
	DurationMS	int64
}
```

agentResult is the projection of the `result` event exposed to the DSL
as `:<name>.text`, `:<name>.session_id`, `:<name>.cost_usd`,
`:<name>.duration_ms`, `:<name>.stop_reason`.

### `assistantMessage`

```go
type assistantMessage struct {
	Content []struct {
		Type	string		`json:"type"`
		Text	string		`json:"text,omitempty"`
		Name	string		`json:"name,omitempty"`
		ID	string		`json:"id,omitempty"`
		Input	json.RawMessage	`json:"input,omitempty"`
	} `json:"content"`
}
```

assistantMessage covers the shape of `assistant.message` for tool-use
detection when show-tools is on.

### `csrfEntry`

```go
type csrfEntry struct {
	createdAt time.Time
}
```

CSRF token store with expiry (#6 fix: bounded store with TTL cleanup)

### `exprParser`

```go
type exprParser struct {
	src	string
	pos	int
	row	database.Row
}
```
### `hyperstreamWriter`

```go
type hyperstreamWriter struct {
	w		http.ResponseWriter
	flusher		http.Flusher
	target		string
	swap		string
	suspense	string
	channel		string
	seq		int64
}
```

hyperstreamWriter serializes hyperstream <hs-partial> envelopes onto a
Server-Sent Events stream. It is the kilnx server-side counterpart of
the hyperstream JS client (https://github.com/andreahlert/hyperstream).

### `kxExprParser`

```go
type kxExprParser struct {
	src	string
	pos	int
	params	map[string]string
}
```
### `loggingResponseWriter`

```go
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode	int
}
```
### `ogData`

```go
type ogData struct {
	Title		string
	Description	string
	Image		string
	SiteName	string
	URL		string
	FetchedAt	time.Time
}
```
### `parsedFilter`

```go
type parsedFilter struct {
	Name	string
	Args	[]string
}
```

parsedFilter represents a single filter in a chain

### `rateLimitEntry`

```go
type rateLimitEntry struct {
	count		int
	expiresAt	time.Time
}
```
### `renderContext`

```go
type renderContext struct {
	queries			map[string][]database.Row
	paginate		map[string]PaginateInfo
	currentUser		*Session
	queryParams		map[string]string			// URL query parameters (?key=value)
	querySourceModels	map[string]string			// query name -> primary model name (set by analyzer)
	customManifests		map[string]*parser.CustomFieldManifest	// model name -> manifest (for list-page rebinding)
	eachStack		[]database.Row				// stack of active {{each}} rows for parent-scope resolution
	eachModels		[]string				// parallel stack of source model names for each entry in eachStack ("" if unknown)
	models			[]parser.Model				// all models, for resolving computed fields
	currentRow		database.Row				// active row when inside {{each}} block
	fragmentArgs		map[string]string			// active component argument bindings
	fragmentDepth		int					// recursion guard for component fragments
	fragmentComponents	map[string]*parser.Page			// component name -> fragment (for inline rendering)
	actions			[]parser.Page				// declared actions for action= attribute expansion
	i18n			*I18n					// translation engine
	request			*http.Request				// current HTTP request (for language detection)
}
```

renderContext holds query results available during template rendering

### `rowQuerier`

```go
type rowQuerier interface {
	QueryRowsWithParams(sqlStr string, params map[string]string) ([]database.Row, error)
}
```
### `stdlibFn`

```go
type stdlibFn func(args []string) (string, error)
```

stdlibFn implements a Kilnx stdlib function. All values are stringly typed
(Kilnx is string-first end-to-end); functions parse / format on demand.

### `streamInnerEvent`

```go
type streamInnerEvent struct {
	Type	string	`json:"type"`
	Index	int	`json:"index,omitempty"`
	Delta	struct {
		Type	string	`json:"type"`
		Text	string	`json:"text"`
	}	`json:"delta,omitempty"`
	ContentBlock	struct {
		Type	string		`json:"type"`
		Name	string		`json:"name,omitempty"`
		ID	string		`json:"id,omitempty"`
		Input	json.RawMessage	`json:"input,omitempty"`
	}	`json:"content_block,omitempty"`
}
```

streamInnerEvent is the nested envelope inside `stream_event.event`.

### `streamJSONEvent`

```go
type streamJSONEvent struct {
	Type		string		`json:"type"`
	Subtype		string		`json:"subtype,omitempty"`
	SessionID	string		`json:"session_id,omitempty"`
	Message		json.RawMessage	`json:"message,omitempty"`
	Event		json.RawMessage	`json:"event,omitempty"`
	Result		string		`json:"result,omitempty"`
	IsError		bool		`json:"is_error,omitempty"`
	StopReason	string		`json:"stop_reason,omitempty"`
	TotalCostUSD	float64		`json:"total_cost_usd,omitempty"`
	DurationMS	int64		`json:"duration_ms,omitempty"`
}
```

streamJSONEvent is the subset of fields we care about across all event
types emitted by `claude -p --output-format stream-json`.

## Functions

### `BuildPermissionMap`

```go
func BuildPermissionMap(app *parser.App) PermissionMap
```

BuildPermissionMap parses the raw permission strings from the AST into a
structured map for fast runtime lookups.

### `BuildTenantMap`

```go
func BuildTenantMap(app *parser.App) TenantMap
```

BuildTenantMap returns a lookup of tenant-scoped models from an app.
Model names are lowercased for case-insensitive matching. The tenant
reference value is preserved as written so FK column generation stays
consistent with `fieldToColumnName`.

### `CheckPassword`

```go
func CheckPassword(password, hash string) bool
```

CheckPassword compares a plaintext password with a bcrypt hash

### `HashPassword`

```go
func HashPassword(password string) (string, error)
```

HashPassword hashes a plaintext password with bcrypt

### `LoadEmailTemplate`

```go
func LoadEmailTemplate(templateName string, params map[string]string) string
```

LoadEmailTemplate loads a template file and interpolates {key} placeholders

### `NewI18n`

```go
func NewI18n(translations map[string]map[string]string, defaultLang string, detect bool) *I18n
```

NewI18n returns an I18n with the given lang->key->value map. If detect is
true the user's language is resolved from request headers and params,
otherwise defaultLang is always used. Empty defaultLang falls back to "en".

### `NewJobQueue`

```go
func NewJobQueue(server *Server) *JobQueue
```

NewJobQueue returns a JobQueue that knows about the job definitions in
server's app. Call Start to begin processing.

### `NewLogger`

```go
func NewLogger(config *parser.LogConfig) *Logger
```

NewLogger returns a Logger using config, falling back to sensible defaults
(info level, 100ms slow-query threshold, errors logged) when config is nil.

### `NewRateLimiter`

```go
func NewRateLimiter(rules []parser.RateLimit) *RateLimiter
```

NewRateLimiter returns a RateLimiter applying rules plus implicit defaults
for auth endpoints (login, register, forgot-password) and starts a
background goroutine that prunes expired entries.

### `NewServer`

```go
func NewServer(app *parser.App, db database.Executor, port int) *Server
```

NewServer wires app and db into a Server listening on port. It builds the
session store, job queue, rate limiter, logger, tenant map, fragment
component index and i18n table from app's config.

### `NewSessionStore`

```go
func NewSessionStore(secret string) *SessionStore
```

NewSessionStore returns a SessionStore that signs cookies with secret and
runs a background goroutine to expire stale sessions. If secret is empty a
random fallback is generated and a warning is logged to stderr.

### `RewritePermissionSQL`

```go
func RewritePermissionSQL(sql string, pm PermissionMap, role string, params map[string]string) (string, error)
```

RewritePermissionSQL rewrites a SELECT query to include the permission
condition in the WHERE clause.  It follows the same structural conventions
as RewriteTenantSQL but operates on permission rules rather than tenants.

If the user's role has no conditional restriction for the queried table,
the SQL is returned unchanged.

### `RewriteTenantSQL`

```go
func RewriteTenantSQL(sql string, tenants TenantMap, params map[string]string) (string, error)
```

RewriteTenantSQL rewrites a SELECT query against a tenant-scoped table
to include `WHERE <qualifier>.<tenant>_id = :current_user.<tenant>_id`.
Behaviour summary:

  - Empty tenant map: no-op, returns sql unchanged.
  - Non-tenant table: returns sql unchanged.
  - Mutations (INSERT/UPDATE/DELETE) on a tenant table: validated via
    CheckTenantMutation, never rewritten.
  - Tenant-scoped SELECT we can safely rewrite: returns rewritten SQL.
  - Tenant-scoped SELECT we cannot parse safely: returns ErrUnsafeTenantShape.
  - No `current_user.<tenant>_id` in params: returns ErrMissingTenantParam.

Callers MUST NOT execute the original SQL on error. This is the
defense-in-depth contract: when in doubt, fail the query.

This is not a substitute for application-level authorization. It closes
one specific class of bug (forgetting the tenant predicate on a SELECT)
and refuses unsupported shapes so they are addressed by the developer.

### `RunTests`

```go
func RunTests(app *parser.App, db *database.DB, baseURL string) (passed, failed int)
```

RunTests executes all test blocks and returns pass/fail counts

### `SendEmail`

```go
func SendEmail(to, subject, body string) error
```

SendEmail sends an email using SMTP

### `SendEmailWithAttachment`

```go
func SendEmailWithAttachment(to, subject, body, attachPath string) error
```

SendEmailWithAttachment sends an email with a file attachment using MIME multipart

### `SendEmailWithTemplate`

```go
func SendEmailWithTemplate(to, subject, body, templateName string, params map[string]string) error
```

SendEmailWithTemplate sends an email, optionally using a template

### `WatchAndServe`

```go
func WatchAndServe(filename string, db *database.DB, port int) error
```

WatchAndServe loads the Kilnx app at filename, starts the scheduler and
job queue, watches the source file (and its imports) for changes to
trigger hot reload, and serves HTTP on port. Blocks until the server exits.

### `annotateFetchRows`

```go
func annotateFetchRows(rows []database.Row, status int) []database.Row
```

annotateFetchRows attaches status_code/ok to the first row so the result is
usable through renderContext.queries (page render path).

### `applyFilters`

```go
func applyFilters(value string, chain string) (string, bool)
```

applyFilters runs a filter chain on a value, returns (result, isRaw)

### `applyOp`

```go
func applyOp(left string, op byte, right string) (string, error)
```

applyOp performs string-aware arithmetic. If both sides parse as numbers
the result is numeric (int when result is whole, else float). Otherwise
`+` falls back to string concatenation (handy for building keys/ids);
other ops require numbers.

### `arg`

```go
func arg(a []string, i int) string
```
### `bindFetchResult`

```go
func bindFetchResult(params map[string]string, name string, rows []database.Row, status int)
```

bindFetchResult populates params with `<name>.<key>` entries from the first
row of the response, plus `<name>.status_code` and `<name>.ok` so users can
branch with `on <name>.ok` / `on <name>.status_code`.

### `bodyShouldBeJSON`

```go
func bodyShouldBeJSON(headers map[string]string) bool
```

bodyShouldBeJSON returns true if any user-supplied header has Content-Type
set to a JSON media type (case-insensitive).

### `boolStr`

```go
func boolStr(b bool) string
```
### `buildAgentArgs`

```go
func buildAgentArgs(node parser.Node, params map[string]string, mcpConfigPath string) ([]string, error)
```

buildAgentArgs returns the argv passed to `claude`. The user prompt is
piped through stdin; `-p` puts the CLI in non-interactive mode.

### `buildCustomIterRows`

```go
func buildCustomIterRows(row database.Row, manifest *parser.CustomFieldManifest) []database.Row
```

buildCustomIterRows creates synthetic rows for {{each q.custom}} iteration.
Each row has name, value, label, and kind fields. Designed for detail pages
where the query returns a single row; on list pages use {q.custom.field} dot-access.

### `buildFragmentArgs`

```go
func buildFragmentArgs(argStr string, frag *parser.Page, ctx *renderContext) map[string]string
```

buildFragmentArgs parses an argument string and applies defaults for a fragment.

### `buildLLMMessages`

```go
func buildLLMMessages(node parser.Node, db rowQuerier, params map[string]string) ([]anthropic.MessageParam, error)
```
### `buildRequestBody`

```go
func buildRequestBody(node parser.Node, params map[string]string, wantJSON bool) (io.Reader, string, error)
```

buildRequestBody resolves :param references in body values and encodes the
result either as application/json (when wantJSON) or
application/x-www-form-urlencoded. Returns an empty body for GET requests.

### `checkTenantMutation`

```go
func checkTenantMutation(sql, scrubbed string, tenants TenantMap, params map[string]string) error
```

checkTenantMutation verifies that an INSERT / UPDATE / DELETE on a
tenant-scoped table binds the tenant column explicitly. This is not a
rewrite: intent must remain visible in the .kilnx source. The
`scrubbed` argument has had comments and string literals stripped so
we do not accept a bind needle hidden inside a comment.

### `claudeBin`

```go
func claudeBin() string
```

claudeBin can be overridden in tests via t.Setenv("PATH", ...) or by
setting KILNX_CLAUDE_BIN explicitly. Default is the binary on PATH.

### `clientIP`

```go
func clientIP(r *http.Request) string
```
### `compareNumeric`

```go
func compareNumeric(a, b string) int
```

compareNumeric compares two string values as numbers. Returns -1, 0, or 1.

### `computeAcceptKey`

```go
func computeAcceptKey(key string) string
```
### `csrfCleanupLoop`

```go
func csrfCleanupLoop()
```
### `evalAuthExpr`

```go
func evalAuthExpr(expr string, session *Session) bool
```

evalAuthExpr evaluates a boolean expression against session data.
Supports: "field == value", "field != value", "field in ['a','b']",
numeric comparisons, and "and"/"or" conjunctions.

### `evalExpression`

```go
func evalExpression(src string, params map[string]string) (string, error)
```
### `evalSingleAuthCondition`

```go
func evalSingleAuthCondition(expr string, session *Session) bool
```

evalSingleAuthCondition evaluates one predicate: field op value or field in [...].

### `evaluateComputedExpr`

```go
func evaluateComputedExpr(expr string, row database.Row) string
```

evaluateComputedExpr evaluates a computed field expression against a row.
Supports: identifiers (resolved against the row), int/float literals,
+ - * / operators, parentheses. Returns the formatted value, or "" if
any identifier is missing or non-numeric.

### `evaluateCondition`

```go
func evaluateCondition(condition string, ctx *renderContext, currentRow database.Row) bool
```

evaluateCondition evaluates a condition expression.
Supported forms:

	field                                -> truthy check (non-empty, non-zero, non-false)
	field == "value"                     -> equality
	field != "value"                     -> inequality
	queryName.count > 0                  -> numeric comparison
	queryName.count == 0                 -> numeric/string equality
	expr1 and expr2                      -> logical AND
	expr1 or expr2                       -> logical OR

### `evaluateExpect`

```go
func evaluateExpect(step parser.TestStep, lastBody string, lastStatus int, lastLocation string, db *database.DB) bool
```

evaluateExpect handles all expect assertion variants

### `evaluateQueryCondition`

```go
func evaluateQueryCondition(condition string, params map[string]string) bool
```

evaluateQueryCondition evaluates a simple condition against query parameters.
Supported forms:

	params.key                -> truthy check (non-empty)
	params.key == "value"     -> equality
	params.key != "value"     -> inequality

### `evaluateSingleCondition`

```go
func evaluateSingleCondition(condition string, ctx *renderContext, currentRow database.Row) bool
```

evaluateSingleCondition evaluates a single comparison or truthy check.

### `executeFetch`

```go
func executeFetch(node parser.Node, params map[string]string) ([]database.Row, int, error)
```

executeFetch performs an HTTP request and returns the response rows plus
the HTTP status code. A non-nil error indicates a transport-level failure
(DNS, connection, timeout, body read) and should propagate to the caller
so the surrounding action can roll back. HTTP 4xx/5xx are NOT treated as
errors: the response body is parsed and the status code is returned so
the caller may bind `<name>.status_code` / `<name>.ok` and let the user
branch with `on`.

### `executeLLM`

```go
func executeLLM(ctx context.Context, node parser.Node, db rowQuerier, params map[string]string) (string, error)
```
### `executeLLMAgent`

```go
func executeLLMAgent(ctx context.Context, node parser.Node, app *parser.App, params map[string]string, w http.ResponseWriter) (*agentResult, error)
```

executeLLMAgent spawns `claude -p` and consumes the stream-json output.
When node.LLMStreamTarget is set the response writer must be a
http.Flusher; assistant text deltas are emitted as hyperstream
envelopes in real time. The final agentResult exposes cost, duration,
session id, stop reason and the full text.

### `executeLLMStream`

```go
func executeLLMStream(ctx context.Context, w http.ResponseWriter, node parser.Node, db rowQuerier, params map[string]string) error
```

executeLLMStream runs a streaming Messages.New call and writes hyperstream
envelopes onto w. Caller is responsible for setting any auth/CSRF before
invocation; this function takes over the response (writes SSE headers).

### `expandActionAttributes`

```go
func expandActionAttributes(content string, ctx *renderContext) string
```

expandActionAttributes replaces action="/path" on supported elements with
hx-post/hx-delete/etc. It uses the matching action block's Method (always
set by parser, defaults to POST for `action ...`) to pick the verb. For
non-GET verbs a CSRF placeholder ({csrf}) is emitted via hx-vals; the
surrounding pipeline replaces {csrf} with a real token. <form> elements
are intentionally not rewritten.

### `expandColumnModeFields`

```go
func expandColumnModeFields(rows []database.Row, manifest *parser.CustomFieldManifest) []database.Row
```

expandColumnModeFields adds "custom.<field>" aliases for column-mode manifest fields
so that {q.custom.field} template access works the same as JSON-mode fields.

### `expandCustomFields`

```go
func expandCustomFields(rows []database.Row) []database.Row
```

expandCustomFields parses the "custom" JSON column in each row and adds
flattened "custom.<field>" keys so templates can access {q.custom.revenue}.

### `expandEachBlocks`

```go
func expandEachBlocks(content string, ctx *renderContext, nonce string) string
```

expandEachBlocks processes all {{each queryName}}...{{else}}...{{end}} blocks.
Uses a stack-based parser to handle nested blocks correctly.

### `expandFragmentCalls`

```go
func expandFragmentCalls(content string, ctx *renderContext) string
```

expandFragmentCalls processes {{componentName arg=expr}} blocks inside HTML content.
It looks up the component fragment, binds arguments, and renders the body inline.

### `expandFragmentCallsOutsideEach`

```go
func expandFragmentCallsOutsideEach(content string, ctx *renderContext) string
```

expandFragmentCallsOutsideEach expands fragment component calls that are
NOT inside a {{each}}...{{end}} block. Calls inside each-blocks are handled
per-row inside expandEachBlocks (so they can reference the current row).

### `expandIfBlocks`

```go
func expandIfBlocks(content string, ctx *renderContext, currentRow database.Row) string
```

expandIfBlocks processes all {{if expr}}...{{else}}...{{end}} blocks.
currentRow is non-nil when inside an {{each}} block.

### `expandPluralization`

```go
func expandPluralization(content string, ctx *renderContext, params map[string]string) string
```

expandPluralization processes {expr|plural:'singular','plural'} within a string.

### `expandQueryConditionals`

```go
func expandQueryConditionals(sql string, params map[string]string) string
```

expandQueryConditionals processes {{if expr}}...{{end}} blocks inside a SQL
string, using the provided parameters for condition evaluation. It returns the
SQL with conditional fragments included or removed based on the evaluation.

### `expandSingleFragmentCall`

```go
func expandSingleFragmentCall(match string, ctx *renderContext) string
```

expandSingleFragmentCall expands one {{name args}} occurrence using ctx.
Returns the original match if the name is not a known component or if the
recursion guard has fired.

### `expandTranslations`

```go
func expandTranslations(content string, ctx *renderContext) string
```

expandTranslations processes {t.key} and {t.key param=expr} placeholders.
It looks up the translation in the detected language, resolves any parameters,
and applies pluralization filters within the translated value.

### `extractCSRFFromHTML`

```go
func extractCSRFFromHTML(html string) string
```
### `extractEventType`

```go
func extractEventType(payload map[string]interface{}) string
```
### `extractFormAction`

```go
func extractFormAction(htmlContent string) string
```
### `extractFormData`

```go
func extractFormData(r *http.Request, config *parser.AppConfig) map[string]string
```

extractFormData reads form values from a POST request, including file uploads.
config may be nil; defaults are used in that case.

### `fetchOGData`

```go
func fetchOGData(url string) *ogData
```

fetchOGData fetches Open Graph metadata from a URL

### `findComputedField`

```go
func findComputedField(models []parser.Model, modelName, fieldName string) *parser.Field
```

findComputedField returns the parser.Field for a model's computed field by name,
or nil if no such computed field exists.

### `findMatchingEnd`

```go
func findMatchingEnd(content string) (body, elseBody string, endPos int)
```

findMatchingEnd finds the body, else body, and position after {{end}} for a block,
accounting for nested {{each}}/{{if}} blocks.

### `findModel`

```go
func findModel(models []parser.Model, name string) *parser.Model
```

findModel returns a pointer to the model with the given name, or nil if not found.

### `firstLine`

```go
func firstLine(sql string) string
```
### `flattenJSON`

```go
func flattenJSON(obj map[string]interface{}, prefix string) database.Row
```

flattenJSON converts a nested JSON object into a flat map[string]string.
Nested keys are joined with dots: {"user": {"name": "Alice"}} -> {"user.name": "Alice"}

### `flattenMap`

```go
func flattenMap(m map[string]interface{}, prefix string, result map[string]string)
```
### `flattenPayload`

```go
func flattenPayload(payload map[string]interface{}, prefix string) map[string]string
```

flattenPayload converts nested JSON to flat key-value pairs
e.g., {"data": {"id": "123"}} -> {"event_data_id": "123"}

### `fnAbs`

```go
func fnAbs(a []string) (string, error)
```
### `fnBcrypt`

```go
func fnBcrypt(a []string) (string, error)
```
### `fnCeil`

```go
func fnCeil(a []string) (string, error)
```
### `fnClamp`

```go
func fnClamp(a []string) (string, error)
```
### `fnCoalesce`

```go
func fnCoalesce(a []string) (string, error)
```
### `fnContains`

```go
func fnContains(a []string) (string, error)
```
### `fnEnds`

```go
func fnEnds(a []string) (string, error)
```
### `fnFloor`

```go
func fnFloor(a []string) (string, error)
```
### `fnFormat`

```go
func fnFormat(a []string) (string, error)
```
### `fnInt`

```go
func fnInt(a []string) (string, error)
```
### `fnJSONGet`

```go
func fnJSONGet(a []string) (string, error)
```
### `fnMatches`

```go
func fnMatches(a []string) (string, error)
```
### `fnMax`

```go
func fnMax(a []string) (string, error)
```
### `fnMin`

```go
func fnMin(a []string) (string, error)
```
### `fnNow`

```go
func fnNow(a []string) (string, error)
```
### `fnRegexExtract`

```go
func fnRegexExtract(a []string) (string, error)
```
### `fnReplace`

```go
func fnReplace(a []string) (string, error)
```
### `fnRound`

```go
func fnRound(a []string) (string, error)
```
### `fnStarts`

```go
func fnStarts(a []string) (string, error)
```
### `fnUUID`

```go
func fnUUID(a []string) (string, error)
```
### `fnUnary`

```go
func fnUnary(f func(string) string) stdlibFn
```
### `fnUnbase64`

```go
func fnUnbase64(a []string) (string, error)
```
### `formatComputedNumber`

```go
func formatComputedNumber(v float64) string
```

formatComputedNumber renders a float as a string, dropping the trailing ".0" for
integral values so that "2 * 3" renders as "6" rather than "6.000000".

### `formatFloat`

```go
func formatFloat(f float64) string
```
### `formatNumber`

```go
func formatNumber(f float64, decimals int) string
```

formatNumber formats a float with thousand separators and given decimal places

### `generateCSRFToken`

```go
func generateCSRFToken() string
```
### `generateNonce`

```go
func generateNonce() string
```

generateNonce returns a short random hex string for placeholder uniqueness

### `generateSessionID`

```go
func generateSessionID() string
```
### `getEnv`

```go
func getEnv(key, fallback string) string
```
### `getPaginateField`

```go
func getPaginateField(info PaginateInfo, field string) string
```

getPaginateField returns a pagination metadata field value

### `getRoom`

```go
func getRoom(name string) *Room
```
### `goDateFormat`

```go
func goDateFormat(format string) string
```

goDateFormat converts strftime-like format to Go layout

### `hasUserPage`

```go
func hasUserPage(app *parser.App, path string) bool
```

hasUserPage reports whether the app declares a `page` at exactly the
given path. Uses exact string equality (not matchPath) because the
auth dispatcher only needs to cover fixed paths like /login and
/register; parameterised matching would let a page like /login-extra
accidentally shadow the built-in /login.

### `inferHTTPVerb`

```go
func inferHTTPVerb(action *parser.Page) string
```

inferHTTPVerb returns the HTTP verb declared on an action block.
Method is always set by the parser (default "POST" for `action ...`,
see parser.parseAction), so this is a thin uppercase wrapper kept for
clarity at call sites.

### `init`

```go
func init()
```
### `interpolate`

```go
func interpolate(text string, ctx *renderContext) string
```

interpolate replaces {name.field} patterns with query result values
Supports:
  - {queryName.field} -> first row of named query, specific column
  - {queryName.count} -> number of rows in named query (built-in)

### `interpolateEscaped`

```go
func interpolateEscaped(text string, ctx *renderContext) string
```

interpolateEscaped replaces {query.field} patterns with escaped values.
Used for content outside {{each}} blocks.

### `interpolateRow`

```go
func interpolateRow(text string, row database.Row, ctx *renderContext) string
```

interpolateRow replaces {field} patterns with the current row's values (escaped),
and {query.field} with cross-query values (also escaped).

### `isAllowedURLScheme`

```go
func isAllowedURLScheme(rawURL string) bool
```

isAllowedURLScheme returns true if the URL uses an allowed scheme.

### `isAllowedUploadExt`

```go
func isAllowedUploadExt(filename string) bool
```
### `isIdentPart`

```go
func isIdentPart(c byte) bool
```
### `isIdentStart`

```go
func isIdentStart(c byte) bool
```
### `isInsideEachBlock`

```go
func isInsideEachBlock(text string) func(int) bool
```

isInsideEachBlock returns a closure that checks if a position in the text
falls inside a {{each}}...{{end}} block. Used to defer filter processing
to the per-row iteration in expandEachBlocks.

### `isLocalPath`

```go
func isLocalPath(path string) bool
```

isLocalPath validates that a redirect path is local (not an open redirect) (#1 fix)

### `isMutationStart`

```go
func isMutationStart(lower string) bool
```
### `isPathWithinAllowedDirs`

```go
func isPathWithinAllowedDirs(path string) bool
```

isPathWithinAllowedDirs checks whether the given path is inside one of the
allowed directories (current working dir, uploads/, templates/).

### `isSQLKeyword`

```go
func isSQLKeyword(s string) bool
```
### `isSensitiveField`

```go
func isSensitiveField(name, passwordField string) bool
```
### `jsonCoerce`

```go
func jsonCoerce(v string) any
```

jsonCoerce tries to emit numbers/bools/null as native JSON types when the
value is unambiguous. Anything else stays a string. This lets users pass
`body amount: :total` to APIs (Stripe etc.) that require typed numbers.

### `linkify`

```go
func linkify(text string) string
```
### `loadApp`

```go
func loadApp(filename string) (*parser.App, error)
```
### `loadEmailConfig`

```go
func loadEmailConfig() EmailConfig
```
### `looksLikeExpression`

```go
func looksLikeExpression(s string) bool
```
### `matchPath`

```go
func matchPath(pattern, urlPath string) bool
```

matchPath checks if a route pattern matches a URL path.
Supports :param segments: /users/:id matches /users/5

### `matchPathParams`

```go
func matchPathParams(pattern, urlPath string) map[string]string
```

matchPathParams extracts :param values from a URL given a pattern

### `matchRateLimitPath`

```go
func matchRateLimitPath(pattern, path string) bool
```
### `mergeConsecutiveRoles`

```go
func mergeConsecutiveRoles(msgs []anthropic.MessageParam) []anthropic.MessageParam
```

mergeConsecutiveRoles collapses consecutive messages with the same role
to satisfy Anthropic's alternating user/assistant requirement.

### `nextCronOccurrence`

```go
func nextCronOccurrence(expr string) time.Time
```

nextCronOccurrence parses "every monday at 9:00" and returns the next occurrence

### `parseFilterChain`

```go
func parseFilterChain(chain string) []parsedFilter
```

parseFilterChain splits "upcase | truncate: 30 | default: N/A" into filters

### `parseInList`

```go
func parseInList(s string) []string
```

parseInList parses items from a comma-separated list, stripping quotes.

### `parseJSONResponse`

```go
func parseJSONResponse(body []byte) ([]database.Row, error)
```

parseJSONResponse converts JSON into database.Row slices for template use.
Supports: object (single row), array of objects (multiple rows), or wraps primitives.

### `parsePermissionRule`

```go
func parsePermissionRule(raw string) *PermissionRule
```
### `printRoutes`

```go
func printRoutes(app *parser.App)
```
### `processRawInRow`

```go
func processRawInRow(content string, row database.Row, ctx *renderContext, nonce string) string
```

processRawInRow handles {field | filters} inside {{each}} blocks where field comes from the current row.
Skips expressions that are inside nested {{each}} blocks so the inner loop processes them.

### `readWSFrame`

```go
func readWSFrame(reader *bufio.Reader) ([]byte, byte, error)
```

readWSFrame reads a WebSocket frame and returns the payload, opcode, and any error.

### `redactURL`

```go
func redactURL(raw string) string
```

redactURL strips the query string so secrets passed via :param substitution
do not end up in stdout/log lines.

### `redirectWithError`

```go
func redirectWithError(w http.ResponseWriter, r *http.Request, path, msg string)
```

redirectWithError issues a 303 See Other to `path?error=...` so the
user-declared page can re-render with the error visible via a query
parameter (`{error|default:""}`). Keeps POST handlers in Go while
letting the UI live entirely in .kilnx land.

### `render404`

```go
func render404(path string, pages []parser.Page) string
```
### `renderDefaultLayout`

```go
func renderDefaultLayout(title, nav, content string) string
```
### `renderForbidden`

```go
func renderForbidden(pages []parser.Page, session *Session) string
```
### `renderHTML`

```go
func renderHTML(content string, ctx *renderContext) string
```

renderHTML is the main template processing function for NodeHTML content.
It processes all template directives and returns the final HTML string.

### `renderMarkdown`

```go
func renderMarkdown(text string) string
```

renderMarkdown converts a subset of markdown to HTML.
Supports: **bold**, *italic*, `inline code`, ```code blocks```,
[links](url), ~strikethrough~, and newlines to <br>.

### `renderNav`

```go
func renderNav(pages []parser.Page, currentPath string, session *Session, appName string, logoutPath string) string
```
### `renderSSERows`

```go
func renderSSERows(rows []database.Row) string
```

renderSSERows renders query results as HTML for SSE consumption

### `renderUnfurl`

```go
func renderUnfurl(og *ogData) string
```

renderUnfurl generates an HTML card for unfurled link preview

### `renderWithLayout`

```go
func renderWithLayout(layout parser.Layout, title, nav, content string, layoutCtx *renderContext) string
```
### `resolveAgentCwd`

```go
func resolveAgentCwd(node parser.Node, app *parser.App, params map[string]string) (string, func(), error)
```

resolveAgentCwd resolves the working directory for an agent subprocess
against `config workspace-root`. When node.LLMAgentCwd is empty a tmp
directory is created inside workspaceRoot and the returned cleanup
removes it. When declared, the path is :param-expanded, resolved with
EvalSymlinks, and validated to live inside workspaceRoot; cleanup is a
no-op (the admin owns the lifecycle of declared directories).

### `resolveComputedFromQuery`

```go
func resolveComputedFromQuery(ctx *renderContext, queryName, field string) (string, bool)
```

resolveComputedFromQuery attempts to evaluate a computed field {queryName.field}
using the first row of the named query and the model that produced it. Returns
(value, true) on success; otherwise ("", false) so the caller can fall through.

### `resolveComputedFromRow`

```go
func resolveComputedFromRow(ctx *renderContext, row database.Row, modelName, field string) (string, bool)
```

resolveComputedFromRow attempts to evaluate a computed field on a row, using
the model name carried alongside the row. Returns (value, true) on success.

### `resolveEmailRecipient`

```go
func resolveEmailRecipient(to string, params map[string]string) string
```

resolveEmailRecipient resolves the "to" field.
Can be a literal email, a :param from context, or a query result.

### `resolveImports`

```go
func resolveImports(absPath, projectRoot string, seen map[string]bool, depth int) (string, error)
```
### `resolveKxValue`

```go
func resolveKxValue(value string, params map[string]string) string
```

resolveKxValue is the single entry point used by fetch / future-callers to
turn a user-supplied string value into its final form. It tries the
expression evaluator first; on failure it falls back to legacy `:param`
substitution so existing apps keep working.

### `resolvePermissionPlaceholders`

```go
func resolvePermissionPlaceholders(filter string, params map[string]string) string
```

resolvePermissionPlaceholders validates that any :current_user.id reference
has a bound parameter available.

### `resolveQueryParam`

```go
func resolveQueryParam(expr string, params map[string]string) string
```

resolveQueryParam resolves a parameter reference like "params.status" or "status"
from the provided params map.

### `resolveSessionField`

```go
func resolveSessionField(field string, session *Session) string
```

resolveSessionField returns the value of a session field referenced as
"current_user.fieldName" or plain "fieldName".

### `resolveTenantPlaceholder`

```go
func resolveTenantPlaceholder(template string, session *Session, pathParams map[string]string, r *http.Request) string
```

resolveTenantPlaceholder replaces {user.field}, {:param}, {header.H} in a manifest
path template using session, path params, and request headers.

### `resolveValue`

```go
func resolveValue(expr string, ctx *renderContext, currentRow database.Row) string
```

resolveValue resolves a template expression to its value.
Handles: "paginate.query.field", "params.key", "queryName.field", "queryName.count",
bare "field", and parent-scope "^field", "^^field", etc.
Returns the original "{expr}" string if not found.

### `runSingleTest`

```go
func runSingleTest(test parser.Test, app *parser.App, db *database.DB, baseURL string) bool
```
### `sanitizeEmailAddress`

```go
func sanitizeEmailAddress(addr string) string
```

sanitizeEmailAddress removes control characters that could be used for
header injection (CRLF) or null-byte attacks.

### `sanitizeHostHeader`

```go
func sanitizeHostHeader(host string) string
```

sanitizeHostHeader removes control characters and spaces from a Host header
to prevent Host Header Injection attacks.

### `sanitizeIdentifier`

```go
func sanitizeIdentifier(name string) string
```
### `serializeCustomBrackets`

```go
func serializeCustomBrackets(data map[string]string)
```

serializeCustomBrackets collects custom[field]=value entries from form data,
JSON-encodes them as data["custom"], and also promotes each field as a top-level
key so column-mode SQL (e.g. INSERT ... (:revenue)) can bind directly.

### `sha256Hex`

```go
func sha256Hex(s string) string
```
### `slugify`

```go
func slugify(s string) string
```
### `sortStringsByLenDesc`

```go
func sortStringsByLenDesc(s []string)
```
### `splitArgPairs`

```go
func splitArgPairs(s string) []string
```

splitArgPairs tokenizes a translation argument string into space-separated
pairs while respecting quoted values. A naive strings.Split on " " would
corrupt arguments such as name="John Doe".

### `splitArgStr`

```go
func splitArgStr(s string) []string
```

splitArgStr tokenizes a fragment call argument string into space-separated
tokens, respecting single and double quoted substrings. Whitespace inside
quotes is preserved within the token (e.g. title="Hello World").

### `splitCondition`

```go
func splitCondition(condition string) (string, string, string)
```

splitCondition splits "left op right" while respecting quoted strings.
Returns left, operator, right. If no operator found, returns condition, "", "".

### `splitLogical`

```go
func splitLogical(condition, keyword string) []string
```

splitLogical splits a condition string on a logical keyword (" and " or " or "),
respecting quoted strings. Returns the original string in a single-element slice
if the keyword is not found outside quotes.

### `splitPluralSpec`

```go
func splitPluralSpec(spec string) []string
```

splitPluralSpec splits a plural spec by comma, but not inside quotes.

### `stringLen`

```go
func stringLen(s string) int
```
### `stripQuotes`

```go
func stripQuotes(s string) string
```

stripQuotes removes surrounding double or single quotes from a string.

### `stripSQLNoise`

```go
func stripSQLNoise(sql string) string
```

stripSQLNoise removes comments and string literals. The returned
string preserves overall length by replacing stripped ranges with
spaces, so byte positions remain valid for downstream regexps.

### `substituteParams`

```go
func substituteParams(value string, params map[string]string) string
```

substituteParams replaces every `:key` reference in value with its mapped
param. Keys are tried longest-first so `:user.id` takes precedence over
`:user`.

### `timeAgo`

```go
func timeAgo(t time.Time) string
```

timeAgo returns a human-readable relative time string

### `touchesTenantTable`

```go
func touchesTenantTable(sql string, tenants TenantMap) bool
```

touchesTenantTable returns true if the SQL (whitespace-tokenised) mentions
any tenant-scoped model name as a standalone identifier. Conservative: we
would rather reject an innocuous query than miss a tenant leak.

### `translatePermissionCondition`

```go
func translatePermissionCondition(condition, qualifier string) (string, error)
```

translatePermissionCondition converts a simple permission condition like
"author = current_user" into a SQL predicate.
Supported shapes (case-insensitive):

	field = current_user
	field = 'literal'
	field = 123

### `truncateSQL`

```go
func truncateSQL(sql string) string
```
### `unfurlURLs`

```go
func unfurlURLs(text string) string
```

unfurlURLs finds URLs in text and appends link preview cards

### `validateCSRFToken`

```go
func validateCSRFToken(token string) bool
```
### `validateFormData`

```go
func validateFormData(modelName string, app *parser.App, formData map[string]string) []string
```

validateFormData validates form data against model constraints

### `validateInlineRules`

```go
func validateInlineRules(validations []parser.Validation, formData map[string]string) []string
```

validateInlineRules validates form data against explicit validation rules
(not model-based). Supports: "required", "is email", "is date", "min N", "max N"

### `verifySignature`

```go
func verifySignature(payload []byte, signature, secret string) bool
```
### `verifyStripeSignature`

```go
func verifyStripeSignature(payload []byte, header, secret string) bool
```
### `watchFile`

```go
func watchFile(filename string, srv *Server)
```
### `windowDuration`

```go
func windowDuration(window string) time.Duration
```
### `writeJSON`

```go
func writeJSON(w http.ResponseWriter, status int, data interface{})
```
### `writeMCPConfig`

```go
func writeMCPConfig(node parser.Node, app *parser.App) (string, func(), error)
```

writeMCPConfig materialises a `--mcp-config` JSON file for the agent
subset declared by node.LLMAgentMCP. Returns "" and a no-op cleanup
when no servers are referenced.

### `writeWSFrame`

```go
func writeWSFrame(conn net.Conn, data []byte) error
```
### `writeWSPing`

```go
func writeWSPing(conn net.Conn) error
```

writeWSPing sends a WebSocket ping frame (opcode 0x9) with no payload.

### `(I18n) Translate`

```go
func (i *I18n) Translate(key string, r *http.Request) string
```

Translate returns the translation for a key in the detected language

### `(I18n) TranslateAll`

```go
func (i *I18n) TranslateAll(text string, r *http.Request) string
```

TranslateAll replaces {t.key} patterns in text with translations

### `(I18n) detectLanguage`

```go
func (i *I18n) detectLanguage(r *http.Request) string
```

detectLanguage reads Accept-Language header and returns best match

### `(JobQueue) Enqueue`

```go
func (jq *JobQueue) Enqueue(name string, params map[string]string) error
```

Enqueue persists a job to the _kilnx_jobs table

### `(JobQueue) Start`

```go
func (jq *JobQueue) Start()
```

Start recovers any jobs left in the executing state from a previous run
and launches the background poller goroutine.

### `(JobQueue) pollQueue`

```go
func (jq *JobQueue) pollQueue()
```

pollQueue continuously polls _kilnx_jobs for available work

### `(JobQueue) processNextJob`

```go
func (jq *JobQueue) processNextJob()
```
### `(JobQueue) recoverOrphanedJobs`

```go
func (jq *JobQueue) recoverOrphanedJobs()
```

recoverOrphanedJobs resets jobs stuck in 'executing' state back to 'available'.
This handles the case where the server was restarted while jobs were running.

### `(JobQueue) safeExecuteNodes`

```go
func (jq *JobQueue) safeExecuteNodes(nodes []parser.Node, params map[string]string) (err error)
```

safeExecuteNodes runs nodes with panic recovery, returning any error

### `(Logger) LogError`

```go
func (l *Logger) LogError(msg string, err error)
```

LogError logs an error, optionally with a goroutine stack trace.
Nil-tolerant: tests and early-boot paths may hold a nil Logger.

### `(Logger) LogRequest`

```go
func (l *Logger) LogRequest(r *http.Request, status int, duration time.Duration)
```

LogRequest logs an HTTP request if request logging is enabled

### `(Logger) LogSecurity`

```go
func (l *Logger) LogSecurity(msg string, err error)
```

LogSecurity always emits a security-relevant event regardless of the
user's `log.errors` config. Routed to stderr so operators never lose
the signal that a tenant guard, CSRF check, or similar invariant
fired. Nil-tolerant for tests.

### `(Logger) LogSlowQuery`

```go
func (l *Logger) LogSlowQuery(sql string, duration time.Duration)
```

LogSlowQuery logs a query if it exceeds the slow query threshold

### `(Logger) LoggingMiddleware`

```go
func (l *Logger) LoggingMiddleware(next http.Handler) http.Handler
```

LoggingMiddleware wraps an http.Handler with request logging

### `(PermissionMap) CanAccess`

```go
func (pm PermissionMap) CanAccess(role, resource string) bool
```

CanAccess reports whether role has any permission (read or write) for
resource.  It respects the hard-coded role hierarchy as a fallback.

### `(PermissionMap) CanRead`

```go
func (pm PermissionMap) CanRead(role, resource string) bool
```

CanRead reports whether role may read the resource.

### `(PermissionMap) CanWrite`

```go
func (pm PermissionMap) CanWrite(role, resource string) bool
```

CanWrite reports whether role may write the resource.

### `(PermissionMap) ConditionForRead`

```go
func (pm PermissionMap) ConditionForRead(role, resource string) string
```

ConditionForRead returns the first read-condition for role+resource, or "".

### `(PermissionMap) ConditionForWrite`

```go
func (pm PermissionMap) ConditionForWrite(role, resource string) string
```

ConditionForWrite returns the first write-condition for role+resource, or "".

### `(PermissionMap) hasRule`

```go
func (pm PermissionMap) hasRule(role, resource, action string) bool
```
### `(RateLimiter) Check`

```go
func (rl *RateLimiter) Check(r *http.Request, session *Session) bool
```

Check returns true if the request is allowed, false if rate limited

### `(RateLimiter) CheckClause`

```go
func (rl *RateLimiter) CheckClause(count int, period string, key string) bool
```

CheckClause enforces a per-key counter for `requires limit N/period per scope`
clauses. Returns true if the request is within the limit. Uses the same
in-memory map as path-level rules, scoped under a "clause:" prefix.

### `(RateLimiter) CheckWithRule`

```go
func (rl *RateLimiter) CheckWithRule(r *http.Request, session *Session) (bool, *parser.RateLimit)
```

CheckWithRule returns (exceeded bool, matched rule) for the request.
exceeded is true when the request should be blocked.

### `(RateLimiter) cleanup`

```go
func (rl *RateLimiter) cleanup()
```
### `(RateLimiter) minWindow`

```go
func (rl *RateLimiter) minWindow() time.Duration
```

minWindow returns the shortest window duration across all rules

### `(Room) add`

```go
func (room *Room) add(conn net.Conn)
```
### `(Room) broadcast`

```go
func (room *Room) broadcast(message []byte)
```
### `(Room) remove`

```go
func (room *Room) remove(conn net.Conn)
```
### `(Server) RefreshJobQueue`

```go
func (s *Server) RefreshJobQueue()
```

RefreshJobQueue updates the job definitions from the current app (for hot-reload)

### `(Server) Reload`

```go
func (s *Server) Reload(app *parser.App)
```

Reload swaps in a newly parsed app for hot-reload, refreshing superuser
identity and the fragment component index under the server's write lock.

### `(Server) Start`

```go
func (s *Server) Start() error
```

Start binds the HTTP listener and serves the request mux wrapped with the
logging middleware. It blocks until the listener returns.

### `(Server) StartJobQueue`

```go
func (s *Server) StartJobQueue()
```

StartJobQueue starts the background job queue poller.

### `(Server) StartScheduler`

```go
func (s *Server) StartScheduler()
```

StartScheduler launches goroutines for each schedule defined in the app.
It stops any previously running schedule goroutines before starting new ones.

### `(Server) StopScheduler`

```go
func (s *Server) StopScheduler()
```

StopScheduler signals all running schedule goroutines to exit.

### `(Server) buildHandler`

```go
func (s *Server) buildHandler() http.Handler
```
### `(Server) checkRateLimit`

```go
func (s *Server) checkRateLimit(count int, period string, scope string, r *http.Request, session *Session) bool
```

checkRateLimit returns true if the request is within the rate limit.
Uses the in-memory RateLimiter (mutex-protected, race-safe) so the check
works identically on SQLite and PostgreSQL deployments. Single-instance only:
in a multi-process deployment each instance enforces its own counter.

### `(Server) evalRequiresClauses`

```go
func (s *Server) evalRequiresClauses(clauses []parser.RequiresClause, session *Session, r *http.Request) bool
```

evalRequiresClauses returns true when the session satisfies ALL clauses (AND semantics).

Privilege checks (auth, role, expr, superuser) are bypassed for the configured
superuser identity. Availability and traffic controls (flag, rate-limit) are
always evaluated, since a disabled flag or exhausted endpoint is a signal that
no caller, including the operator, should reach the handler.

### `(Server) executeNodes`

```go
func (s *Server) executeNodes(nodes []parser.Node, params map[string]string) error
```

executeNodes runs a list of nodes (for schedules and jobs).
Returns the first error encountered during query execution.

### `(Server) getApp`

```go
func (s *Server) getApp() *parser.App
```
### `(Server) getSession`

```go
func (s *Server) getSession(r *http.Request) *Session
```

getSession extracts the session from the request cookie, verifying HMAC signature

### `(Server) handleAPI`

```go
func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request, endpoint parser.Page)
```

handleAPI processes an API endpoint and returns JSON.
For mutation methods (POST/PUT/DELETE), supports validate, transactions, and redirect.

### `(Server) handleAction`

```go
func (s *Server) handleAction(w http.ResponseWriter, r *http.Request, action parser.Page, app *parser.App)
```

handleAction processes a POST/PUT/DELETE request.
All mutation queries within an action are wrapped in an implicit transaction.

### `(Server) handleActionNodes`

```go
func (s *Server) handleActionNodes(w http.ResponseWriter, r *http.Request, nodes []parser.Node, formData map[string]string, app *parser.App)
```

handleActionNodes processes action nodes inside `on` branches.
Supports redirect, respond, send email, query execution, and job enqueue.

### `(Server) handleForgotPassword`

```go
func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request)
```

handleForgotPassword processes reset requests. Only POST is served;
GET is rendered by the user-declared `page /forgot-password` (enforced
at compile time by the analyzer).

### `(Server) handleLogin`

```go
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request)
```

handleLogin processes the login form POST

### `(Server) handleLogout`

```go
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request)
```

handleLogout clears the session (POST only, CSRF validated)

### `(Server) handleRegister`

```go
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request)
```

handleRegister handles user registration

### `(Server) handleResetPassword`

```go
func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request)
```

handleResetPassword processes the password reset form

### `(Server) handleSocket`

```go
func (s *Server) handleSocket(w http.ResponseWriter, r *http.Request, sock parser.Socket)
```

handleSocket handles WebSocket upgrade and bidirectional communication

### `(Server) handleStream`

```go
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request, stream parser.Stream)
```

handleStream serves a Server-Sent Events endpoint.
It executes the stream's SQL query at the configured interval
and sends results as SSE events.

### `(Server) handleWebhook`

```go
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request, wh parser.Webhook)
```

handleWebhook processes an incoming webhook POST

### `(Server) hasPermission`

```go
func (s *Server) hasPermission(userRole, requiredRole string, perms []parser.Permission) bool
```

hasPermission checks if userRole has the required access level

### `(Server) invalidateDynamicManifestCache`

```go
func (s *Server) invalidateDynamicManifestCache(sql string)
```

invalidateDynamicManifestCache clears the manifest cache entry when a
mutation targets a _<model>_field_defs table.

### `(Server) mergeDBFieldDefs`

```go
func (s *Server) mergeDBFieldDefs(modelName string, base *parser.CustomFieldManifest) *parser.CustomFieldManifest
```

mergeDBFieldDefs queries _<model>_field_defs and merges rows into base.
Static fields always win on name collision.

### `(Server) modelHasCustomFields`

```go
func (s *Server) modelHasCustomFields(app *parser.App, modelName string) bool
```

modelHasCustomFields reports whether a model has any custom field support
(static manifest or dynamic fields).

### `(Server) populateCurrentUserParams`

```go
func (s *Server) populateCurrentUserParams(params map[string]string, sess *Session)
```

populateCurrentUserParams copies the logged-in user's row into `params`
under the `current_user.<column>` prefix, redacting known-sensitive
columns (password, tokens, secrets) plus the `auth.password` column
declared in the .kilnx config. Callers SHOULD use this helper instead
of iterating session.Data directly.

### `(Server) rateLimitKey`

```go
func (s *Server) rateLimitKey(scope string, r *http.Request, session *Session) string
```

rateLimitKey returns the key for rate limiting based on scope.
Returns empty string when the scope cannot be resolved (e.g. `per tenant`
on a single-tenant app); the caller treats empty as "no limit applied".

### `(Server) renderFragment`

```go
func (s *Server) renderFragment(frag parser.Page, r *http.Request) string
```

renderFragment renders a fragment (partial HTML, no page wrapper)

### `(Server) renderFragmentWithParams`

```go
func (s *Server) renderFragmentWithParams(frag parser.Page, params map[string]string) string
```

renderFragmentWithParams renders a fragment using provided params (for WebSocket broadcast)

### `(Server) renderPage`

```go
func (s *Server) renderPage(p parser.Page, allPages []parser.Page, r *http.Request) string
```
### `(Server) requireAPIAuth`

```go
func (s *Server) requireAPIAuth(w http.ResponseWriter, r *http.Request, page parser.Page) bool
```

requireAPIAuth checks auth for API endpoints, returning JSON 401/403 instead of HTML redirect.

### `(Server) requireAuth`

```go
func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request, page parser.Page) bool
```

requireAuth checks if the page requires auth and/or satisfies all requires clauses.
Returns true if the request should proceed, false if redirected or forbidden.

### `(Server) resolveFlag`

```go
func (s *Server) resolveFlag(name string) bool
```

resolveFlag checks if a feature flag is enabled.
Priority: 1) env var FLAG_<NAME>, 2) DB table _kilnx_flags, 3) false

### `(Server) resolveManifest`

```go
func (s *Server) resolveManifest(model *parser.Model, app *parser.App, session *Session, pathParams map[string]string, r *http.Request) *parser.CustomFieldManifest
```

resolveManifest returns the custom field manifest for a model, resolving dynamic
paths at request time. Results are cached per resolved path.

### `(Server) runCronSchedule`

```go
func (s *Server) runCronSchedule(sched parser.Schedule, stop <-chan struct{})
```

runCronSchedule handles "every monday at 9:00" style expressions

### `(Server) runSchedule`

```go
func (s *Server) runSchedule(sched parser.Schedule, stop <-chan struct{})
```
### `(Server) sendSSEEvent`

```go
func (s *Server) sendSSEEvent(w http.ResponseWriter, flusher http.Flusher, stream parser.Stream, params map[string]string)
```
### `(Server) warnTenantScopeOnce`

```go
func (s *Server) warnTenantScopeOnce()
```

warnTenantScopeOnce logs once that `per tenant` resolved to an empty key.
Surfaces a misconfiguration that would otherwise silently disable the limit.

### `(SessionStore) Create`

```go
func (ss *SessionStore) Create(user database.Row, identityField string) string
```

Create issues a new 24h session for user, persists it to SQLite if a DB is
attached, and returns the session ID.

### `(SessionStore) Delete`

```go
func (ss *SessionStore) Delete(id string)
```

Delete removes the session from memory and from the persistence DB.

### `(SessionStore) Get`

```go
func (ss *SessionStore) Get(id string) *Session
```

Get returns the session for id, or nil if missing or expired.

### `(SessionStore) SetDB`

```go
func (ss *SessionStore) SetDB(db database.Executor)
```

SetDB attaches a database for session persistence and loads existing sessions

### `(SessionStore) cleanupLoop`

```go
func (ss *SessionStore) cleanupLoop()
```

cleanupLoop periodically removes expired sessions

### `(SessionStore) loadFromDB`

```go
func (ss *SessionStore) loadFromDB()
```

loadFromDB restores non-expired sessions from SQLite on startup

### `(SessionStore) signSessionID`

```go
func (ss *SessionStore) signSessionID(id string) string
```

signSessionID creates an HMAC-signed cookie value: "id.signature"

### `(SessionStore) verifySessionID`

```go
func (ss *SessionStore) verifySessionID(signed string) (string, bool)
```

verifySessionID verifies and extracts the session ID from a signed cookie value

### `(exprParser) parseExpr`

```go
func (p *exprParser) parseExpr() (float64, bool)
```

parseExpr handles + and -

### `(exprParser) parseFactor`

```go
func (p *exprParser) parseFactor() (float64, bool)
```

parseFactor handles unary minus, parens, identifiers, numbers

### `(exprParser) parseIdent`

```go
func (p *exprParser) parseIdent() (float64, bool)
```
### `(exprParser) parseNumber`

```go
func (p *exprParser) parseNumber() (float64, bool)
```
### `(exprParser) parseTerm`

```go
func (p *exprParser) parseTerm() (float64, bool)
```

parseTerm handles * and /

### `(exprParser) skipSpaces`

```go
func (p *exprParser) skipSpaces()
```
### `(hyperstreamWriter) envelope`

```go
func (h *hyperstreamWriter) envelope(body string, final bool) string
```
### `(hyperstreamWriter) writeDelta`

```go
func (h *hyperstreamWriter) writeDelta(text string) error
```

writeDelta emits a non-final partial carrying a token chunk.

### `(hyperstreamWriter) writeFinal`

```go
func (h *hyperstreamWriter) writeFinal() error
```

writeFinal closes the partial; clients use this to drop suspense placeholders.

### `(hyperstreamWriter) writeSSE`

```go
func (h *hyperstreamWriter) writeSSE(envelope string) error
```
### `(kxExprParser) parseAdditive`

```go
func (p *kxExprParser) parseAdditive() (string, error)
```
### `(kxExprParser) parseCallArgs`

```go
func (p *kxExprParser) parseCallArgs() ([]string, error)
```
### `(kxExprParser) parseMultiplicative`

```go
func (p *kxExprParser) parseMultiplicative() (string, error)
```
### `(kxExprParser) parseNumber`

```go
func (p *kxExprParser) parseNumber() string
```
### `(kxExprParser) parsePrimary`

```go
func (p *kxExprParser) parsePrimary() (string, error)
```
### `(kxExprParser) parseStringLiteral`

```go
func (p *kxExprParser) parseStringLiteral(quote byte) (string, error)
```
### `(kxExprParser) parseUnary`

```go
func (p *kxExprParser) parseUnary() (string, error)
```
### `(kxExprParser) readIdentifier`

```go
func (p *kxExprParser) readIdentifier() string
```
### `(kxExprParser) readIdentifierWithDots`

```go
func (p *kxExprParser) readIdentifierWithDots() string
```
### `(kxExprParser) skipSpace`

```go
func (p *kxExprParser) skipSpace()
```
### `(loggingResponseWriter) Flush`

```go
func (lw *loggingResponseWriter) Flush()
```

Flush proxies to the underlying ResponseWriter for SSE support.

### `(loggingResponseWriter) Hijack`

```go
func (lw *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error)
```

Hijack proxies to the underlying ResponseWriter for WebSocket support.

### `(loggingResponseWriter) WriteHeader`

```go
func (lw *loggingResponseWriter) WriteHeader(code int)
```

WriteHeader records the status code so LoggingMiddleware can report it,
then forwards to the underlying ResponseWriter.


## Notes

<!-- MANUAL-NOTES START -->
# `internal/runtime`

The HTTP server and AST interpreter that backs `kilnx run` and the binaries produced by `kilnx build`. Largest package in the codebase: 27 source files, around 10k lines.

## What this package owns

- HTTP routing, request parsing, response rendering for pages, actions, fragments, APIs, websockets, SSE streams.
- Built-in authentication (email/password, sessions, CSRF) and authorization (permissions, requires clauses, tenant scoping).
- Form handling, validation, file uploads.
- Background jobs, scheduled tasks, webhook receipt and verification.
- SSR template rendering with htmx integration, i18n, layouts.
- Outbound HTTP fetches and Anthropic-LLM calls invocable from `.kilnx` actions.
- Email sending, computed-field evaluation, dynamic field manifests.
- Live reload during development.

The runtime executes the parser AST directly. There is no intermediate code generation step at run time; the optimizer rewrites the AST before the runtime sees it.

## Request lifecycle

Every HTTP request passes through this rough sequence inside `server.go`'s catch-all handler:

1. **Logging middleware** wraps the response writer ([`logger.go`](../../../internal/runtime/logger.go)).
2. **Rate limiter** decides whether to short-circuit with 429 ([`ratelimit.go`](../../../internal/runtime/ratelimit.go)).
3. **Webhook routes** match first, no CSRF ([`webhook.go`](../../../internal/runtime/webhook.go)).
4. **WebSocket routes** match next, upgrade and hand off ([`websocket.go`](../../../internal/runtime/websocket.go)).
5. **Auth POSTs** (login, register, logout, forgot, reset) are handled by the runtime ([`auth.go`](../../../internal/runtime/auth.go)) when an `auth` block exists.
6. **POST/PUT/DELETE** match against declared `action` blocks ([`forms.go`](../../../internal/runtime/forms.go) for body parsing, `server.go` for execution).
7. **SSE streams** match by path ([`stream.go`](../../../internal/runtime/stream.go)).
8. **API endpoints** match and serve JSON ([`api.go`](../../../internal/runtime/api.go)).
9. **Pages and fragments** are the fall-through, rendered by [`render.go`](../../../internal/runtime/render.go).

Cross-cutting concerns intercept at multiple steps: permissions and requires clauses ([`permissions.go`](../../../internal/runtime/permissions.go)), tenant SQL rewriting ([`tenant.go`](../../../internal/runtime/tenant.go)), i18n ([`i18n.go`](../../../internal/runtime/i18n.go)), expression evaluation ([`expr.go`](../../../internal/runtime/expr.go)).

## Sub-area map

### `server.go`

Owner of the HTTP server, the main `*Server` type, and the request dispatch loop. `NewServer` wires together the session store, job queue, rate limiter, logger, tenant map, fragment-component index, and i18n table from the parsed `*parser.App`. `Start` builds the mux (embedded htmx and SSE assets at `/_kilnx/...`, uploads at `/_uploads/`, optional user `static/` at `/_static/`, `GET /healthz`, then the catch-all) and listens. `Reload` swaps in a hot-reloaded app under the server's write lock. The `AppRuntime` interface in [`interfaces.go`](../../../internal/runtime/interfaces.go) describes what tests and downstream packages depend on.

### `auth.go`

Email/password authentication. `SessionStore` keeps an in-memory hot path plus persistence in `_kilnx_sessions`, signs cookie values with HMAC-SHA256 over the configured `config.secret` (random fallback with a stderr warning when missing). Provides `handleLogin`/`handleLogout`/`handleRegister`/`handleForgotPassword`/`handleResetPassword`, bcrypt password hashing, password-reset token generation and email delivery, host-header sanitization. `getSession`/`requireAuth`/`requireAPIAuth` are called from every authenticated route.

### `render.go`

The SSR template engine. Walks `{{each}}`, `{{if}}`, and `{field}` directives over a `renderContext` of executed queries. Handles filter chains (`{name | upcase | truncate: 30 | default: N/A}`), `{csrf}` token expansion, `{paginate.<query>.field}` metadata, `{params.key}` URL-param access. Runs a final i18n pass for `{t.key}` placeholders. Largest single file; consult before adding template syntax.

### `permissions.go`

Parses the `permissions` block into a `PermissionMap` keyed by role and lower-cased model. Recognises `all`/`read`/`write` actions, optional `where` conditions, and a hard-coded admin/editor/viewer hierarchy as a fallback. `CanAccess` and `CheckPermission` are called from page guards, action guards, and tenant rewriting. Requires clauses (per-route fine-grained checks) live alongside in `requires_clauses_*.go` test files but their evaluator lives here and in `expr.go`.

### `forms.go`

Form data extraction and validation. `extractFormData` parses URL-encoded and multipart bodies, including file uploads with extension whitelisting (`allowedUploadExts`). Maintains a CSRF token store with a 30-minute TTL and a background sweeper. Resolves dynamic-field bracket syntax (`custom[fieldname]`) and provides validation helpers used by actions and APIs.

### `scheduler.go`

Background scheduler. `StartScheduler` launches one goroutine per `schedule` block: interval-based (`every 30 seconds`) or cron-style (`every monday at 9:00`, parsed by `nextCronOccurrence`). Also hosts the job queue (`JobQueue`), which polls `_kilnx_jobs` for available rows, runs them with retry/backoff, and exposes `enqueue` to action bodies. `RefreshJobQueue` re-reads job definitions on hot reload.

### `websocket.go`

WebSocket upgrade and broadcast. Implements RFC 6455 framing by hand (no third-party lib): `writeWSFrame`, masking, ping/pong, close handling. `Room` tracks connected clients per path; `broadcast` writes to every conn with a 5-second deadline and prunes dead connections. `handleSocket` runs the user-declared `socket` body in response to inbound messages.

### `stream.go`

Server-Sent Events. Re-runs the stream's SQL query on a ticker and writes each result set as an SSE event with `event: <name>` and `data:` framing per line. Honours `auth`, `requires_role`, and requires clauses before opening the stream. Uses `RewriteTenantSQL` to scope the query before execution. Single SSE event per tick, no diffing.

### `webhook.go`

Inbound webhook handler. POST-only. Body is capped at 1 MiB. When `secret_env` is configured, verifies HMAC-SHA256 signatures in either GitHub format (`X-Hub-Signature-256: sha256=hex`), generic (`X-Signature-256`), or Stripe format (`Stripe-Signature: t=...,v1=...` with a 5-minute replay window). Flattens nested JSON payloads into a flat `event_<key>` param map and routes to the matching `event` handler in the user's `webhook` block.

### `fetch.go`

Outbound HTTP from action bodies (`fetch ...`). 10-second timeout, max 5 redirects. Bodies encode as JSON when a Content-Type header opts in, otherwise form-encoded. Honours `env:VAR` header values, `:param` substitution in URL and headers, and the expression evaluator from `expr.go` for header values. Caps response read at 2 MiB. Returns rows for the template plus `<name>.status_code` and `<name>.ok` so user code can branch with `on <name>.ok`. Redacts query strings in stdout logs to avoid leaking secrets.

### `llm.go`

Anthropic Claude integration. Reads `ANTHROPIC_API_KEY`, defaults to `claude-haiku-4-5-20251001`, supports `system` prompt and `history` SQL (rows with `papel`/`conteudo` columns; `assistente` maps to `assistant`, `sistema` collapses to `user`). Merges consecutive same-role messages so the API's alternating requirement is satisfied. 60-second client timeout, 1 MiB response cap.

### `i18n.go`

Translation lookup. `I18n` holds `lang -> key -> value`. `Translate(key, r)` resolves the request's language from `?lang=` query, then `Accept-Language` (with base-language fallback for `pt-BR -> pt`), falling back to the configured default and finally to the key itself. `TranslateAll` substitutes every `{t.key}` placeholder during a render-pass tail.

### `layout.go`

Layout rendering. `renderDefaultLayout` emits the built-in HTML wrapper (htmx and SSE script tags). `renderWithLayout` substitutes `{page.title}`, `{page.content}`, `{nav}`, `{kilnx.js}` placeholders inside a user-declared `layout` block, then runs the template engine over it for any layout-level queries.

### `logger.go`

Pluggable structured logging. `LogRequest` (request line, status, duration, opt-in via `log.requests`), `LogSlowQuery` (threshold from `log.slow_query_ms`, default 100ms), `LogError` (with optional `runtime.Stack` dump when `log.stacktrace` is set), `LogSecurity` (always to stderr, never silenced; used for tenant-guard violations and CSRF failures). `LoggingMiddleware` wraps a `http.Handler`, capturing the status via a `loggingResponseWriter` that also implements `Hijacker` and `Flusher` so WebSockets and SSE keep working.

### `ratelimit.go`

Fixed-window rate limiter keyed by IP or user. In-memory map capped at `maxRateLimitEntries = 100_000` with adaptive cleanup interval (half the shortest configured window, clamped to 1s..60s). Auth endpoints (`/login`, `/register`, `/forgot-password`) get implicit defaults of 10 req/min/IP unless the developer overrides them. `CheckClause` is the per-key counter used by `requires limit N/period per scope` clauses on routes; entries are namespaced under a `clause:` prefix.

### `api.go`

JSON API endpoint dispatcher. Handles CORS preflight (`OPTIONS`) per `config.cors_origins`. For mutations (POST/PUT/DELETE) opens a `TxHandle`, extracts form/JSON body data, merges with path and query params, and runs the action body inside the transaction. Returns paginated row sets when the query uses `paginate`, otherwise a flat row list. Status codes follow the redirect / validation outcome.

### `computed.go`

Computed-field evaluator. `findComputedField` looks up a `parser.Field` by model and field name; `evaluateComputedExpr` runs a tiny arithmetic parser over identifiers (resolved against the row), int/float literals, `+ - * /` operators, and parentheses. Used during render to project virtual columns that are not stored in the table.

### `dynamic_fields.go`

Dynamic custom-field manifests. `mergeDBFieldDefs` queries `_<model>_field_defs` and merges the rows into the static manifest. Static fields always win on name collision. `invalidateDynamicManifestCache` is called from action bodies that mutate the field-defs table, so the manifest cache (`Server.manifestCache`) is dropped on writes. `modelHasCustomFields` is the cheap lookup used by the SQL rewriter to decide whether to expand `custom.field` shorthand.

### `email.go`

Outbound SMTP. Reads `KILNX_SMTP_HOST`/`PORT`/`USER`/`PASS`/`FROM` from the environment. `LoadEmailTemplate` reads `templates/<name>.html`, interpolates `{key}` placeholders with HTML escaping. `SendEmailWithTemplate` ties the two together. Address sanitization strips CR/LF/NUL to prevent header injection.

### `expr.go`

Expression evaluator and stdlib. Pure functions callable from action bodies (`body amount: round(:total * 1.5)`, `body slug: slugify(:title)`): `lower`, `upper`, `trim`, `len`, `slugify`, `bcrypt`, `sha256`, `base64`/`unbase64`, `uuid`, `now`, `coalesce`, `regex`, `matches`, `json_get`, `round`/`floor`/`ceil`/`abs`, `min`/`max`/`clamp`, `replace`, `contains`, `starts`/`ends`, `int`, `format`. `resolveKxValue` is the single entry point: tries the expression evaluator when input "looks like" a function call, falls back to `:param` substitution otherwise. WASM-style extensibility is reserved for the `plugin` keyword (not yet implemented here).

### `query_conditional.go`

Inline `{{if expr}}...{{else}}...{{end}}` blocks inside SQL strings. Lets queries opt their `WHERE` clauses in or out based on params. Supports truthy checks, `==`, and `!=` against literal strings, with `params.key` and bare `key` references both honoured.

### `tenant.go`

Multi-tenant SQL rewriter. `BuildTenantMap` indexes tenant-scoped models from the `tenant: <ref>` directive. `RewriteTenantSQL` injects the appropriate `WHERE <tenant>_id = :current_user.<tenant>_id` predicate into SELECTs and rejects mutations that do not bind the tenant id explicitly. Fail-closed: any unsupported SQL shape (subquery in FROM, schema-qualified names, missing WHERE position) returns a sentinel error and the runtime aborts the query. Strips comments and string literals before content scans so attackers cannot hide tenant table names inside them.

### `testing.go`

End-to-end test runner for `test` blocks. `RunTests` iterates test scenarios; `runSingleTest` drives an `http.Client` with a cookie jar and `CheckRedirect` returning `http.ErrUseLastResponse` so redirects are observable. Implements steps: `as <role>` (creates user, logs in, extracts CSRF), `visit`, `submit`, `expect`, `expect_redirect`, etc. Used by the `kilnx test` CLI command, not at request time.

### `unfurl.go`

Open Graph metadata fetcher for link previews. 5-second timeout, 3-redirect cap. Caches results in-memory for one hour. Parses `<meta property="og:..."` and `<meta name="twitter:..."` tags plus `<title>` as a fallback. Used by template helpers that render link previews.

### `watcher.go`

Development hot reload. `WatchAndServe` is the entry point used by `kilnx run`: loads the app, prints routes, starts the server, and spawns `watchFile` to poll the source file's mtime every 500ms. Imports are resolved transitively (`resolveImports`, depth cap 64, project-root containment check, `.kilnx` extension required) so editing any imported file triggers a full reparse. On reload `Reload`/`StartScheduler`/`RefreshJobQueue` are called in sequence.

### `interfaces.go`

`AppRuntime` interface implemented by `*Server`. Lets tests and downstream packages depend on the abstraction rather than the concrete type. Exposes `RenderPage`, `HandleAction`, `HandleAPI`.

## Cross-cutting hot spots

A few pieces of behavior are scattered across files because they are touched by many sub-areas:

- **`renderContext`** and the template engine span `render.go`, `layout.go`, and `i18n.go`.
- **`executeNodes`** is the AST walker that runs action bodies; it is in `server.go` but is invoked from `webhook.go`, `scheduler.go`, `api.go`, and the action path.
- **`evalRequiresClauses`** is in `permissions.go` but is called from `stream.go`, `api.go`, the action path, and the page-render guard.
- **`RewriteTenantSQL`** in `tenant.go` is the single chokepoint for tenant scoping; every SQL execution path that touches user data must call it. New SQL execution sites are the most common place to introduce a bug.
- **`resolveKxValue`** and `substituteParams` from `expr.go` are the canonical way to interpolate user-supplied values; never use `strings.ReplaceAll` directly on a `:param` token.

## Gotchas

- **String-first by design**. Every value flowing through the runtime is a string. Numeric coercion happens at the edges (`fnInt`, `jsonCoerce`, computed fields). Resist adding typed scaffolding inside the engine; it shows up as cascading complexity in the templating and SQL layers.
- **Hot reload swaps the AST under a write lock**. Anything that caches parser pointers (manifests, fragment indexes, tenant maps) must be rebuilt in `Reload`. Forgetting one creates ghost behavior after edits.
- **Permissions and requires clauses are independent layers**. The role-based map handles coarse access; clauses handle row-level conditions. Both must pass. Adding a guard means checking both.
- **Tenant rewriter is fail-closed**. If your new SQL shape returns `ErrUnsafeTenantShape`, the right answer is usually to refactor the SQL, not to widen the rewriter. Widening it requires a security review.
- **CSRF tokens have a 30-minute TTL with a background sweeper**. Tests that drive forms slowly can race the sweeper. Use the test client in `testing.go` rather than ad-hoc HTTP.
- **WebSocket framing is hand-rolled**. There is no `gorilla/websocket` here. Bug-fix WebSocket code with care; tests in `websocket_test.go` are the spec.
- **Slow-query callback is wired only for real `*database.DB`**. Tests using a fake `Executor` skip slow-query telemetry; real-world dashboards reflect production behavior only.
- **Auth defaults are aggressive**. `defaultAuthRateLimits` applies even when the user has no `limit` block. Disabling those requires declaring an explicit `limit` for the auth path.
- **`config.secret` falls back to a random ephemeral value**. Sessions invalidate on every restart in that case. The warning is on stderr; ops dashboards must surface it.

## When to touch this package

Almost any user-visible behavior change ends up here. A non-exhaustive triage:

- New keyword or AST node: parser/analyzer changes first, then a new sub-area or extension to an existing one. New files belong with their conceptual neighbors (a new auth method goes in `auth.go`, a new fetch flag in `fetch.go`).
- New template syntax: `render.go`. Update the directive table at the top of the file.
- New stdlib function: `expr.go`, register in the `stdlib` map.
- New permission action: `permissions.go` and the regex.
- New dialect-aware behavior: usually belongs in [`internal/database`](./database.md), not here. Cross-check before adding.
- Anything security-relevant (auth, CSRF, tenant scoping, webhook signatures): always pair with `LogSecurity` and an integration test.
<!-- MANUAL-NOTES END -->
