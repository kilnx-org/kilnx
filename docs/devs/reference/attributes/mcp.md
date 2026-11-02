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

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-21`, but this entity's spec was last edited on `2026-05-20`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `36bcf1b` (2026-05-20) |
| **Source last touched** | `9276716` (2026-05-21) |
| **Source files** | `internal/parser/parser.go` |

