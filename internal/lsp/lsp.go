package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/kilnx-org/kilnx/internal/analyzer"
	"github.com/kilnx-org/kilnx/internal/lexer"
	"github.com/kilnx-org/kilnx/internal/parser"
)

// Serve starts the LSP server on stdin/stdout
func Serve() {
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	files := make(map[string]string) // uri -> content

	for {
		msg, err := readMessage(reader)
		if err != nil {
			if err == io.EOF {
				return
			}
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			continue
		}

		switch req.Method {
		case "initialize":
			resp := jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: initializeResult{
					Capabilities: serverCapabilities{
						TextDocumentSync: 1, // Full sync
						CompletionProvider: &completionOptions{
							TriggerCharacters: []string{" ", "\n"},
						},
						HoverProvider: true,
					},
				},
			}
			writeMessage(writer, resp)

		case "initialized":
			// no-op

		case "textDocument/didOpen":
			var params didOpenParams
			if json.Unmarshal(req.Params, &params) == nil {
				files[params.TextDocument.URI] = params.TextDocument.Text
				publishDiagnostics(writer, params.TextDocument.URI, params.TextDocument.Text)
			}

		case "textDocument/didChange":
			var params didChangeParams
			if json.Unmarshal(req.Params, &params) == nil {
				if len(params.ContentChanges) > 0 {
					files[params.TextDocument.URI] = params.ContentChanges[0].Text
					publishDiagnostics(writer, params.TextDocument.URI, params.ContentChanges[0].Text)
				}
			}

		case "textDocument/didSave":
			var params didSaveParams
			if json.Unmarshal(req.Params, &params) == nil {
				if content, ok := files[params.TextDocument.URI]; ok {
					publishDiagnostics(writer, params.TextDocument.URI, content)
				}
			}

		case "textDocument/completion":
			var params completionParams
			if json.Unmarshal(req.Params, &params) == nil {
				items := getCompletions(files[params.TextDocument.URI], params.Position)
				resp := jsonRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result:  completionList{Items: items},
				}
				writeMessage(writer, resp)
			}

		case "textDocument/hover":
			var params hoverParams
			if json.Unmarshal(req.Params, &params) == nil {
				result := getHover(files[params.TextDocument.URI], params.Position)
				resp := jsonRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result:  result,
				}
				writeMessage(writer, resp)
			}

		case "shutdown":
			resp := jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: nil}
			writeMessage(writer, resp)

		case "exit":
			os.Exit(0)
		}
	}
}

func publishDiagnostics(w io.Writer, uri string, content string) {
	stripped := lexer.StripComments(content)
	tokens := lexer.Tokenize(stripped)
	app, parseErr := parser.Parse(tokens, stripped)

	var diags []lspDiagnostic

	// Collect parse errors
	if parseErr != nil {
		for _, line := range strings.Split(parseErr.Error(), "\n") {
			d := lspDiagnostic{
				Range:    rangeFromErrorLine(line),
				Severity: 1, // Error
				Source:   "kilnx",
				Message:  line,
			}
			diags = append(diags, d)
		}
	}

	// Run static analyzer if parse succeeded (even partially)
	if app != nil {
		for _, ad := range analyzer.Analyze(app) {
			severity := 2 // Warning
			if ad.Level == "error" {
				severity = 1
			}
			line := ad.Line
			if line > 0 {
				line-- // LSP is 0-indexed
			}
			diags = append(diags, lspDiagnostic{
				Range: lspRange{
					Start: position{Line: line, Character: 0},
					End:   position{Line: line, Character: 999},
				},
				Severity: severity,
				Source:   "kilnx",
				Message:  fmt.Sprintf("[%s] %s", ad.Context, ad.Message),
			})
		}
	}

	notification := jsonRPCNotification{
		JSONRPC: "2.0",
		Method:  "textDocument/publishDiagnostics",
		Params: publishDiagnosticsParams{
			URI:         uri,
			Diagnostics: diags,
		},
	}
	writeMessage(w, notification)
}

// rangeFromErrorLine tries to extract a line number from "line N: ..." format
func rangeFromErrorLine(msg string) lspRange {
	line := 0
	if idx := strings.Index(msg, "line "); idx >= 0 {
		rest := msg[idx+5:]
		if colonIdx := strings.Index(rest, ":"); colonIdx > 0 {
			if n, err := strconv.Atoi(rest[:colonIdx]); err == nil && n > 0 {
				line = n - 1 // LSP is 0-indexed
			}
		}
	}
	return lspRange{
		Start: position{Line: line, Character: 0},
		End:   position{Line: line, Character: 999},
	}
}

func getCompletions(content string, pos position) []completionItem {
	// Determine context: what indent level are we at?
	lines := strings.Split(content, "\n")
	currentLine := ""
	if pos.Line < len(lines) {
		currentLine = lines[pos.Line]
	}

	indent := 0
	for _, c := range currentLine {
		if c == ' ' {
			indent++
		} else if c == '\t' {
			indent += 2
		} else {
			break
		}
	}

	var items []completionItem

	if indent == 0 {
		// Top-level keywords
		for _, kw := range topLevelKeywords {
			items = append(items, completionItem{
				Label:  kw.Name,
				Kind:   14, // Keyword
				Detail: kw.Detail,
			})
		}
	} else {
		// Inside a block: suggest body keywords and field types
		for _, kw := range bodyKeywords {
			items = append(items, completionItem{
				Label:  kw.Name,
				Kind:   14,
				Detail: kw.Detail,
			})
		}
		for _, ft := range fieldTypes {
			items = append(items, completionItem{
				Label:  ft.Name,
				Kind:   12, // Value
				Detail: ft.Detail,
			})
		}
	}

	return items
}

func getHover(content string, pos position) *hoverResult {
	lines := strings.Split(content, "\n")
	if pos.Line >= len(lines) {
		return nil
	}
	line := lines[pos.Line]
	word := wordAt(line, pos.Character)
	if word == "" {
		return nil
	}

	if desc, ok := keywordDocs[word]; ok {
		return &hoverResult{
			Contents: markupContent{
				Kind:  "markdown",
				Value: fmt.Sprintf("**%s**\n\n%s", word, desc),
			},
		}
	}

	return nil
}

func wordAt(line string, col int) string {
	if col >= len(line) {
		col = len(line) - 1
	}
	if col < 0 {
		return ""
	}

	start := col
	for start > 0 && isWordChar(line[start-1]) {
		start--
	}
	end := col
	for end < len(line) && isWordChar(line[end]) {
		end++
	}
	if start >= end {
		return ""
	}
	return line[start:end]
}

func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// JSON-RPC message reading/writing

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

func writeMessage(w io.Writer, msg interface{}) {
	body, err := json.Marshal(msg)
	if err != nil {
		return
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	w.Write([]byte(header))
	w.Write(body)
}

// LSP protocol types (minimal subset)

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{}     `json:"result"`
}

type jsonRPCNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type lspRange struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

type textDocumentIdentifier struct {
	URI string `json:"uri"`
}

type textDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type didOpenParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type didChangeParams struct {
	TextDocument   textDocumentIdentifier `json:"textDocument"`
	ContentChanges []struct {
		Text string `json:"text"`
	} `json:"contentChanges"`
}

type didSaveParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type completionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
}

type hoverParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
}

type initializeResult struct {
	Capabilities serverCapabilities `json:"capabilities"`
}

type serverCapabilities struct {
	TextDocumentSync   int                `json:"textDocumentSync"`
	CompletionProvider *completionOptions `json:"completionProvider,omitempty"`
	HoverProvider      bool               `json:"hoverProvider"`
}

type completionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

type completionList struct {
	Items []completionItem `json:"items"`
}

type completionItem struct {
	Label  string `json:"label"`
	Kind   int    `json:"kind"`
	Detail string `json:"detail,omitempty"`
}

type hoverResult struct {
	Contents markupContent `json:"contents"`
}

type markupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type lspDiagnostic struct {
	Range    lspRange `json:"range"`
	Severity int      `json:"severity"`
	Source   string   `json:"source"`
	Message  string   `json:"message"`
}

type publishDiagnosticsParams struct {
	URI         string          `json:"uri"`
	Diagnostics []lspDiagnostic `json:"diagnostics"`
}

// Completion data

type keywordInfo struct {
	Name   string
	Detail string
}

var topLevelKeywords = []keywordInfo{
	{"config", "App configuration (port, database, secrets)"},
	{"model", "Define a data model (generates database table)"},
	{"auth", "Authentication configuration"},
	{"permissions", "Role-based access control rules"},
	{"layout", "Page wrapper template"},
	{"page", "GET route returning full HTML page"},
	{"action", "POST/PUT/DELETE route for mutations"},
	{"fragment", "Partial HTML response (for htmx swaps)"},
	{"api", "JSON endpoint"},
	{"stream", "Server-Sent Events endpoint"},
	{"socket", "Bidirectional WebSocket endpoint"},
	{"webhook", "External event receiver"},
	{"schedule", "Timed background task"},
	{"job", "Async background job"},
	{"queries", "Named reusable SQL queries"},
	{"test", "Declarative test case"},
	{"translations", "i18n translation strings"},
	{"log", "Logging configuration"},
	{"limit", "Rate limiting rules"},
}

var bodyKeywords = []keywordInfo{
	{"query", "Execute a SQL query"},
	{"validate", "Validate form data against model"},
	{"redirect", "HTTP redirect to path"},
	{"html", "Raw HTML block with template directives"},
	{"send", "Send email (send email to :recipient)"},
	{"enqueue", "Dispatch async job"},
	{"broadcast", "Send to WebSocket room"},
	{"on", "Conditional branch (success/error/not found)"},
	{"respond", "Return HTTP response with status"},
	{"requires", "Require authentication or role"},
	{"method", "HTTP method (POST, PUT, DELETE)"},
	{"title", "Page title"},
	{"retry", "Job retry count"},
}

var fieldTypes = []keywordInfo{
	{"text", "Text string field"},
	{"email", "Email field with validation"},
	{"bool", "Boolean field"},
	{"timestamp", "Date/time field"},
	{"richtext", "Rich text (unescaped HTML)"},
	{"option", "Enumeration (option [a, b, c])"},
	{"int", "Integer field"},
	{"float", "Floating point field"},
	{"password", "Password field (auto-hashed with bcrypt)"},
	{"image", "Image file path"},
	{"phone", "Phone number field"},
	{"required", "Field constraint: non-null"},
	{"unique", "Field constraint: unique value"},
	{"default", "Field constraint: default value"},
	{"auto", "Field constraint: auto-generated"},
	{"min", "Field constraint: minimum value"},
	{"max", "Field constraint: maximum value"},
}

var keywordDocs = map[string]string{
	"config":       "Application configuration block. Sets port, database path, app name, and secret environment variables.",
	"model":        "Defines a data model that maps to a SQLite table. Fields specify columns with types and constraints. Auto-migrated on startup.",
	"auth":         "Configures authentication. Specifies the user table, identity field, password field, and login/redirect paths. Enables auto-generated login/register/logout routes.",
	"permissions":  "Defines role-based access control rules. Each role lists what it can read/write. Supports ownership checks with `where field = current_user`.",
	"layout":       "Defines a page wrapper template. Use `{page.title}`, `{page.content}`, and `{kilnx.js}` as placeholders.",
	"page":         "Declares a GET route that returns a full HTML page. Can specify layout, title, and auth requirements.",
	"action":       "Declares a POST/PUT/DELETE route for data mutations. All queries within an action run in an implicit transaction.",
	"fragment":     "Declares a partial HTML endpoint (no page wrapper). Designed for htmx swap targets.",
	"api":          "Declares a JSON endpoint. Same body grammar as pages but outputs JSON.",
	"stream":       "Declares a Server-Sent Events endpoint. Polls a SQL query at a specified interval.",
	"socket":       "Declares a WebSocket endpoint with on connect/message/disconnect handlers.",
	"webhook":      "Declares an external event receiver with HMAC signature verification.",
	"schedule":     "Declares a timed background task. Supports interval (every 5m) and cron (every monday at 9:00) expressions.",
	"job":          "Declares an async background job. Enqueued via `enqueue`. Supports `retry N` for automatic retries with exponential backoff.",
	"query":        "Executes a SQL query. SELECT results are available for template interpolation. INSERT/UPDATE/DELETE are mutations.",
	"validate":     "Validates form data against a model's constraints (required, email, min, max, unique).",
	"redirect":     "Redirects to a path. Supports `:param` interpolation and htmx HX-Redirect.",
	"html":         "Raw HTML block. Supports `{field}`, `{{each query}}`, `{{if expr}}`, and pipe filters.",
	"send":         "Sends an email asynchronously. Syntax: `send email to :recipient`.",
	"enqueue":      "Dispatches an async job to the persistent job queue.",
	"broadcast":    "Sends a message to all WebSocket clients in a room.",
	"on":           "Conditional branch based on last query result: success, error, not found, forbidden.",
	"requires":     "Requires authentication (`requires auth`) or a specific role (`requires admin`).",
	"text":         "Text string type. Maps to SQLite TEXT.",
	"email":        "Email type with built-in validation. Maps to SQLite TEXT.",
	"bool":         "Boolean type. Maps to SQLite INTEGER (0/1).",
	"timestamp":    "Date/time type. Maps to SQLite DATETIME. Use `auto` for auto-generated timestamps.",
	"int":          "Integer type. Maps to SQLite INTEGER.",
	"float":        "Floating point type. Maps to SQLite REAL.",
	"password":     "Password type. Automatically hashed with bcrypt on INSERT.",
	"required":     "Field constraint: value must be non-null.",
	"unique":       "Field constraint: value must be unique across all rows.",
	"optional":     "Field constraint: value can be null.",
	"default":      "Field constraint: provides a default value if none specified.",
	"retry":        "Job retry count. Syntax: `retry N`. Failed jobs retry with exponential backoff.",
	"translations": "Defines i18n translation strings. Use `{t.key}` in templates.",
	"limit":        "Defines rate limiting rules per path pattern. Supports per-user and per-IP limits.",
}
