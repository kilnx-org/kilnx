# Contributing to Kilnx

Thanks for your interest in contributing to Kilnx!

## Getting Started

```bash
git clone https://github.com/kilnx-org/kilnx.git
cd kilnx
go build -o kilnx ./cmd/kilnx/
go test -race ./...
```

## Project Structure

```
cmd/kilnx/    CLI entrypoint
internal/     Core implementation
  lexer/      Tokenizer
  parser/     Recursive descent parser
  analyzer/   Type checking and static analysis
  optimizer/  SQL optimization
  runtime/    HTTP server and AST interpreter
  lsp/        Language Server Protocol
examples/     Example .kilnx apps
```

## Guidelines

- Run `go vet ./...` and `gofmt` before submitting
- Add tests for new features
- Keep dependencies minimal (currently only SQLite and bcrypt)
- Read [PRINCIPLES.md](PRINCIPLES.md) before proposing language changes

## Reporting Issues

Open an issue on [GitHub](https://github.com/kilnx-org/kilnx/issues) with a minimal `.kilnx` file that reproduces the problem.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
