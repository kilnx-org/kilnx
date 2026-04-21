package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

const protocolVersion = "2024-11-05"

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Serve starts the MCP server on stdin/stdout using JSON-RPC 2.0 over stdio.
func Serve() {
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	for {
		msg, err := readMessage(reader)
		if err != nil {
			if err == io.EOF {
				return
			}
			continue
		}

		var req mcpRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			continue
		}

		// Notifications have no ID — acknowledge and skip.
		if len(req.ID) == 0 || string(req.ID) == "null" {
			continue
		}

		switch req.Method {
		case "initialize":
			writeMessage(writer, mcpResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]any{
					"protocolVersion": protocolVersion,
					"capabilities": map[string]any{
						"resources": map[string]any{},
						"tools":     map[string]any{},
					},
					"serverInfo": map[string]any{
						"name":    "kilnx-mcp",
						"version": "0.1.0",
					},
				},
			})

		case "ping":
			writeMessage(writer, mcpResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  map[string]any{},
			})

		case "resources/list":
			writeMessage(writer, mcpResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  map[string]any{"resources": resourceList},
			})

		case "resources/read":
			var params struct {
				URI string `json:"uri"`
			}
			if err := json.Unmarshal(req.Params, &params); err != nil || params.URI == "" {
				writeError(writer, req.ID, -32602, "invalid params")
				continue
			}
			text := resourceContent(params.URI)
			if text == "" {
				writeError(writer, req.ID, -32602, "resource not found: "+params.URI)
				continue
			}
			writeMessage(writer, mcpResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]any{
					"contents": []map[string]any{
						{"uri": params.URI, "mimeType": "text/plain", "text": text},
					},
				},
			})

		case "tools/list":
			writeMessage(writer, mcpResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  map[string]any{"tools": toolList},
			})

		case "tools/call":
			var params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			if err := json.Unmarshal(req.Params, &params); err != nil {
				writeError(writer, req.ID, -32602, "invalid params")
				continue
			}
			result, isErr := callTool(params.Name, params.Arguments)
			writeMessage(writer, mcpResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]any{
					"content": []map[string]any{
						{"type": "text", "text": result},
					},
					"isError": isErr,
				},
			})

		default:
			writeError(writer, req.ID, -32601, "method not found: "+req.Method)
		}
	}
}

func writeError(w io.Writer, id json.RawMessage, code int, msg string) {
	writeMessage(w, mcpResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &mcpError{Code: code, Message: msg},
	})
}

// Resources

var resourceList = []map[string]any{
	{
		"uri":         "kilnx://quickref",
		"name":        "Quick Reference",
		"description": "Kilnx language overview, CLI commands, and links to full docs",
		"mimeType":    "text/plain",
	},
	{
		"uri":         "kilnx://keywords",
		"name":        "Keyword Reference",
		"description": "All Kilnx keywords with descriptions and usage notes",
		"mimeType":    "text/plain",
	},
	{
		"uri":         "kilnx://grammar-summary",
		"name":        "Grammar Summary",
		"description": "Concise syntax reference for all Kilnx constructs",
		"mimeType":    "text/plain",
	},
	{
		"uri":         "kilnx://examples",
		"name":        "Code Examples",
		"description": "Working Kilnx code examples for common patterns",
		"mimeType":    "text/plain",
	},
}

func resourceContent(uri string) string {
	switch uri {
	case "kilnx://quickref":
		return quickRef
	case "kilnx://keywords":
		return buildKeywordRef()
	case "kilnx://grammar-summary":
		return grammarSummary
	case "kilnx://examples":
		return codeExamples
	}
	return ""
}

// Tools

var toolList = []map[string]any{
	{
		"name":        "check",
		"description": "Validate a Kilnx source string using the static analyzer. Returns diagnostics (errors and warnings) or confirms no issues.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"source": map[string]any{
					"type":        "string",
					"description": "Kilnx source code to validate",
				},
			},
			"required": []string{"source"},
		},
	},
	{
		"name":        "keyword_info",
		"description": "Look up documentation for a specific Kilnx keyword.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keyword": map[string]any{
					"type":        "string",
					"description": "The keyword to look up (e.g. 'page', 'action', 'model')",
				},
			},
			"required": []string{"keyword"},
		},
	},
}

func callTool(name string, args map[string]any) (result string, isError bool) {
	switch name {
	case "check":
		source, _ := args["source"].(string)
		if source == "" {
			return "Error: 'source' argument is required", true
		}
		return checkSource(source)

	case "keyword_info":
		keyword, _ := args["keyword"].(string)
		if keyword == "" {
			return "Error: 'keyword' argument is required", true
		}
		if doc, ok := keywordDocs[keyword]; ok {
			return fmt.Sprintf("%s\n\n%s", keyword, doc), false
		}
		return fmt.Sprintf("Unknown keyword: %s\n\nTop-level blocks: config, model, auth, permissions, layout, page, action, fragment, api, stream, socket, webhook, schedule, job, test\nBody keywords: query, validate, redirect, html, send, enqueue, broadcast, on, requires, fetch", keyword), false

	default:
		return "Unknown tool: " + name, true
	}
}

// checkSource runs kilnx check on the provided source in an isolated subprocess.
// This avoids os.Exit calls in the parser (e.g. missing required env vars) from
// killing the MCP server process. Env vars referenced in the source with
// "env VARNAME" patterns are injected with dummy values if not already set,
// so static analysis can run without needing real secrets.
func checkSource(source string) (string, bool) {
	tmp, err := os.CreateTemp("", "kilnx-mcp-check-*.kilnx")
	if err != nil {
		return "Error: could not create temp file: " + err.Error(), true
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(source); err != nil {
		tmp.Close()
		return "Error: could not write temp file: " + err.Error(), true
	}
	tmp.Close()

	self, err := os.Executable()
	if err != nil {
		return "Error: could not locate kilnx binary: " + err.Error(), true
	}

	env := os.Environ()
	for _, varName := range extractEnvVars(source) {
		if os.Getenv(varName) == "" {
			env = append(env, varName+"=mcp-check-dummy")
		}
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(self, "check", tmp.Name())
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		combined := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
		if combined == "" || combined == "\n" {
			combined = err.Error()
		}
		return strings.TrimSpace(combined), true
	}

	out := strings.TrimSpace(stdout.String())
	if out == "" {
		out = "No issues found."
	}
	return out, false
}

// extractEnvVars scans source for "env VARNAME" patterns and returns the var names.
func extractEnvVars(source string) []string {
	var vars []string
	seen := map[string]bool{}
	for _, line := range strings.Split(source, "\n") {
		fields := strings.Fields(line)
		for i, f := range fields {
			if f == "env" && i+1 < len(fields) {
				name := fields[i+1]
				if !seen[name] {
					vars = append(vars, name)
					seen[name] = true
				}
			}
		}
	}
	return vars
}

func buildKeywordRef() string {
	var sb strings.Builder
	sb.WriteString("# Kilnx Keyword Reference\n\n")

	sb.WriteString("## Top-level blocks\n\n")
	for kw, doc := range keywordDocs {
		// Only top-level blocks in this section
		isTopLevel := false
		for _, tl := range topLevelKeywords {
			if tl == kw {
				isTopLevel = true
				break
			}
		}
		if isTopLevel {
			sb.WriteString(fmt.Sprintf("### %s\n%s\n\n", kw, doc))
		}
	}

	sb.WriteString("## Body keywords\n\n")
	for kw, doc := range keywordDocs {
		isBody := false
		for _, b := range bodyKeywords {
			if b == kw {
				isBody = true
				break
			}
		}
		if isBody {
			sb.WriteString(fmt.Sprintf("### %s\n%s\n\n", kw, doc))
		}
	}

	sb.WriteString("## Field types\n\n")
	for _, ft := range fieldTypeNames {
		if doc, ok := keywordDocs[ft]; ok {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", ft, doc))
		}
	}

	sb.WriteString("\n## Constraints\n\nrequired, unique, default, auto, min, max\n")
	return sb.String()
}

// JSON-RPC transport (identical pattern to internal/lsp)

func readMessage(r *bufio.Reader) ([]byte, error) {
	contentLength := 0
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			fmt.Sscanf(strings.TrimPrefix(line, "Content-Length:"), "%d", &contentLength)
		}
	}
	if contentLength == 0 {
		return nil, fmt.Errorf("no content length")
	}
	body := make([]byte, contentLength)
	_, err := io.ReadFull(r, body)
	return body, err
}

func writeMessage(w io.Writer, msg any) {
	body, err := json.Marshal(msg)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(body))
	w.Write(body)
}

// Keyword data

var topLevelKeywords = []string{
	"config", "model", "auth", "permissions", "layout", "page", "action",
	"fragment", "api", "stream", "socket", "webhook", "schedule", "job", "test",
}

var bodyKeywords = []string{
	"query", "validate", "redirect", "html", "send", "enqueue", "broadcast",
	"on", "requires", "fetch", "respond",
}

var fieldTypeNames = []string{
	"text", "email", "int", "float", "bool", "timestamp", "richtext",
	"option", "password", "image", "phone",
}

var keywordDocs = map[string]string{
	"config":       "Application configuration block. Sets port, database path, app name, and secret environment variables.\n\nExample:\n  config\n    database: env DATABASE_URL default \"sqlite://app.db\"\n    port: env PORT default 8080\n    secret: env SECRET_KEY required",
	"model":        "Defines a data model that maps to a database table. Fields specify columns with types and constraints. Auto-migrated on startup.\n\nExample:\n  model user\n    name: text required min 2 max 100\n    email: email unique\n    created: timestamp auto",
	"auth":         "Configures authentication. Specifies the user table, identity field, password field, and login/redirect paths. Enables auto-generated login/register/logout routes.\n\nExample:\n  auth\n    table: user\n    identity: email\n    password: password\n    login: /login\n    after login: /dashboard",
	"permissions":  "Defines role-based access control rules. Each role lists what it can read/write. Supports ownership checks with `where field = current_user`.\n\nExample:\n  permissions\n    admin can read write user\n    user can read write post where author = current_user",
	"layout":       "Defines a page wrapper template. Use `{page.title}`, `{page.content}`, and `{kilnx.js}` as placeholders.\n\nExample:\n  layout main\n    html\n      <html><body>{page.content}</body></html>",
	"page":         "Declares a GET route that returns a full HTML page. Can specify layout, title, and auth requirements.\n\nExample:\n  page /dashboard requires auth\n    layout main\n    title Dashboard\n    query stats: SELECT count(*) as total FROM user\n    html\n      <h1>Total users: {stats.total}</h1>",
	"action":       "Declares a POST/PUT/DELETE route for data mutations. All queries within an action run in an implicit transaction.\n\nExample:\n  action /post/create POST requires auth\n    validate post\n    query: INSERT INTO post (title, body, author_id) VALUES (:title, :body, :current_user)\n    redirect /dashboard",
	"fragment":     "Declares a partial HTML endpoint (no page wrapper). Designed for htmx swap targets.\n\nExample:\n  fragment /users/list\n    query users: SELECT * FROM user ORDER BY created DESC\n    html\n      {{each users}}<div>{name}</div>{{end}}",
	"api":          "Declares a JSON endpoint. Same body grammar as pages but outputs JSON.\n\nExample:\n  api /api/users\n    query users: SELECT id, name, email FROM user\n    respond users",
	"stream":       "Declares a Server-Sent Events endpoint. Polls a SQL query at a specified interval.\n\nExample:\n  stream /events/users every 5s\n    query count: SELECT count(*) as total FROM user",
	"socket":       "Declares a WebSocket endpoint with on connect/message/disconnect handlers.",
	"webhook":      "Declares an external event receiver with HMAC signature verification.",
	"schedule":     "Declares a timed background task. Supports interval (every 5m) and cron (every monday at 9:00) expressions.\n\nExample:\n  schedule cleanup every 24h\n    query: DELETE FROM session WHERE expires < datetime('now')",
	"job":          "Declares an async background job. Enqueued via `enqueue`. Supports `retry N` for automatic retries with exponential backoff.\n\nExample:\n  job send-welcome\n    retry 3\n    query: SELECT email FROM user WHERE id = :user_id\n    send email to {user.email}",
	"test":         "Declares a test case with HTTP request/response assertions.\n\nExample:\n  test \"homepage returns 200\"\n    GET /\n    expect status 200",
	"query":        "Executes a SQL query. SELECT results are available for template interpolation. INSERT/UPDATE/DELETE are mutations. Named parameters use `:name` syntax.\n\nExample:\n  query posts: SELECT * FROM post WHERE author_id = :current_user ORDER BY created DESC",
	"validate":     "Validates form data against a model's constraints (required, email, min, max, unique).\n\nExample:\n  validate user",
	"redirect":     "Redirects to a path. Supports `:param` interpolation and htmx HX-Redirect.\n\nExample:\n  redirect /post/{post.id}",
	"html":         "Raw HTML block. Supports `{field}`, `{{each query}}...{{end}}`, `{{if expr}}...{{end}}`, and pipe filters like `{name | upper}`, `{date | timeago}`.",
	"send":         "Sends an email asynchronously. Syntax: `send email to :recipient`.",
	"enqueue":      "Dispatches an async job to the persistent job queue.\n\nExample:\n  enqueue send-welcome user_id={new_user.id}",
	"broadcast":    "Sends a message to all WebSocket clients in a room.",
	"on":           "Conditional branch based on last query result: success, error, not found, forbidden.\n\nExample:\n  on success: redirect /dashboard\n  on error: redirect /form?error=1",
	"requires":     "Requires authentication (`requires auth`) or a specific role (`requires admin`).",
	"fetch":        "Makes an HTTP request to an external API. Syntax: `fetch name: GET/POST url`. Supports `header` and `body` blocks. Results available via `{name.field}` in templates.",
	"respond":      "Return HTTP response with a specific status code or data.\n\nExample:\n  respond 404\n  respond users",
	"text":         "Text string type. Maps to TEXT in SQLite/PostgreSQL.",
	"email":        "Email type with built-in format validation. Maps to TEXT.",
	"bool":         "Boolean type. Maps to INTEGER (0/1) in SQLite, BOOLEAN in PostgreSQL.",
	"timestamp":    "Date/time type. Use `auto` for auto-generated timestamps. Maps to DATETIME/TIMESTAMPTZ.",
	"int":          "Integer type. Maps to INTEGER.",
	"float":        "Floating point type. Maps to REAL/DOUBLE PRECISION.",
	"password":     "Password type. Automatically hashed with bcrypt on INSERT. Never stored in plaintext.",
	"richtext":     "Rich text type. Content is rendered without HTML escaping — use only with trusted input.",
	"option":       "Enumeration type. Syntax: `field: option [value1, value2, value3]`.",
	"image":        "Image file path. Used with upload handling.",
	"phone":        "Phone number field with format validation.",
	"required":     "Field constraint: value must be non-null.",
	"unique":       "Field constraint: value must be unique across all rows.",
	"default":      "Field constraint: provides a default value if none specified. Example: `status: text default \"active\"`.",
	"auto":         "Field constraint: auto-generated value. On timestamp fields: current time on INSERT. On text fields with `default`: uses the default.",
	"min":          "Field constraint: minimum value or length. Example: `name: text required min 2`.",
	"max":          "Field constraint: maximum value or length. Example: `bio: text max 500`.",
}

const quickRef = `# Kilnx

Kilnx is a declarative backend language that compiles .kilnx source files to a single standalone binary. Models, routes, queries, auth, jobs, WebSockets, SSE, and tests live in one file. SQLite and PostgreSQL are first-class. Zero JavaScript, zero runtime dependencies for the user, built for the htmx era.

## Language constructs

Top-level blocks: config, model, auth, permissions, layout, page, action, fragment, stream, socket, api, webhook, job, schedule, test

Field types: text, email, int, float, bool, timestamp, richtext, option, password, image, phone

Constraints: required, unique, default, auto, min, max

Body keywords: query, validate, redirect, html, send, enqueue, broadcast, on, requires, fetch, respond

## CLI

- kilnx run <file.kilnx>     — dev server with hot reload
- kilnx build <file.kilnx>   — compile to standalone binary
- kilnx check <file.kilnx>   — static analysis (types, security, SQL)
- kilnx test <file.kilnx>    — run declarative test blocks
- kilnx migrate <file.kilnx> — schema migration
- kilnx lsp                  — Language Server Protocol endpoint
- kilnx mcp                  — Model Context Protocol server

## Security (built-in, do not reimplement)

CSRF protection, bcrypt password hashing, session HMAC, HTML escaping, SQL parameterized binding.

## Core references

- Grammar: https://github.com/kilnx-org/kilnx/blob/main/GRAMMAR.md
- Features: https://github.com/kilnx-org/kilnx/blob/main/FEATURES.md
- Principles: https://github.com/kilnx-org/kilnx/blob/main/PRINCIPLES.md
- Examples: https://github.com/kilnx-org/examples
`

const grammarSummary = `# Kilnx Grammar Summary

## config block

  config
    database: env DATABASE_URL default "sqlite://app.db"
    port: env PORT default 8080
    secret: env SECRET_KEY required
    name: "My App"

## model block

  model <name> [tenant <model>]
    <field>: <type> [constraints...]
    custom fields from "<path>.kilnx"

  Field types: text, email, int, float, bool, timestamp, richtext, option, password, image, phone
  Constraints: required, unique, default <val>, auto, min <n>, max <n>

## auth block

  auth
    table: <model>
    identity: <field>
    password: <field>
    login: <path>
    register: <path>        (optional, defaults to /register)
    logout: <path>          (optional, defaults to /logout)
    after login: <path>
    after register: <path>  (optional)

## permissions block

  permissions
    <role> can read write <model>
    <role> can read <model> where <field> = current_user

## layout block

  layout <name>
    html
      <!DOCTYPE html>
      <html>
        <head><title>{page.title}</title>{kilnx.js}</head>
        <body>{page.content}</body>
      </html>

## page block

  page <path> [requires auth|<role>]
    layout <name>
    title <text>
    query <name>: SELECT ...
    fetch <name>: GET/POST <url>
    html
      <template with {field} interpolations>

## action block

  action <path> [POST|PUT|DELETE] [requires auth|<role>]
    validate <model>
    query: INSERT/UPDATE/DELETE ...
    on success: redirect <path>
    on error: redirect <path>?error=1
    enqueue <job-name> <key>=<val>

## fragment block

  fragment <path>
    query <name>: SELECT ...
    html
      <partial HTML>

## api block

  api <path> [requires auth|<role>]
    query <name>: SELECT ...
    respond <name>

## stream block

  stream <path> every <interval>
    query <name>: SELECT ...

## socket block

  socket <path>
    on connect
      broadcast to room <expr>: <message>
    on message
      query: INSERT ...
    on disconnect
      query: DELETE ...

## job block

  job <name>
    retry <n>
    query: SELECT ...
    send email to {field}

## schedule block

  schedule <name> every <interval>|every <day> at <time>
    query: DELETE FROM ...

## template syntax (inside html blocks)

  {field}                     — interpolate value (HTML-escaped)
  {query.field}               — interpolate query result field
  {field | filter}            — apply filter (upper, lower, date, timeago, truncate, currency)
  {{each query}}<p>{name}</p>{{end}}   — iterate over query results
  {{if field}}<p>shown</p>{{end}}      — conditional block
  {{if !field}}<p>shown</p>{{end}}     — negated conditional

## SQL rules

  query name: SELECT * FROM model WHERE id = :param
  query: INSERT INTO model (field) VALUES (:form_field)

  - Named parameters use :name syntax (bound, never concatenated)
  - :current_user resolves to the authenticated user's ID
  - Tables map 1:1 to model names (lowercase)
  - Multi-query in same block: each named separately
`

const codeExamples = `# Kilnx Code Examples

## Minimal hello world

  config
    database: env DATABASE_URL default "sqlite://app.db"
    port: 8080
    secret: env SECRET_KEY required

  page /
    html
      <h1>Hello, world!</h1>

## User auth + dashboard

  config
    database: env DATABASE_URL default "sqlite://app.db"
    port: env PORT default 8080
    secret: env SECRET_KEY required

  model user
    name: text required min 2 max 100
    email: email unique required
    password: password required
    created: timestamp auto

  auth
    table: user
    identity: email
    password: password
    login: /login
    after login: /dashboard

  page /dashboard requires auth
    query stats: SELECT count(*) as total FROM user
    html
      <h1>Dashboard</h1>
      <p>Total users: {stats.total}</p>

## CRUD with validation

  model post
    title: text required min 3 max 200
    body: richtext required
    author_id: int required
    created: timestamp auto

  page /posts
    query posts: SELECT p.id, p.title, u.name as author FROM post p JOIN user u ON u.id = p.author_id ORDER BY p.created DESC
    html
      {{each posts}}
        <article>
          <h2>{title}</h2>
          <p>by {author}</p>
        </article>
      {{end}}

  action /posts POST requires auth
    validate post
    query: INSERT INTO post (title, body, author_id) VALUES (:title, :body, :current_user)
    on success: redirect /posts
    on error: redirect /posts/new?error=1

## Server-Sent Events

  stream /events/messages every 2s
    query msgs: SELECT id, body, created FROM message ORDER BY created DESC LIMIT 20

## Background job with retry

  job send-welcome
    retry 3
    query u: SELECT email, name FROM user WHERE id = :user_id
    send email to {u.email}

  action /register POST
    validate user
    query: INSERT INTO user (name, email, password) VALUES (:name, :email, :password)
    enqueue send-welcome user_id={new_user.id}
    redirect /dashboard

## JSON API

  api /api/v1/posts requires auth
    query posts: SELECT id, title, created FROM post WHERE author_id = :current_user
    respond posts

## htmx fragment swap

  page /contacts
    query contacts: SELECT * FROM contact ORDER BY name
    html
      <input hx-get="/contacts/search" hx-target="#list" hx-trigger="input">
      <div id="list">
        {{each contacts}}<div>{name} — {email}</div>{{end}}
      </div>

  fragment /contacts/search
    query contacts: SELECT * FROM contact WHERE name LIKE :q OR email LIKE :q ORDER BY name
    html
      {{each contacts}}<div>{name} — {email}</div>{{end}}
`
