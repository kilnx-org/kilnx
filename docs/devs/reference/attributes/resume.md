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
| **Spec last touched** | `66f909b` (2026-05-13) |
| **Source last touched** | `87ebbf6` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

