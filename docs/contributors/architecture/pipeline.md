# Pipeline

A `.kilnx` source file flows through five stages before serving traffic. The same five stages run inside `kilnx run` (interpreted) and inside the `main.go` template emitted by `kilnx build` (AOT). The `runtime` stage is replaced by `build` only in the AOT path, and `build` itself re-uses the runtime: it just embeds the source so the compiled binary re-runs the pipeline at startup.

## Flow

```
.kilnx source
     |
     v
[lexer]      tokens
     |
     v
[parser]     *parser.App (AST)
     |
     v
[analyzer]   []Diagnostic   ----> reject on error
     |
     v
[optimizer]  *parser.App (rewritten in place)
     |
     +----> [runtime]   kilnx run      net/http server
     |
     +----> [build]     kilnx build    standalone Go binary
                                       (embeds source, runtime runs at start)
```

## Stages

### Lexer

- Package: [`internal/lexer`](../../../internal/lexer).
- Entry: [`lexer.Tokenize`](../../../internal/lexer/lexer.go).
- Input: raw source string (after `lexer.StripComments`).
- Output: `[]lexer.Token` with `Type`, `Value`, `Line`, `Column`.
- Responsibility: split source into tokens, track 1-based positions, emit `TokenIndent` and `TokenDedent` for the indentation-sensitive grammar, preserve full lines as `TokenRawLine` for SQL, HTML, and computed expression capture downstream.
- Key files: [`lexer.go`](../../../internal/lexer/lexer.go), [`fuzz_test.go`](../../../internal/lexer/fuzz_test.go).

### Parser

- Package: [`internal/parser`](../../../internal/parser).
- Entry: [`parser.Parse(tokens, source)`](../../../internal/parser/parser.go).
- Input: token stream plus the original source (retained for line-based extraction of SQL, HTML, computed expressions, and error context).
- Output: `*parser.App`, the root AST.
- Responsibility: dispatch on top-level keyword (`model`, `page`, `action`, `fragment`, `api`, `stream`, `schedule`, `job`, `webhook`, `socket`, `limit`, `config`, `auth`, `permissions`, `layout`, `test`, `log`, `translations`, `query`), build typed nodes, resolve named queries into call sites, accumulate parse errors with `synchronize()` recovery between top-level entities.
- Key files: [`parser.go`](../../../internal/parser/parser.go) (AST types and dispatch), `*_spec.go` (per-keyword `spec.Entity` registration, see [ast.md](ast.md) and [spec-registry.md](spec-registry.md)).

### Analyzer

- Package: [`internal/analyzer`](../../../internal/analyzer).
- Entries: [`analyzer.Analyze(app)`](../../../internal/analyzer/analyzer.go), `analyzer.AnalyzeWithDB(app, db)` for live-DB cross-checks.
- Input: `*parser.App`.
- Output: `[]analyzer.Diagnostic` (each carries `Level`, `Message`, `Line`, `Context`).
- Responsibility: derive a [`Schema`](../../../internal/analyzer/analyzer.go) from `app.Models` (with column-mode custom fields folded in), validate SQL against known tables and columns, check action attributes, fragment components, translation params, route uniqueness, security rules in [`security.go`](../../../internal/analyzer/security.go), and (with DB) reconcile against `INFORMATION_SCHEMA` in [`db_check.go`](../../../internal/analyzer/db_check.go).
- Errors abort `kilnx run`, `kilnx build`, and `kilnx test`. Warnings are printed and execution continues. See [`cmd/kilnx/main.go`](../../../cmd/kilnx/main.go) for the gating logic.
- Key files: [`analyzer.go`](../../../internal/analyzer/analyzer.go), [`types.go`](../../../internal/analyzer/types.go), [`security.go`](../../../internal/analyzer/security.go), [`db_check.go`](../../../internal/analyzer/db_check.go).

### Optimizer

- Package: [`internal/optimizer`](../../../internal/optimizer).
- Entry: [`optimizer.Optimize(app)`](../../../internal/optimizer/optimizer.go).
- Input: `*parser.App`. Mutated in place.
- Output: same `*parser.App` with rewritten queries and stream candidates marked.
- Responsibility:
  - rewrite `SELECT *` to an explicit column list when all consumed fields are known from template usage,
  - dedupe identical queries inside the same `Page`, `Fragment`, or `API` body,
  - mark queries eligible to be served as SSE `stream` candidates via `markStreamCandidates`.
- Optimizer is best-effort: it never changes semantics, only narrows what is fetched. Unsafe rewrites (e.g. queries with multiple unnamed `_last` aliases) are skipped.
- Key files: [`optimizer.go`](../../../internal/optimizer/optimizer.go).

### Runtime

- Package: [`internal/runtime`](../../../internal/runtime).
- Entry: [`runtime.NewServer(app, db, port).Start()`](../../../internal/runtime/server.go).
- Input: `*parser.App`, a `database.Executor`, and a port.
- Output: a running HTTP server.
- Responsibility: bind `net/http` mux, serve embedded `htmx.min.js` and `sse.js`, route requests across webhooks, sockets, auth POST handlers, actions, streams, APIs, fragments, and pages, enforce auth and rate limits, render templates, run scheduler and job queue, manage sessions, log requests and slow queries.
- Detailed walk: [runtime-execution.md](runtime-execution.md).
- Key files: [`server.go`](../../../internal/runtime/server.go), [`auth.go`](../../../internal/runtime/auth.go), [`render.go`](../../../internal/runtime/render.go), [`permissions.go`](../../../internal/runtime/permissions.go), [`scheduler.go`](../../../internal/runtime/scheduler.go), [`ratelimit.go`](../../../internal/runtime/ratelimit.go).

### Build (AOT)

- Package: [`internal/build`](../../../internal/build).
- Entry: [`cmdBuild` in `cmd/kilnx/main.go`](../../../cmd/kilnx/main.go), which resolves imports and manifests, then calls [`build.Build`](../../../internal/build/build.go).
- Input: path to a `.kilnx` file and an optional output binary path.
- Output: standalone binary for the host OS and architecture, which runs the same lexer, parser, analyzer, optimizer, runtime stack on startup against the embedded source.
- Responsibility: copy the running kilnx executable and append a bundle (resolved source plus static custom-field manifests) followed by a length and magic trailer. No Go toolchain or source tree is involved.
- Build does not pre-resolve the AST. At startup `main` looks for a trailer on its own executable and, if found, `runBundle` re-runs the full pipeline so behaviour matches `kilnx run`.
- Key files: [`build.go`](../../../internal/build/build.go).

## Where each entry point wires the pipeline

- `kilnx run`: [`cmdRun` in `cmd/kilnx/main.go`](../../../cmd/kilnx/main.go) calls `loadApp` (lexer plus parser), then `analyzer.Analyze`, then `optimizer.Optimize`, then `runtime.NewServer(...).Start()`.
- `kilnx check`: [`cmdCheck`](../../../cmd/kilnx/main.go) stops after analyzer.
- `kilnx test`: parser plus analyzer plus optimizer, then a runtime instance bound to port 9999 to drive `Test` blocks.
- `kilnx build`: [`cmdBuild`](../../../cmd/kilnx/main.go) validates and writes the binary through `internal/build`; [`runBundle`](../../../cmd/kilnx/main.go) is the same call sequence at the output's startup.
