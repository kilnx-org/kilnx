# Changelog

All notable changes to Kilnx are documented in this file.

Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `llm ... agent` runtime (P3): the `agent` discriminator now spawns the `claude` CLI (Claude Code v2.x) as a subprocess and consumes its `stream-json` output. Exposes `:<name>.text`, `:<name>.session_id`, `:<name>.cost_usd`, `:<name>.duration_ms`, `:<name>.stop_reason` to downstream nodes for manual persistence. Honours `permission-mode` (default `plan`), `tools`, `max-budget-usd` (required), `max-turns` (enforced in runtime since CLI lacks the flag), `cwd` (contained inside `config workspace-root` via `EvalSymlinks` prefix check; tmpdir created and removed when omitted), `resume: :session_id` for conversation continuation, and `mcp:` to mount top-level `mcp <name>` server declarations via a per-request `--mcp-config` file. `show-tools: true` opt-in surfaces tool_use/tool_result frames on a separate hyperstream channel when streaming.
- Top-level `mcp <name>` keyword: declares MCP servers (stdio or http/sse transport) reachable by `llm ... agent` blocks. Children: `command`, `args`, `env`, `url`, `transport`.
- `config workspace-root`: filesystem root for agent `cwd` resolution. Required by analyzer whenever any `llm ... agent` block exists.
- `kilnx run` startup check refuses to start when an agent block is declared and the `claude` CLI is not on PATH. `kilnx check` reports the same condition as a warning.
- `fetch` results bind under the user-chosen name (`fetch payment: ...` exposes `:payment.*`); previous releases hardcoded the `fetch.` prefix, silently overwriting multiple fetches in the same action.
- Every `fetch` exposes `:<name>.status_code` and `:<name>.ok` (true for 2xx) so actions can branch with `on payment.ok` without inspecting response shape.
- `fetch` body is encoded as JSON (with typed numbers and booleans) when the user sets `header Content-Type: application/json`. Required by Stripe, OpenAI, and other JSON-only APIs; legacy form-urlencoded encoding remains the default.
- Builtin function library callable from `fetch` body and header values: 27 pure functions across string (`lower`, `upper`, `slugify`, `format`, ...), crypto (`bcrypt`, `sha256`, `uuid`, `base64`), math (`round`, `floor`, `ceil`, `clamp`, `min`, `max`), and util (`coalesce`, `regex`, `matches`, `json_get`, `now`) groups. Opt-in via `name(args)` syntax; plain `:param` templates are unchanged.
- Arithmetic and string concatenation in fetch values: `body amount: round(:total * 100)`, `body label: format("Order {0}", :id)`.

### Changed
- Transport-level `fetch` failures (DNS, timeout, connection refused) now abort the surrounding action with `502 Bad Gateway` and roll back the implicit transaction. Jobs and schedules surface the error to the queue. HTTP 4xx / 5xx are still parsed and exposed (not treated as transport errors). Page renders continue to degrade gracefully.

### Fixed
- `kilnx migrate` now detects schema drift across five dimensions: orphan columns (DB has them but model no longer declares them), column type mismatch, NOT NULL mismatch, single-column UNIQUE mismatch, and DEFAULT presence mismatch (value not compared, since dialect normalization is unreliable across SQLite/Postgres). Previously the diff was unidirectional (model -> DB only), so any DB-side change beyond a missing column went unreported and `kilnx migrate` reported "up to date" against a divergent schema. Warnings are grouped by kind with `table.column (detail)` lines plus a manual-fix hint per kind. Migration itself remains additive: no destructive ALTERs are generated.

### Security
- `fetch` log lines redact the URL query string so secrets passed via `:param` substitution do not leak to stdout.

## [0.1.2] - 2026-04-22

### Added
- Non-unique model-level `index (field_a, field_b, ...)` directive; emits idempotent `CREATE INDEX IF NOT EXISTS ix_<table>_<cols>` for query acceleration, validated by the analyzer the same way as composite UNIQUE
- LSP and MCP surfaces advertise the composite `unique (...)` and `index (...)` directives; model hover lists declared groups
- Composite UNIQUE via model-level `unique (field_a, field_b, ...)` directive; emits idempotent `CREATE UNIQUE INDEX IF NOT EXISTS` and is validated by the analyzer (unknown fields, duplicated fields within a group, or duplicate groups)
- `tenant` modifier on `model` for multi-tenant scoping with fail-closed guards across every query path (refs [#48](https://github.com/kilnx-org/kilnx/issues/48), [#52](https://github.com/kilnx-org/kilnx/pull/52))
- PostgreSQL support via `Dialect` abstraction; switch engines with `database: postgres://...` ([#46](https://github.com/kilnx-org/kilnx/pull/46))
- Multi-file support via `import` statement with `.kilnx` extension enforcement and path traversal protection (closes [#20](https://github.com/kilnx-org/kilnx/issues/20), [#43](https://github.com/kilnx-org/kilnx/pull/43))
- `static` directive to serve files from a directory, validated to stay within project root ([#42](https://github.com/kilnx-org/kilnx/pull/42))
- `fetch` keyword for outbound HTTP from actions, jobs, and webhooks ([#39](https://github.com/kilnx-org/kilnx/pull/39))
- Inline fragment rendering on htmx action redirect: matching fragment is returned in the same response when redirecting to a page that defines it

### Fixed
- SQL `NULL` no longer rendered as truthy in template conditionals ([#44](https://github.com/kilnx-org/kilnx/pull/44))
- Filters inside `{{each}}` blocks now resolve from the current row, not the outer scope ([#40](https://github.com/kilnx-org/kilnx/pull/40))
- `import` treated as directive only at column 0, allowing the token to appear elsewhere
- Timezone-aware time formats parsed in `date` and `timeago` filters
- Named parameter error messages clarified

### Security
- Mutation bypass via SQL comment, subquery, and string literal closed ([#52](https://github.com/kilnx-org/kilnx/pull/52))
- Tenant guard fails closed and redacts sensitive user fields when rendering ([#52](https://github.com/kilnx-org/kilnx/pull/52))
- Static directory traversal blocked ([#42](https://github.com/kilnx-org/kilnx/pull/42)); import traversal blocked and depth limited ([#43](https://github.com/kilnx-org/kilnx/pull/43))

## [0.1.1] - 2026-03-31

### Added
- Governance files, issue templates, and repo automation
- Auto-assignment of PRs to CODEOWNER as reviewer and assignee
- Phase 1 credibility batch ([#38](https://github.com/kilnx-org/kilnx/pull/38))

### Fixed
- `#` characters preserved inside `html` blocks (closes [#32](https://github.com/kilnx-org/kilnx/issues/32))

## [0.1.0] - 2026-03-28

Initial public release. 27 keywords, 2 runtime dependencies (SQLite + bcrypt), single-binary output.

### Added
- Declarative constructs: `config`, `model`, `auth`, `permissions`, `layout`, `page`, `action`, `fragment`, `stream`, `socket`, `api`, `webhook`, `job`, `schedule`, `test`
- Field types: `text`, `email`, `int`, `float`, `bool`, `timestamp`, `richtext`, `option`, `password`, `image`, `phone`
- Constraints: `required`, `unique`, `default`, `auto`, `min`, `max`
- CLI: `kilnx run`, `build`, `check`, `test`, `migrate`, `lsp`, `version`
- SQLite runtime with automatic migrations
- Declarative auth with bcrypt password hashing and HMAC-signed sessions
- htmx-aware fragment rendering with `hx-target` / `hx-swap` semantics
- Server-Sent Events via `stream`, WebSockets via `socket`
- Background jobs and cron-style `schedule`
- Declarative `test` blocks with automatic test database (`app.kilnx.test`)
- Template engine for full HTML control, logical operators, comments
- Static analyzer: type checking, multi-table column validation, subquery analysis, CSRF and security linting
- Query deduplication, JOIN pruning, stream materialization hints ([#12](https://github.com/kilnx-org/kilnx/pull/12))
- Language Server Protocol via `kilnx lsp`
- Docker image published at `ghcr.io/kilnx-org/kilnx:0.1.0`
- Install script and GoReleaser-built pre-compiled binaries for macOS and Linux

### Security
All 14 hardening items from the pre-release security review landed in this version:
- CSRF protection enabled by default on every mutation path; linter flags missing `csrf` tokens
- Parameterized SQL with strict binding; injection paths in string literals, comments, and subqueries closed
- HTML output escaped by default; raw output requires explicit opt-in
- bcrypt password hashing with per-install cost; constant-time credential comparison
- HMAC-signed session cookies with `HttpOnly`, `Secure`, `SameSite=Lax`
- Rate limiting primitives on auth and action endpoints
- `.kilnx` import sandbox: extension check, project root containment, depth limit
- Sensitive field redaction in logs and error output

[Unreleased]: https://github.com/kilnx-org/kilnx/compare/v0.1.2...HEAD
[0.1.2]: https://github.com/kilnx-org/kilnx/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/kilnx-org/kilnx/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/kilnx-org/kilnx/releases/tag/v0.1.0
