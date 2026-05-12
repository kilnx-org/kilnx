# `mcp`

> Comma-separated names of MCP servers declared in `config mcp <name>`.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.2.0` |

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
| **Source last touched** | `5da8498` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

