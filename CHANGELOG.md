# Changelog

All notable changes to Kilnx are documented in this file.

Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
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

[Unreleased]: https://github.com/kilnx-org/kilnx/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/kilnx-org/kilnx/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/kilnx-org/kilnx/releases/tag/v0.1.0
