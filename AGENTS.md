# Agent and LLM Instructions

This file targets any coding agent (Claude, Cursor, Copilot, Aider, others) working in this repository or generating `.kilnx` code. Treat it as authoritative.

## Before writing any `.kilnx` code

Read, in order:

1. [PRINCIPLES.md](PRINCIPLES.md) — 11 constitutional rules. They override any heuristic.
2. [GRAMMAR.md](GRAMMAR.md) — complete syntax. Do not invent keywords, field types, or modifiers.
3. [FEATURES.md](FEATURES.md) — working feature catalogue with runnable examples.
4. [llms.txt](llms.txt) — index of the above for quick navigation.
5. [llms-full.txt](llms-full.txt) — single-file concatenation when a full context load is needed.

## Hard rules

- **No JavaScript in generated apps.** htmx only for interactivity. If a task seems to require JS, it almost certainly does not.
- **No ORMs, no query builders.** SQL is inline. Use `query name: SELECT ...` inside `page`, `action`, `fragment`, `stream`, `socket`, `api`.
- **Security is built in.** Do not add CSRF tokens, bcrypt hashing, session middleware, or SQL escaping manually. The runtime handles all of that. Adding your own opens holes.
- **Do not import external runtime dependencies.** Kilnx apps are a single `.kilnx` file plus optional imports of other `.kilnx` files. No npm, no pip, no go modules in user code.
- **Respect the keyword set.** Top-level blocks: `config`, `model`, `auth`, `permissions`, `layout`, `page`, `action`, `fragment`, `stream`, `socket`, `api`, `webhook`, `job`, `schedule`, `test`. Anything outside this set is wrong.
- **Field types are finite.** `text`, `email`, `int`, `float`, `bool`, `timestamp`, `richtext`, `option`, `password`, `image`, `phone`. No others.
- **One file can be a complete app.** Default to keeping things in one file. Split into additional `.kilnx` files via `import` only when the size genuinely warrants it.

## When editing the compiler (Go source)

- `internal/lexer/`, `internal/parser/`, `internal/analyzer/`, `internal/runtime/`, `internal/database/`, `internal/lsp/`, `internal/build/` are the module boundaries. Respect them.
- Run `go test -race ./...` before declaring work complete. Target is all 311 tests green.
- Never skip pre-commit hooks (no `--no-verify`).
- Commits follow Conventional Commits. Never add `Co-Authored-By` or any mention of the generating model.

## When in doubt

Read the grammar. If the grammar is silent on the construct you want, the construct does not exist; propose an RFC issue instead of inventing syntax.
