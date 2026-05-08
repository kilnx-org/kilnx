# Runtime Execution

What happens between a TCP accept and a flushed response, inside [`internal/runtime`](../../../internal/runtime).

## Server boot

[`runtime.NewServer(app, db, port)`](../../../internal/runtime/server.go) wires the request-handling stack:

- `SessionStore` ([`auth.go`](../../../internal/runtime/auth.go)). HMAC-signed session cookies, in-memory cache plus SQLite persistence, 24h expiry, background cleanup. Falls back to a random secret with a warning when `config.secret` is empty.
- `JobQueue` ([`scheduler.go`](../../../internal/runtime/scheduler.go) and related). Background poller for `enqueue` work.
- `RateLimiter` ([`ratelimit.go`](../../../internal/runtime/ratelimit.go)). Built from `app.RateLimits`.
- `Logger` ([`logger.go`](../../../internal/runtime/logger.go)). Driven by `app.LogConfig`. Wired to `database.DB.OnSlowQuery` for slow query traces.
- `TenantMap` ([`tenant.go`](../../../internal/runtime/tenant.go)). Models with a `tenant: <model>` directive.
- `i18n` ([`i18n.go`](../../../internal/runtime/i18n.go)). Translation table plus optional Accept-Language detection.
- `fragmentComponents`. Index of component-style fragments (those with `FragmentArgs`), keyed by `Path`, used for inline rendering.
- `superuserIdentity`. From `auth.superuser`. Bypasses all role checks.

[`Server.Start`](../../../internal/runtime/server.go) builds the `http.ServeMux` via `buildHandler`, wraps it in `Logger.LoggingMiddleware`, and calls `http.ListenAndServe`. Hot reload swaps the parsed app under a write lock via [`Server.Reload`](../../../internal/runtime/server.go).

## Mux layout

Three permanent routes are registered at boot:

- `GET /_kilnx/htmx.min.js` and `GET /_kilnx/sse.js`. Embedded static assets via `embed.FS`.
- `/_uploads/`. File server over `config.uploads_dir` with `X-Content-Type-Options: nosniff` and `Content-Disposition: attachment`.
- `/_static/`. File server over `config.static_dir`. Resolved against absolute CWD; refused if it escapes CWD. `Cache-Control: no-store` when `KILNX_DEV=1`, otherwise `public, max-age=3600`.
- `GET /healthz`. Static `{"status":"ok"}` JSON.

Everything else is served by a single catch-all handler at `/` so route resolution honors hot reload.

## Request lifecycle

The catch-all in [`server.go`](../../../internal/runtime/server.go) walks routes in this fixed order. First match wins.

1. Snapshot app under read lock: `s.getApp()`.
2. Rate limit check: `RateLimiter.CheckWithRule`. On exceeded, write `429 Too Many Requests` with the rule's message and `Retry-After`.
3. Webhooks. POST endpoints that bypass CSRF and verify a payload signature against `webhook.SecretEnv`. Handled by `handleWebhook`.
4. WebSockets. Path-matched against `app.Sockets`. Handled by `handleSocket`.
5. Auth POST endpoints. When `app.Auth != nil` and method is POST, paths matching `LoginPath`, `LogoutPath`, `RegisterPath`, `ForgotPath`, `ResetPath` go to `handleLogin`, `handleLogout`, `handleRegister`, `handleForgotPassword`, `handleResetPassword`. The runtime owns POST. GET falls through to user-declared pages; the analyzer enforces that each auth path has a page.
6. Actions. POST, PUT, or DELETE. Path-matched against `app.Actions`. Declared `Method` is enforced. `requireAuth` runs first, then `handleAction`.
7. Streams. Path-matched against `app.Streams`. `handleStream` issues SSE.
8. APIs. Path-matched against `app.APIs`. `OPTIONS` always passes through for CORS preflight. Declared `Method` is enforced; mismatch defers a `405`. `requireAPIAuth` runs, then `handleAPI` returns JSON.
9. Fragments. Path-matched against `app.Fragments`, skipping component-style fragments (`FragmentArgs != nil`). `requireAuth`, then `renderFragment`, write `text/html`.
10. Pages. Path-matched against `app.Pages` (GET). `requireAuth`, then `renderPage`, write `text/html`.
11. Fall-through. `404` rendered via `render404`.

`matchPath` (in [`server.go`](../../../internal/runtime/server.go) helpers) handles literal segments and `:param` placeholders.

## Auth and access control

[`Server.requireAuth`](../../../internal/runtime/auth.go) gates pages, actions, and fragments. [`Server.requireAPIAuth`](../../../internal/runtime/auth.go) is the JSON twin (returns `401` or `403` JSON instead of an HTML redirect).

Decision tree for `requireAuth`:

- `page.Auth == false`. Allow.
- `app.Auth == nil`. Allow (no auth subsystem configured).
- No session. Redirect to `auth.login` with `?next=<path>`. For htmx (`HX-Request: true`), set `HX-Redirect` and `401`.
- `RequiresClauses` non-empty. Evaluate the AND of all clauses via `evalRequiresClauses`. Fail renders `403` via `renderForbidden`.
- Legacy `RequiresRole`. `auth` allows any logged-in user. A named role passes if `session.Role == role` or `hasPermission(session.Role, role, app.Permissions)` returns true. Otherwise `403`.

`RequiresClauseKind` covers `Auth`, `Role`, `Expr`, `Superuser`, `Flag`, and `RateLimit`. The expression form is evaluated by [`expr.go`](../../../internal/runtime/expr.go) against the current session.

## Permissions and SQL rewriting

`app.Permissions` is a list of `Permission{Role, Rules}`. [`permissions.go`](../../../internal/runtime/permissions.go) builds a `PermissionMap` and exposes:

- `CanAccess`, `CanRead`, `CanWrite`. Pre-flight checks per role and resource.
- `ConditionForRead`, `ConditionForWrite`. Returns the row-level filter clause from rules like `read post where author = current_user`.
- `RewritePermissionSQL`. Injects the per-role filter into a query as an extra `WHERE` predicate. Placeholder substitution is done by `resolvePermissionPlaceholders`.

Pages, actions, and APIs that issue queries get their SQL rewritten through this layer before it reaches `database.Executor`.

## Handler dispatch

- `handlePage` and `handleAction` walk the body `[]parser.Node` in order. Each `NodeType` has a runtime case. Queries hit the database via the executor; `validate` builds form rules from `Validations`; `respond`, `redirect`, `send email`, `enqueue`, `broadcast`, `fetch`, `llm`, `generate pdf`, `on success/error/not found` all flow from the same loop.
- `handleAction` enforces CSRF on the standard path. Webhooks and APIs are exempt by design (they have their own auth: signature verification or session/JSON 401).
- `handleAPI` returns JSON shaped from query results. CORS headers come from `config.cors_origins`.
- `handleStream` opens an SSE response and re-runs the stream's SQL on a tick.

## Render

[`renderPage`](../../../internal/runtime/server.go) builds a `renderContext` carrying:

- `queries map[string][]database.Row`. Materialized query results, keyed by query name.
- `paginate map[string]PaginateInfo`. Per-query pagination state.
- `currentUser *Session`, `queryParams`, `request`, `i18n`.
- `customManifests`. For list pages that bind to dynamic custom fields.
- `eachStack`, `eachModels`, `currentRow`. State for `{{each}}` iteration and parent-scope resolution with `{^field}`.
- `models`. All declared models, used to resolve computed fields at template time.
- `fragmentArgs`, `fragmentDepth`, `fragmentComponents`. Component-fragment inlining with a recursion guard.
- `actions`. All declared actions, for `action=` attribute expansion in HTML.

Rendering supports the directives documented at the top of [`render.go`](../../../internal/runtime/render.go):

- `{{each name}}...{{else}}...{{end}}`. Iterate query rows.
- `{{if expr}}...{{else}}...{{end}}`. Conditional block.
- `{name.field}`, `{name.count}`, `{field}`. Escaped value lookup.
- `{field | raw}`. Unescaped (used for richtext).
- `{field | filter1 | filter2: arg}`. Filter chain. Built-ins live in `builtinFilters` (`upcase`, `downcase`, `capitalize`, `truncate`, `default`, `raw`, ...).
- `{csrf}`. CSRF token, regenerated per occurrence.
- `{paginate.name.field}`. Pagination metadata.
- `{params.key}`. URL query parameters.

The page's content is then wrapped by its declared `Layout` (slots `{page.title}`, `{page.content}`, `{nav}`). Layout queries are merged into the same render context.

## Side effects in a request

- DB writes via `database.Executor` (transactional per-action when applicable).
- Session mutation through `SessionStore` (cookie set/clear, in-memory map, SQLite row).
- Job enqueue: `JobQueue.Enqueue`, picked up by the background poller.
- Outbound: `send email` (`email.go`), `fetch` (`fetch.go`, hardened against SSRF), `llm` (`llm.go`), `broadcast` (`websocket.go`), `generate pdf` (`internal/pdf` via `pdf.go`).

## Where to start reading

- Routing top-to-bottom: [`server.go`](../../../internal/runtime/server.go).
- Auth and sessions: [`auth.go`](../../../internal/runtime/auth.go).
- Templates and filters: [`render.go`](../../../internal/runtime/render.go).
- Permission rewriting: [`permissions.go`](../../../internal/runtime/permissions.go).
- Background work: [`scheduler.go`](../../../internal/runtime/scheduler.go), `jobQueue` in [`server.go`](../../../internal/runtime/server.go).
- Tenancy: [`tenant.go`](../../../internal/runtime/tenant.go).
