# Internal Packages

Generated from `internal/*` package source. Do not edit by hand. Run `go generate ./...` to regenerate.

| Package | Summary |
|---------|---------|
| [`lexer`](lexer.md) | Package lexer transforms .kilnx source text into a token stream consumed by internal/parser. |
| [`parser`](parser.md) | Package parser builds the Kilnx AST from a token stream produced by internal/lexer. |
| [`analyzer`](analyzer.md) | Package analyzer performs static analysis on the parser AST: type checking of field constraints, validation of route templates, permission/role checks, dead-link detection, and security audits. |
| [`optimizer`](optimizer.md) | Package optimizer rewrites the parser AST so the runtime can execute it efficiently. |
| [`spec`](spec.md) | Package spec is the canonical schema for Kilnx language entities (keywords and attributes). |
| [`database`](database.md) | Package database provides the persistence layer for the Kilnx runtime: connection management, schema migration, and dialect-aware query execution against SQLite or PostgreSQL. |
| [`pathutil`](pathutil.md) | Package pathutil provides helpers for matching declarative route templates (e.g. "/tasks/:id/delete") against concrete request paths (e.g. "/tasks/123/delete"). |
| [`pdf`](pdf.md) | Package pdf is a minimal, dependency-free PDF writer used by the runtime to render reports and printable views from .kilnx pages. |
| [`runtime`](runtime.md) | Package runtime is the HTTP server and AST interpreter that backs `kilnx run` and the binaries produced by `kilnx build`. |
| [`build`](build.md) | Package build compiles a .kilnx source file into a standalone Go binary by emitting Go source that embeds the AST and a minimal runtime, then invoking `go build`. |

