# `agent`

> Agentic loop running as a `claude` CLI subprocess.

| | |
|---|---|
| **Kind** | Keyword |
| **Since** | `0.1.3` |

## Syntax

```
agent
```

## Description

Block-form attribute inside `llm`. Spawns the official `claude` CLI (min v2.0.0) per request and streams `stream-json` envelopes back. Requires `max-budget-usd`. The `claude` CLI must be on PATH at runtime; `kilnx run` validates this at startup when any `agent` block is declared.

## Children

- [`cwd`](../attributes/cwd.md)
- [`tools`](../attributes/tools.md)
- [`max-turns`](../attributes/max-turns.md)
- [`max-budget-usd`](../attributes/max-budget-usd.md)
- [`permission-mode`](../attributes/permission-mode.md)
- [`mcp`](../attributes/mcp.md)
- [`pool`](../attributes/pool.md)
- [`pool-idle-ttl`](../attributes/pool-idle-ttl.md)
- [`show-tools`](../attributes/show-tools.md)
- [`resume`](../attributes/resume.md)

## See also

- [`llm`](llm.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `72e9177` (2026-05-13) |
| **Source last touched** | `72e9177` (2026-05-13) |
| **Source files** | `internal/parser/parser.go`, `internal/runtime/server.go` |

