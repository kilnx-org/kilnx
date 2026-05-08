# Contributor Docs

Internals reference for [Kilnx](../../README.md). For Go programmers contributing to the compiler, runtime, or tooling, and for readers reverse-engineering the codebase.

## Audience

- Contributors patching the lexer, parser, analyzer, optimizer, runtime, or `cmd/`.
- Reviewers grading PRs that touch language semantics or runtime behavior.
- Curious Go developers reading Kilnx as a worked example of a small declarative DSL hosted on `net/http` and SQLite.

User-facing language reference lives under [docs/devs/](../devs/) and is generated from the `internal/spec` registry. See [spec-registry.md](architecture/spec-registry.md) for how that works.

## What Kilnx is

A `.kilnx` file declares models, pages, actions, fragments, APIs, jobs, schedules, sockets, streams, and webhooks. The toolchain reads that file, validates it, and either serves it directly (`kilnx run`) or produces a standalone Go binary (`kilnx build`). One file, one binary. No template language separate from the DSL, no migration tool separate from the model declaration.

## The compile pipeline

Every command (`run`, `check`, `build`, `migrate`, `test`) shares the same front end:

```
.kilnx source
     |
     v
[lexer]      indent-sensitive tokenizer       internal/lexer
     |
     v
[parser]     token stream -> AST              internal/parser
     |
     v
[analyzer]   diagnostics, schema check        internal/analyzer
     |
     v
[optimizer]  SELECT * expansion, JOIN prune,  internal/optimizer
             query dedup
     |
     +----> [runtime]   in-process HTTP server  internal/runtime
     |                  used by `kilnx run`
     |
     +----> [build]     emit cmd/_build/main.go internal/build
                        go build -o app
                        used by `kilnx build`
```

Both `kilnx run` and `kilnx build` execute the same five compile stages. They differ only in how source is loaded and where the runtime is invoked: `kilnx run` reads the file from disk via [`runtime.WatchAndServe`](../../internal/runtime/watcher.go) and watches it for hot reload; `kilnx build` calls [`build.Build`](../../internal/build/build.go) which writes a temporary `cmd/_build/main.go` embedding the source as a Go `const` and shells out to `go build` with `CGO_ENABLED=0`. The compiled binary then re-runs the full pipeline at startup against the embedded source.

See [architecture/pipeline.md](architecture/pipeline.md) for stage-by-stage detail.

## Where to read next

| If you want to                                | Read                                                                              |
| --------------------------------------------- | --------------------------------------------------------------------------------- |
| Stage-by-stage pipeline depth                 | [architecture/pipeline.md](architecture/pipeline.md)                              |
| AST shape, node types, field attachment       | [architecture/ast.md](architecture/ast.md)                                        |
| HTTP request lifecycle and routing order      | [architecture/runtime-execution.md](architecture/runtime-execution.md)            |
| Add a new keyword or attribute                | [architecture/spec-registry.md](architecture/spec-registry.md)                    |
| Catch up on Go idioms used in the codebase    | [go-primer.md](go-primer.md)                                                      |
| Work on a specific package                    | [packages/](packages/)                                                            |

## Repository layout

```
cmd/
  kilnx/             CLI entry point: run, check, build, migrate, test
  kilnx-gendocs/     reads internal/spec registry, emits docs/devs/reference
  _build/            transient. Created by `kilnx build`, deleted on exit.
internal/
  lexer/             tokenizer, INDENT/DEDENT, raw-line capture
  parser/            AST + per-entity *_spec.go registrations
  analyzer/          diagnostics, schema check, optional DB cross-check
  optimizer/         SELECT * expansion, JOIN prune, query dedup
  runtime/           HTTP server, auth, forms, scheduler, jobs, sockets
  build/             generates cmd/_build/main.go and runs go build
  database/          sql.DB wrapper, Executor interface, migrations
  spec/              registry consumed by kilnx-gendocs
docs/
  devs/              app-author documentation (generated reference under reference/)
  contributors/      this section
```

## Building and testing locally

You need a Go toolchain matching `go.mod`. There is no Makefile.

Run the full suite:

```bash
go test ./...
```

One package:

```bash
go test ./internal/runtime/...
```

Build the CLI:

```bash
go build ./cmd/kilnx
```

Regenerate the reference docs after touching any `*_spec.go`:

```bash
go generate ./internal/parser/...
```

The generator is wired through [`internal/parser/doc.go`](../../internal/parser/doc.go), which holds a `//go:generate` directive pointing at [`cmd/kilnx-gendocs`](../../cmd/kilnx-gendocs/main.go). Output lands in [`docs/devs/reference/`](../devs/reference/). Run `go run ./cmd/kilnx-gendocs -check-stale` to confirm no implementation file is newer than its spec.

Smoke-test against an example:

```bash
go run ./cmd/kilnx run examples/blog.kilnx
```

The server prints discovered routes, schedules, jobs, sockets, and rate limits at startup. See [`printRoutes`](../../internal/runtime/watcher.go) for the format.

## Conventions

- The DSL is the source of truth. Anything declared in `.kilnx` should not need a second declaration in Go. The exception is the `*_spec.go` doc registration, which exists only so that human-readable reference docs can be generated.
- No em-dash or en-dash in docs. Use comma, period, or rewrite.
- Reference real paths. Example: [`internal/runtime/server.go`](../../internal/runtime/server.go).
- Errors flow up. Lexer and parser collect errors with line numbers from the original source. The analyzer emits typed `Diagnostic` values with a `Level` of `error` or `warning`. Runtime errors flow through [`Logger`](../../internal/runtime/logger.go).
- The runtime never blocks on disk for routing. Source watching runs in a goroutine in [`watcher.go`](../../internal/runtime/watcher.go) and swaps the AST under a write lock via `Server.Reload`.
- Schedulers and the job queue are started by the runtime, not the CLI. See [`scheduler.go`](../../internal/runtime/scheduler.go).

## Asking questions

If a piece of code is unclear, the answer is usually in:

1. The package doc comment of the file you are reading.
2. The corresponding `*_spec.go` next to it (for parser entities).
3. A `_test.go` file in the same package, which exercises the public API by example.
