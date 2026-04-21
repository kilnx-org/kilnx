# CLI

## kilnx run

Start a development server with hot reload.

```bash
kilnx run app.kilnx
```

- Auto-creates the database on first run
- Applies pending migrations
- Watches `app.kilnx` and reloads on change
- Default port: 8080 (override with `PORT` env or `config port:`)

## kilnx build

Compile to a standalone binary.

```bash
kilnx build app.kilnx -o myapp
./myapp
```

The output binary embeds the parsed app, htmx, and the Kilnx runtime. ~15MB, no runtime dependencies. Target platform follows the host (cross-compile via Go&apos;s `GOOS`/`GOARCH`).

## kilnx check

Static analysis.

```bash
kilnx check app.kilnx
```

Validates: grammar, field type usage, SQL parameter binding, template variable references, security concerns (password fields in public queries, unsafe `raw` filter usage), and tenant scoping.

Exit code: 0 on clean, 1 on errors. Warnings do not fail.

Options:

```bash
kilnx check app.kilnx --db sqlite://app.db    # validate against live DB
```

## kilnx test

Run `test` blocks declaratively.

```bash
kilnx test app.kilnx
```

Spins up an in-memory server with a fresh SQLite database, executes each test as HTTP requests, reports pass/fail counts. No mocks. Tests run against the real routing, queries, and template engine.

## kilnx migrate

Schema migrations. Normally automatic during `run`; use this for production.

```bash
kilnx migrate app.kilnx                # apply pending
kilnx migrate app.kilnx --dry-run      # show SQL without applying
kilnx migrate app.kilnx --status       # show applied migrations
```

Detects: added fields (emits `ALTER TABLE ADD COLUMN`), new models (emits `CREATE TABLE`). Does not detect: removed or renamed fields.

## kilnx lsp

Language Server Protocol endpoint. For editor integrations.

```bash
kilnx lsp
```

Speaks LSP over stdin/stdout. Used by the [VS Code extension](https://marketplace.visualstudio.com/items?itemName=atoolz.kilnx-vscode-toolkit). Provides: completions, diagnostics, hover docs, go-to-definition, document symbols.

## kilnx mcp

Model Context Protocol server. For AI tools (Claude, Cursor, etc.).

```bash
kilnx mcp
```

Exposes the grammar spec, keyword docs, example snippets, and the `check` tool to LLM clients. Configure in your MCP client&apos;s settings.

## kilnx version

```bash
kilnx version
```

Prints the version and git commit hash.

## Global flags

| Flag | Effect |
|------|--------|
| `--help` | Show command help |
| `--verbose` | Verbose output |

## Environment variables

Most config values are overridable via env vars declared in the `config` block:

```kilnx
config
  database: env DATABASE_URL default "sqlite://app.db"
  port: env PORT default 8080
  secret: env SECRET_KEY required
```

`env VAR required` fails fast if the variable is unset. Useful for production secrets.
