# Contributing to Kilnx

Thanks for your interest in contributing to Kilnx!

## Getting Started

```bash
git clone https://github.com/kilnx-org/kilnx.git
cd kilnx
git config core.hooksPath .githooks   # enable repo-managed git hooks
go build -o kilnx ./cmd/kilnx/
go test -race ./...
```

## Project Structure

```
cmd/
  kilnx/           CLI entrypoint
  kilnx-gendocs/   Reference-doc generator (consumes internal/spec)
internal/
  lexer/           Tokenizer
  parser/          Recursive descent parser + per-keyword *_spec.go
  analyzer/        Type checking and static analysis
  optimizer/       SQL optimization
  runtime/         HTTP server and AST interpreter
  spec/            Language-spec types and registry
docs/devs/         User-facing docs (reference/ subdir is generated)
smoke/             Smoke test .kilnx apps (used by CI)
```

## Guidelines

- Run `go vet ./...` and `gofmt` before submitting
- Add tests for new features
- Keep dependencies minimal (currently only SQLite, pgx and bcrypt)
- Read [PRINCIPLES.md](PRINCIPLES.md) before proposing language changes

## Adding or changing a keyword/attribute

The `.kilnx` language spec is the source of truth for documentation. Each
keyword/attribute is registered once, in a `*_spec.go` file co-located
with its parser implementation.

1. Add or edit the relevant `internal/parser/<name>_spec.go` (or
   `attrs_spec.go` for attributes shared across multiple keywords).
2. Run `go generate ./...` to regenerate `docs/devs/reference/`.
3. Commit the spec change and the regenerated docs together.

The pre-commit hook in `.githooks/pre-commit` does steps 2 and 3
automatically when you stage changes that touch the spec. CI also runs
the generator and fails the PR if `docs/devs/reference/` is out of sync
with the spec.

### Provenance and staleness detection

Each generated reference page includes a **Provenance** section showing:

- **Spec last touched**: short SHA + date of the commit that last edited
  the entity's `*_spec.go` registration.
- **Source last touched**: short SHA + date of the commit that last
  edited any implementation file matching the entity. Source files are
  detected heuristically by grepping `internal/` for the dispatch
  patterns Kilnx uses (`case "<name>":`, `Value == "<name>"`,
  `Value: "<name>"`), excluding `_test.go` and `_spec.go`.
- **Source files**: the matched implementation paths.

If the source was touched **after** the spec, the page renders a
**Stale** warning, suggesting that `Description` may no longer reflect
behavior. CI runs `kilnx-gendocs --check-stale` and fails the PR when
any entity is stale: review the description against current behavior
and update the `*_spec.go` file.

Because source detection is heuristic, when introducing a new dispatch
pattern (a new way the parser/runtime branches on a keyword name),
extend the regex in `cmd/kilnx-gendocs/main.go` (`findSourceFiles`) so
provenance keeps tracking it.

## Reporting Issues

Open an issue on [GitHub](https://github.com/kilnx-org/kilnx/issues) with a minimal `.kilnx` file that reproduces the problem.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
