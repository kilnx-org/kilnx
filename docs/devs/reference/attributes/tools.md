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
| **Spec last touched** | `56b81a7` (2026-05-13) |
| **Source last touched** | `2a440f8` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

