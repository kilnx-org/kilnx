# `internal/build`

> Package build produces standalone binaries by appending an app bundle to a copy of the running kilnx executable, and reads that bundle back at startup.

| | |
|---|---|
| **Import path** | `github.com/kilnx-org/kilnx/internal/build` |
| **Source last touched** | `5da8498` (2026-05-08) |
| **Doc last touched** | `5da8498` (2026-05-08) |


## Overview

Used by the `kilnx build` CLI command. The kilnx CLI already contains the
whole runtime, so a build needs neither the Go toolchain nor the kilnx
source tree. Output runs on the same OS and architecture as the kilnx
that produced it, with no kilnx CLI required at runtime.

## Files

| File | Summary |
|------|---------|
| [`build.go`](../../../internal/build/build.go) | _no file-level doc_ |

## Types

### `Bundle`

```go
type Bundle struct {
	Source		string			`json:"source"`
	Manifests	map[string]string	`json:"manifests,omitempty"`
}
```

Bundle is everything a standalone binary needs from the project: the
resolved source with imports inlined, and the raw text of each static
custom-field manifest keyed by model name.

## Functions

### `Build`

```go
func Build(bundle Bundle, kilnxFile, outputPath string) (string, error)
```

Build writes a standalone binary to outputPath: a copy of the running
kilnx executable with the bundle appended. It needs neither the Go
toolchain nor the kilnx source tree, and targets the host OS and
architecture. It returns the path written.

### `Read`

```go
func Read() (*Bundle, error)
```

Read returns the bundle appended to the running executable, or nil for a
plain kilnx CLI.

### `read`

```go
func read(exePath string) (*Bundle, error)
```
### `sameFile`

```go
func sameFile(a, b string) (bool, error)
```
### `write`

```go
func write(exePath string, bundle Bundle, kilnxFile, outputPath string) (string, error)
```

## Notes

<!-- MANUAL-NOTES START -->
# `internal/build`

Compiles a `.kilnx` source file into a single standalone Go binary. Single-file package.

## Purpose

`kilnx build app.kilnx -o myapp` shells out to the Go toolchain. The resulting binary embeds the original `.kilnx` source as a string constant, parses and serves it at runtime, and has no dependency on the `kilnx` CLI itself. This is how Kilnx apps deploy to PaaS targets like Railway, Fly.io, Render, Cloud Run.

## File map

- [`build.go`](../../../internal/build/build.go): everything. Public `Build` function, `generateMainGo` template, `findKilnxRoot` lookup.
- [`doc.go`](../../../internal/build/doc.go): package doc comment.

## Public surface

```go
func Build(kilnxFile, outputPath string) error
```

Reads `kilnxFile`, derives `outputPath` from the filename if empty (stripping the extension), generates a `main.go` that embeds the source, and runs `go build` to produce the binary.

## How it works

1. **Locate the kilnx source tree**: `findKilnxRoot` walks up from both the executable path and the current working directory looking for a `go.mod` whose module path contains `kilnx-org/kilnx`. The build needs the runtime's Go source to be reachable on disk because it links against it. Returns `""` when nothing is found, in which case `Build` errors with a hint to run inside the kilnx repo or use Docker.

2. **Stage a temporary main package**: creates `<kilnxRoot>/cmd/_build/` (mode `0o750`), writes a synthesized `main.go` (mode `0o600`) into it. The directory is removed via `defer os.RemoveAll`.

3. **Embed the .kilnx source**: `generateMainGo` returns a Go source string containing a `const embeddedSource = ` raw-string literal. Backticks in the source are escaped via `` `+ "`" + ` `` concatenation so the raw literal stays valid.

4. **Compile**: runs `go build -o <absOutput> ./cmd/_build/` with `cmd.Dir = kilnxRoot` and `CGO_ENABLED=0` appended to the environment. Stdout and stderr stream straight through, so the user sees Go compiler errors verbatim.

5. **Report size**: stats the resulting file and prints `Built: <path> (<MB>)`.

## What the embedded `main.go` does at runtime

The generated entrypoint pipelines the same boot sequence as `kilnx run`, just against an in-memory source string:

1. `lexer.StripComments` → `lexer.Tokenize` → `parser.Parse`.
2. `analyzer.Analyze`. Errors print to stderr and abort, warnings print and continue.
3. `optimizer.Optimize`.
4. Resolve port: `app.Config.Port`, then `PORT` env var (PaaS convention; rejected if non-numeric or out of range), then `--port` flag.
5. Resolve database URL: `app.Config.Database`, then `--db` flag. Defaults to `app.db`.
6. `database.Open`, `db.MigrateInternal` (sessions, jobs, etc.), `db.Migrate(app.Models, app.CustomManifests)`.
7. `runtime.NewServer`, `srv.StartScheduler`, `srv.StartJobQueue`, `srv.Start`.

The PORT handling is intentional: PaaS platforms inject `PORT`, so the binary respects it without flags.

## Build-dir layout

```
<kilnxRoot>/
  cmd/
    _build/             <- created and removed by Build
      main.go           <- synthesized, embeds .kilnx source
```

The `_build` name (with leading underscore) is a Go convention for a directory the toolchain treats as part of a parent module but that build tools or test helpers can recognise as transient. `defer os.RemoveAll(buildDir)` runs even when `go build` fails.

## Gotchas

- **Requires the kilnx source tree on disk**. There is no precompiled runtime archive. Distributing `kilnx build` to users who don't have the repo means shipping the repo too, or going through Docker. The error message points to a `Dockerfile` flow.
- **Concurrent builds collide**. Two `Build` calls running at the same time both write to `cmd/_build/main.go` and both try to remove the directory on exit. The function is not goroutine-safe across processes either. Serialize at the caller.
- **Module path check is substring-based**: `findKilnxRoot` matches any `go.mod` containing `kilnx-org/kilnx`. A fork with that string in a comment would also match.
- **CGO disabled**. The produced binary is statically linked. SQLite goes through `modernc.org/sqlite` (pure Go), Postgres through `pgx`. No OS-level deps.
- **Backtick escaping is naive but correct**: each `` ` `` in the source becomes `` `+ "`" + ` ``. There is no other transformation, so any other Go-string-breaking sequence inside the `.kilnx` file is irrelevant; only backticks matter inside a raw string literal.
- **Output path**: when `outputPath` is empty, the binary is named after the `.kilnx` file with the extension stripped. The path is then resolved with `filepath.Abs` before being passed to `go build`, so relative outputs land where the user invoked `kilnx build`, not inside `kilnxRoot`.

## When to touch this file

- Adjusting the boot sequence baked into the binary (new flag, new init step) means editing `generateMainGo`.
- Changing how the runtime is located means editing `findKilnxRoot`.
- Changing the emitted binary's behavior in any way that the runtime can't infer from the AST goes here, because the analyzer and optimizer otherwise see the same input as `kilnx run`.
<!-- MANUAL-NOTES END -->
