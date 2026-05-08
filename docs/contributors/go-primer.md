# Go Primer for Kilnx Contributors

A short tour of the Go idioms that recur across the Kilnx codebase, aimed at contributors fluent in another statically typed language. The goal is not to teach Go from scratch but to flatten the ramp into this specific repo. Every example below is grounded in a real file you can open.

## Package layout

A package is a directory of `.go` files declaring `package <name>` on the first non-comment line. Files in the same directory share the package and see each other's exported and unexported identifiers without imports.

Kilnx splits the compiler and runtime under [`internal/`](../../internal/), which Go treats specially: identifiers exported from `internal/X` are only importable by code under the same parent (`github.com/kilnx-org/kilnx/...`). The CLI binary lives under [`cmd/kilnx`](../../cmd/kilnx/main.go); a second binary, [`cmd/kilnx-gendocs`](../../cmd/kilnx-gendocs/main.go), generates reference Markdown.

Naming convention in this repo: capitalized identifiers (`Tokenize`, `Parse`, `App`, `Server`) are exported and stable; lowercase ones (`parserState`, `entities`, `dispatchToken`) are package-private and may change without notice.

## Package doc comments

A doc comment immediately above the `package` clause is the package's public documentation. [`internal/parser/doc.go`](../../internal/parser/doc.go) is the canonical example:

```go
// Package parser builds the Kilnx AST from a token stream produced by
// internal/lexer. ...
package parser
```

Package docs are the first thing to read when entering a new directory. Most Kilnx packages have one.

## Interfaces

Interfaces in Go are structural: a type satisfies an interface by having the right methods, no `implements` keyword required. The runtime relies on this for dependency inversion.

[`database.Executor`](../../internal/database/interfaces.go) defines the surface the runtime needs:

```go
type Executor interface {
    QueryRows(sqlStr string) ([]Row, error)
    QueryRowsWithParams(sqlStr string, params map[string]string) ([]Row, error)
    ExecWithParams(sqlStr string, params map[string]string) error
    BeginTxHandle() (*TxHandle, error)
    Dialect() Dialect
}
```

The real implementation is `*database.DB` in [`database.go`](../../internal/database/database.go), backed by `database/sql`. Tests substitute fakes that implement the same method set. The runtime never names the concrete type beyond `New*` constructors.

When you see a function take `db database.Executor`, the contract is: "this needs SQL execution, the caller picks the implementation." This is the standard Go pattern for testability.

## Struct embedding

Go has no inheritance. Composition is achieved by declaring a field with no name:

```go
type Logger struct { ... }

type Server struct {
    logger *Logger
    // ... other fields
}
```

The runtime prefers explicit fields over embedding to keep ownership obvious. [`Server`](../../internal/runtime/server.go) holds named pointers to `*SessionStore`, `*JobQueue`, `*RateLimiter`, `*Logger`, and `*I18n` rather than embedding them.

## Errors

Go errors are values. Functions return `(T, error)`. The caller checks the error with an `if err != nil` block. This codebase wraps errors with context using `fmt.Errorf` and the `%w` verb so callers can unwrap with `errors.Is` or `errors.As` if needed:

```go
// internal/database/database.go
return nil, fmt.Errorf("opening database %s: %w", url, err)
```

The analyzer takes a different shape because it produces multiple diagnostics rather than a single error: it returns `[]analyzer.Diagnostic`, where each carries `Level` ("error" or "warning"), `Message`, `Line`, and `Context`. The CLI in [`printDiagnostics`](../../cmd/kilnx/main.go) formats them and decides whether to abort.

The parser collects errors via `addError` on `parserState` and recovers with `synchronize()`, so a single `Parse` call can report many syntax errors at once.

## Goroutines and channels

A `goroutine` is a lightweight thread launched with the `go` keyword. Channels are typed pipes used to coordinate them.

The scheduler in [`scheduler.go`](../../internal/runtime/scheduler.go) launches one goroutine per `schedule` block, with a `stop` channel for shutdown:

```go
stop := make(chan struct{})
go s.runSchedule(sched, stop)
```

`runSchedule` uses `time.NewTicker` and `select` to wait on whichever event fires first:

```go
ticker := time.NewTicker(interval)
defer ticker.Stop()
for {
    select {
    case <-ticker.C:
        s.executeScheduleBody(sched)
    case <-stop:
        return
    }
}
```

`chan struct{}` is the idiomatic shutdown signal: it carries no data, only the closure event. Closing the channel broadcasts to every receiver. The job queue in the same file uses an analogous polling pattern.

The hot-reload watcher in [`watcher.go`](../../internal/runtime/watcher.go) is a goroutine launched by `WatchAndServe` that polls file mtime on a 500ms interval and calls `Server.Reload` when the source changes.

## Synchronization

Concurrent access to shared state is guarded with `sync.Mutex` or `sync.RWMutex`. [`Server`](../../internal/runtime/server.go) uses an `sync.RWMutex` to protect the parsed app pointer during hot reload:

```go
func (s *Server) getApp() *parser.App {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.app
}
```

`RLock` allows many concurrent readers but no writers; `Lock` is exclusive. The `defer` keyword schedules `Unlock` to run when the function returns, no matter the path.

Other concurrency primitives in the runtime: `sync.Once` (one-shot warnings), `sync.Map` (manifest cache in [`server.go`](../../internal/runtime/server.go)), `chan struct{}` for shutdown.

## `defer`

`defer` schedules a call to run when the surrounding function returns. The runtime uses it heavily for cleanup that must happen on every exit path:

```go
tx, err := s.db.BeginTxHandle()
if err != nil { ... }
defer tx.Rollback() // no-op if already committed
```

Multiple deferred calls run in LIFO order. Defer is the right tool for `Close`, `Unlock`, `Rollback`, `Stop`.

## Slices and maps

`[]T` is a slice (dynamic array). `map[K]V` is a hash map. Both are reference types: passing them into a function shares the underlying storage. The parser appends to slices on `App` (`app.Pages = append(app.Pages, page)`); the rendering layer keys per-query results on a `map[string][]database.Row`.

The zero value of a slice is `nil`, which is a valid empty slice. The zero value of a map is also `nil` but is read-only; you must `make(map[K]V)` before assigning.

## `database/sql`

[`internal/database/database.go`](../../internal/database/database.go) wraps Go's standard `database/sql` package. The driver is `mattn/go-sqlite3` (CGO-free with `modernc.org/sqlite` when `CGO_ENABLED=0`). `*sql.DB` is a connection pool, safe for concurrent use. Transactions are obtained with `Begin`; the Kilnx wrapper exposes them as `*TxHandle` with `QueryRowsWithParams`, `ExecWithParams`, `Commit`, `Rollback`. Action handlers wrap their body in a transaction so failures roll back partial mutations.

## `go generate`

`go generate` runs commands embedded in source files as `//go:generate` comments. [`internal/parser/doc.go`](../../internal/parser/doc.go) holds:

```go
//go:generate go run ../../cmd/kilnx-gendocs -o ../../docs/devs/reference
```

Running `go generate ./internal/parser/...` invokes that command. We use it to regenerate the language reference Markdown after touching any `*_spec.go` file. There is no commit hook; reviewers run it manually or rely on `go run ./cmd/kilnx-gendocs -check-stale` in CI.

## `init()`

A function named `init()` (no args, no return) runs once per package, before `main`, in dependency-import order. Kilnx uses `init()` to populate the spec registry without touching parser code at every site:

```go
// internal/parser/page_spec.go
func init() {
    spec.Register(spec.Entity{Name: "page", Kind: spec.KindKeyword, ...})
}
```

The `cmd/kilnx-gendocs` binary blank-imports `internal/parser` (`_ "github.com/kilnx-org/kilnx/internal/parser"`) purely to trigger these `init()` calls. See [architecture/spec-registry.md](architecture/spec-registry.md).

## Testing conventions

Test files live next to the code under test as `*_test.go`. Package `foo` ships its tests under `package foo` (white-box) or `package foo_test` (black-box). Kilnx mostly uses white-box tests so they can reach unexported helpers.

A test function takes `*testing.T`:

```go
func TestRequireAuth(t *testing.T) {
    if !ok {
        t.Errorf("expected auth gate to allow request: got %v", err)
    }
}
```

Run all tests:

```bash
go test ./...
```

One package, with verbose output:

```bash
go test -v ./internal/runtime/...
```

Race detector (recommended for runtime work):

```bash
go test -race ./internal/runtime/...
```

Several packages also ship `_test.go` files using subtests via `t.Run(name, ...)` and table-driven tests, both standard Go idioms.

## What we don't use

To save you searching:

- No generics in hot paths. The codebase predates Go 1.18 in spirit; concrete types win when types are simple.
- No reflection in the parser or runtime hot path. The analyzer reflects over models in a few targeted places only.
- No third-party logging library. The runtime's [`Logger`](../../internal/runtime/logger.go) wraps `log` from the standard library.
- No DI container. Wiring is explicit in `NewServer` and friends.
- No build tags besides the standard ones. `CGO_ENABLED=0` is set by [`build.Build`](../../internal/build/build.go) and respected by the SQLite driver selection in [`database`](../../internal/database/).

## Where to read more

- The Go tour: [https://go.dev/tour](https://go.dev/tour).
- Effective Go: [https://go.dev/doc/effective_go](https://go.dev/doc/effective_go).
- For `database/sql` patterns specifically, [http://go-database-sql.org/](http://go-database-sql.org/) is the canonical primer.

When in doubt, the answer is usually the `_test.go` file next to the code you are trying to understand. Tests are how this repo documents its public API.
