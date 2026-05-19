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

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-19`, but this entity's spec was last edited on `2026-05-13`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `72e9177` (2026-05-13) |
| **Source last touched** | `d461e57` (2026-05-19) |
| **Source files** | `internal/parser/parser.go` |

