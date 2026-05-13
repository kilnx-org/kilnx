# `tools`

> Comma-separated list of tools the agent may use.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.3` |

## Syntax

```
tools: <name>, <name>, ...
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `names` | `list` | no |

## Description

Mapped to the `claude` CLI `--allowedTools` flag. Common values: `read`, `write`, `bash`, `web_search`, `glob`, `grep`. An empty list disables all tools (planning-only).

## Used in

- [`agent`](../keywords/agent.md)

## Provenance

| | |
|---|---|
| **Source last touched** | `5da8498` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

