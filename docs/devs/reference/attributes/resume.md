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

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-19`, but this entity's spec was last edited on `2026-05-13`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `72e9177` (2026-05-13) |
| **Source last touched** | `d461e57` (2026-05-19) |
| **Source files** | `internal/parser/parser.go` |

