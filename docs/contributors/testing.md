# Testing

Testing conventions used across the Kilnx Go codebase. Two distinct concerns share the word "test":

1. **Go tests for the Kilnx implementation.** Standard `go test` covering the lexer, parser, analyzer, optimizer, runtime, and CLI. This page is mostly about these.
2. **The `test` keyword inside `.kilnx` apps.** A small DSL for HTTP-level smoke tests that user apps run via `kilnx test app.kilnx`. See [`docs/devs/reference/keywords/test.md`](../devs/reference/keywords/test.md) and the runner in [`internal/runtime/testing.go`](../../internal/runtime/testing.go). Mentioned here for completeness, treated separately below.

## Running the suite

```bash
go test ./...                 # all packages
go test -race ./...           # race detector, the default for local dev
go test -run TestParse ./internal/parser
go test -bench=. ./internal/parser
go test -fuzz=FuzzParse ./internal/parser
```

CI runs `go test -race ./...` plus the smoke pipeline. See [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml).

## Layout

Tests live next to the code they cover, no separate `tests/` tree. One `_test.go` file per concern:

```
internal/parser/parser_test.go        recursive descent parser
internal/parser/bench_test.go         parser benchmarks
internal/parser/fuzz_test.go          parser fuzz seeds
internal/parser/spec_test.go          spec registry invariants
internal/analyzer/analyzer_test.go    type/schema checks
internal/analyzer/security_test.go    security rules
internal/runtime/server_render_test.go HTTP rendering integration
internal/runtime/webhook_test.go      webhook handler
internal/runtime/*_e2e_test.go        end-to-end HTTP integration
internal/pathutil/match_test.go       URL path matching
smoke/                                .kilnx files exercised by CI
```

## Table-driven tests

The default style. Cleanest example: [`internal/pathutil/match_test.go`](../../internal/pathutil/match_test.go).

```go
func TestMatch(t *testing.T) {
    cases := []struct {
        template string
        path     string
        want     bool
    }{
        {"/tasks/:id/delete", "/tasks/123/delete", true},
        {"/tasks/:id", "/tasks/abc", true},
        {"/tasks/:id", "/tasks/abc/extra", false},
        {"/", "/", true},
    }
    for _, tc := range cases {
        got := Match(tc.template, tc.path)
        if got != tc.want {
            t.Errorf("Match(%q,%q) = %v, want %v",
                tc.template, tc.path, got, tc.want)
        }
    }
}
```

Use this whenever a function has clear input and expected output. Name the slice `cases` or `tests`, name the iterator `tc` or `tt`. Skip subtests (`t.Run`) for short tables; promote to `t.Run(tc.name, ...)` when failure messages stop being self-describing.

## Parser tests

[`internal/parser/parser_test.go`](../../internal/parser/parser_test.go) defines two helpers reused throughout the package:

```go
func parse(t *testing.T, src string) *App {
    t.Helper()
    tokens := lexer.Tokenize(src)
    app, err := Parse(tokens, src)
    if err != nil {
        t.Fatalf("parse error: %v", err)
    }
    return app
}

func parseAllowErrors(t *testing.T, src string) (*App, error) {
    t.Helper()
    tokens := lexer.Tokenize(src)
    return Parse(tokens, src)
}
```

A typical parser test looks like:

```go
func TestPageRequiresAuth(t *testing.T) {
    app := parse(t, "page /dashboard requires auth\n  \"Dashboard\"")
    p := app.Pages[0]
    if !p.Auth {
        t.Error("page should have Auth=true")
    }
    if p.RequiresRole != "auth" {
        t.Errorf("expected RequiresRole 'auth', got %q", p.RequiresRole)
    }
}
```

To add a parser test for a new keyword or attribute:

1. Pick the smallest source string that exercises the path. Use `\n` and indented lines.
2. Call `parse(t, src)` (or `parseAllowErrors` if you want to assert error behavior).
3. Index into `app.Pages`, `app.Webhooks`, etc., and assert AST fields.

For invariants over the entire spec registry (every entity has a `Summary`, all `Children` are registered, etc.), add to [`spec_test.go`](../../internal/parser/spec_test.go) rather than rolling your own walker.

## Analyzer tests

Analyzer tests build a `parser.App` directly via struct literals, no parser involved. See [`internal/analyzer/analyzer_test.go`](../../internal/analyzer/analyzer_test.go) and [`internal/analyzer/security_test.go`](../../internal/analyzer/security_test.go):

```go
func TestCheckWebhookSecrets(t *testing.T) {
    app := &parser.App{
        Webhooks: []parser.Webhook{
            {Path: "/webhook/stripe"},
            {Path: "/webhook/github", SecretEnv: "GITHUB_SECRET"},
        },
    }
    diags := checkWebhookSecrets(app)
    if len(diags) != 1 {
        t.Fatalf("expected 1 diagnostic, got %d", len(diags))
    }
    if !strings.Contains(diags[0].Context, "/webhook/stripe") {
        t.Errorf("expected context about /webhook/stripe, got %q", diags[0].Context)
    }
}
```

To add a new analyzer test:

1. Construct the `parser.App` directly. Skip the parser, you want full control of the AST shape and a stable input.
2. Call the specific check function (`checkWebhookSecrets`, `checkPasswordExposure`, etc.) rather than the umbrella `Analyze`. Tighter assertions, faster feedback.
3. Assert on the number of diagnostics first, then on `Severity`, `Message`, and `Context`. Avoid asserting exact messages: use `strings.Contains` for the substring you care about.

## Runtime tests

Runtime tests use `net/http/httptest`. Two flavors:

- **Unit tests**: drive a single handler with a fabricated request. Example: [`internal/runtime/webhook_test.go`](../../internal/runtime/webhook_test.go) builds a `parser.Webhook`, computes an HMAC, and calls `s.handleWebhook(rr, req, wh)` directly.
- **End-to-end tests**: spin up the full server and hit it over HTTP. Files end in `_e2e_test.go`, e.g. [`webhook_e2e_test.go`](../../internal/runtime/webhook_e2e_test.go), [`requires_clauses_e2e_test.go`](../../internal/runtime/requires_clauses_e2e_test.go), [`stream_e2e_test.go`](../../internal/runtime/stream_e2e_test.go).

Helpers in [`mock_test.go`](../../internal/runtime/mock_test.go) and [`helpers_test.go`](../../internal/runtime/helpers_test.go) build a `*Server` against an in-memory SQLite for both flavors.

## Benchmarks

Standard Go bench format, one file per package: [`internal/parser/bench_test.go`](../../internal/parser/bench_test.go), [`internal/analyzer/bench_test.go`](../../internal/analyzer/bench_test.go), [`internal/runtime/bench_test.go`](../../internal/runtime/bench_test.go). Name benchmarks `BenchmarkX1k`, `BenchmarkX10k`, etc., when scaling input size. Run with:

```bash
go test -bench=. -benchmem ./internal/parser
```

## Fuzzing

The parser has a fuzz harness at [`internal/parser/fuzz_test.go`](../../internal/parser/fuzz_test.go) seeded with realistic `.kilnx` snippets covering each top-level keyword. Run locally:

```bash
go test -fuzz=FuzzParse -fuzztime=30s ./internal/parser
```

Crashes are saved under `internal/parser/testdata/fuzz/`. When a fuzz crash is fixed, the corpus entry is left in place as a permanent regression test.

## Smoke tests

[`smoke/`](../../smoke/) holds tiny `.kilnx` programs the CI pipeline runs end to end. The current entry is [`hello.kilnx`](../../smoke/hello.kilnx):

```kilnx
page /
  html
    <p>Hello World</p>

test "homepage loads"
  visit /
  expect status 200
  expect page / contains "Hello World"
```

CI exercises three paths against this file (see [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml) and [`.github/workflows/declarative-tests.yml`](../../.github/workflows/declarative-tests.yml)):

```bash
./kilnx check smoke/hello.kilnx
./kilnx test  smoke/hello.kilnx
./kilnx build smoke/hello.kilnx -o hello
```

Add a new smoke file when introducing a feature large enough to need an integration story but small enough to live in a self-contained `.kilnx` example. Keep them under fifty lines.

## The `test` keyword inside .kilnx apps

Distinct from Go testing: the `test` block is a feature of the Kilnx language itself. The runner is [`RunTests`](../../internal/runtime/testing.go) in `internal/runtime/testing.go`. Steps include `as <role>`, `visit <path>`, `fill <field> <value>`, `submit`, and `expect status N` / `expect page <path> contains "<text>"`. The full reference is generated at [`docs/devs/reference/keywords/test.md`](../devs/reference/keywords/test.md). When changing the runner, add coverage in [`testing_test.go`](../../internal/runtime/testing_test.go).

## Pre-commit and CI

The repo ships a hook bundle at `.githooks/` enabled via:

```bash
git config core.hooksPath .githooks
```

`pre-commit` runs `gofmt`, `go vet`, and `go run ./cmd/kilnx-gendocs` so spec changes always commit alongside their generated docs. CI re-runs the same checks plus `go test -race ./...`, the smoke pipeline, and `kilnx-gendocs -check-stale`. Keep the local loop tight: `go test -race ./...` should pass before pushing.
