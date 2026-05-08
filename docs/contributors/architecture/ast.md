# AST

The parser produces a single `*parser.App` value rooted in [`internal/parser/parser.go`](../../../internal/parser/parser.go). Every later stage (analyzer, optimizer, runtime, build) consumes that struct.

## App root

[`parser.App`](../../../internal/parser/parser.go) holds slices and pointers per top-level declaration:

| Field | Source keyword | Notes |
|-------|----------------|-------|
| `Models` | `model` | Tables, fields, indexes, custom field manifests, tenant scoping. |
| `Pages` | `page` | GET routes that render HTML. |
| `Actions` | `action` | POST/PUT/DELETE handlers. Same `Page` shape as pages. |
| `Fragments` | `fragment` | Partial HTML responses. Path-based or component-style. |
| `APIs` | `api` | JSON endpoints. |
| `Streams` | `stream` | SSE endpoints driven by a polling SQL query. |
| `Schedules` | `schedule` | Recurring tasks (interval or cron-like phrase). |
| `Jobs` | `job` | Async background units invoked by `enqueue`. |
| `Webhooks` | `webhook` | Signed HTTP receivers with `on <event>` dispatch. |
| `Sockets` | `socket` | Bidirectional WebSocket endpoints. |
| `RateLimits` | `limit` | Path-pattern throttles. |
| `Config` | `config` | App-level settings (`*AppConfig`). |
| `Auth` | `auth` | Built-in auth wiring (`*AuthConfig`). |
| `Permissions` | `permissions` | Role-based access rules. |
| `Layouts` | `layout` | Named HTML shells with `{page.title}` and `{page.content}` slots. |
| `Tests` | `test` | Scripted end-to-end scenarios. |
| `LogConfig` | `log` | Logging level and request/error toggles. |
| `Translations` | `translations` | `lang -> key -> value`. |
| `NamedQueries` | top-level `query <name>` | `name -> SQL`. Resolved into `Page.Body` after parse. |
| `CustomManifests` | `*_fields.kilnx` files referenced by `custom` | Runtime-extensible fields per model. |

Pages, actions, fragments, and APIs all share the [`parser.Page`](../../../internal/parser/parser.go) struct. They are distinguished by which slice they live on and which fields they use (`Method`, `FragmentArgs`, etc.).

## Top-level dispatch

The parse loop is a switch over the leading keyword of each top-level statement. See [`parser.Parse`](../../../internal/parser/parser.go) for the canonical list. Each case calls a `parseX` method that consumes its block and appends to the matching `App` slice. Errors are accumulated, then `synchronize()` skips to the next plausible top-level boundary so multiple errors surface in one pass.

## `*_spec.go` files

Each top-level keyword has both:

- a `parseX` method on `parserState` that builds the AST node, and
- a `<name>_spec.go` file that registers a [`spec.Entity`](../../../internal/spec/spec.go) describing the keyword for documentation generation.

Examples:

- [`parser/model_spec.go`](../../../internal/parser/model_spec.go) registers the `model` keyword (summary, syntax, allowed children, examples).
- [`parser/page_spec.go`](../../../internal/parser/page_spec.go) registers `page`.
- [`parser/action_spec.go`](../../../internal/parser/action_spec.go), [`api_spec.go`](../../../internal/parser/api_spec.go), [`fragment_spec.go`](../../../internal/parser/fragment_spec.go), [`stream_spec.go`](../../../internal/parser/stream_spec.go), [`schedule_spec.go`](../../../internal/parser/schedule_spec.go), [`job_spec.go`](../../../internal/parser/job_spec.go), [`webhook_spec.go`](../../../internal/parser/webhook_spec.go), [`socket_spec.go`](../../../internal/parser/socket_spec.go), [`limit_spec.go`](../../../internal/parser/limit_spec.go), [`config_spec.go`](../../../internal/parser/config_spec.go), [`auth_spec.go`](../../../internal/parser/auth_spec.go), [`permissions_spec.go`](../../../internal/parser/permissions_spec.go), [`layout_spec.go`](../../../internal/parser/layout_spec.go), [`log_spec.go`](../../../internal/parser/log_spec.go), [`translations_spec.go`](../../../internal/parser/translations_spec.go), [`query_spec.go`](../../../internal/parser/query_spec.go), [`test_spec.go`](../../../internal/parser/test_spec.go) cover the rest.
- Attribute specs: [`attrs_spec.go`](../../../internal/parser/attrs_spec.go), [`field_attrs_spec.go`](../../../internal/parser/field_attrs_spec.go), [`body_nodes_spec.go`](../../../internal/parser/body_nodes_spec.go).

A `spec.Register` call in `init()` populates the global registry. `cmd/kilnx-gendocs` imports `internal/parser` for its side effects, then renders Markdown from the registry. Adding a new keyword therefore means: add the `parseX`, add the AST struct, add a `<name>_spec.go` registration, regenerate docs. See [spec-registry.md](spec-registry.md) for details.

## Body statements: `Node`

The body of a `page`, `action`, `fragment`, `api`, `schedule`, `job`, `webhook` event handler, or `socket` handler is `[]parser.Node`. `Node.Type` is a `NodeType` enum and selects which other fields are meaningful. The full list lives in [`parser.go`](../../../internal/parser/parser.go):

| `NodeType` | Keyword form | Active fields (selected) |
|------------|--------------|--------------------------|
| `NodeText` | template literal | `Value` |
| `NodeQuery` | `query [name]: <SQL>` | `Name`, `SQL`, `SourceModel`, `Paginate` |
| `NodeRedirect` | `redirect <path>` | `Value` |
| `NodeValidate` | `validate { ... }` | `ModelName`, `Validations` |
| `NodeRespond` | `respond <selector> [query: <SQL>]` | `RespondTarget`, `RespondSwap`, `QuerySQL`, `StatusCode` |
| `NodeHTML` | `html { ... }` | `HTMLContent` |
| `NodeSendEmail` | `send email to ...` | `EmailTo`, `EmailSubject`, `EmailTemplate`, `EmailAttach`, `Props` |
| `NodeEnqueue` | `enqueue <job>` | `JobName`, `JobParams` |
| `NodeOn` | `on success`, `on error`, `on not found` | `Props["condition"]`, `Children` |
| `NodeBroadcast` | `broadcast to <room>` | `BroadcastRoom`, `BroadcastFrag` |
| `NodeGeneratePDF` | `generate pdf from <template> data <query>` | `TemplateName`, `DataQueryName` |
| `NodeFetch` | `fetch <name>: <method> <url>` | `FetchURL`, `FetchMethod`, `FetchHeaders`, `FetchBody` |
| `NodeLLM` | `llm <name>: <model>` | `LLMModel`, `LLMSystem`, `LLMHistorySQL` |

`Node.Children` holds nested nodes for `on` branching. `Props` is a generic key-value bag used by nodes whose fields do not warrant first-class struct members.

## Models, fields, custom fields

[`parser.Model`](../../../internal/parser/parser.go) carries `Fields`, `UniqueConstraints`, `Indexes`, `Tenant`, and the `custom` manifest references (`CustomFieldsFile`, `CustomFieldsFallback`, `DynamicFields`). [`parser.Field`](../../../internal/parser/parser.go) is one column with `Type` (a [`FieldType`](../../../internal/parser/parser.go) like `text`, `email`, `int`, `reference`, `option`, `computed`, ...), `Required`, `Unique`, `Default`, `Min`, `Max`, `Options`, `Reference`, `Computed`, `ComputedExpr`. Custom fields use [`CustomFieldDef`](../../../internal/parser/parser.go) and [`CustomFieldManifest`](../../../internal/parser/parser.go) and feed into the analyzer and runtime separately from declared fields.

## Auth and access control

- [`parser.AuthConfig`](../../../internal/parser/parser.go): paths and field names for the built-in auth subsystem.
- [`parser.Permission`](../../../internal/parser/parser.go): role plus rule list (`all`, `read post`, `write post where author = current_user`, ...).
- [`parser.RequiresClause`](../../../internal/parser/parser.go) and `RequiresClauseKind`: the comma-separated AND-list attached to a `page`, `action`, `api`, `stream`, or `socket`. Kinds include `Auth`, `Role`, `Expr`, `Superuser`, `Flag`, `RateLimit`. The runtime evaluates these in [`auth.go`](../../../internal/runtime/auth.go).

## Notes for contributors

- Adding a top-level keyword: extend the dispatch in `parser.Parse`, add a struct on `App`, add a `parseX` method, register a `spec.Entity` in a new `<name>_spec.go`.
- Adding a body statement: add a `NodeType` constant, add the fields it needs to `Node`, parse it inside whichever block it belongs to, then teach analyzer, optimizer, and runtime to handle it.
- Resist adding cross-cutting fields to `Node`. Prefer `Props` for one-off keys, struct fields when the analyzer or runtime relies on them.
