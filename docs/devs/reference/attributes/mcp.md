# `mcp`

> Comma-separated names of MCP servers declared in `config mcp <name>`.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.3` |

## Syntax

```
mcp: <name>, <name>, ...
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `names` | `list` | yes |

## Description

Each name must resolve to an MCP server block in the top-level `config`. The matching servers are mounted into the agent via `--mcp-config`.

## Used in

- [`agent`](../keywords/agent.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `56b81a7` (2026-05-13) |
| **Source last touched** | `2a440f8` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

