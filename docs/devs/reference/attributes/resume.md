# `resume`

> Resume an existing claude session by UUID (supports `:param`).

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.3` |

## Syntax

```
resume: <session-id>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `value` | `string` | yes |

## Description

Maps to `claude --resume <id>`. The id is a UUIDv4 string previously returned by the CLI; bind variables (`:session_id`) are substituted before invocation.

## Used in

- [`agent`](../keywords/agent.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `36bcf1b` (2026-05-20) |
| **Source last touched** | `af278bb` (2026-05-20) |
| **Source files** | `internal/parser/parser.go` |

