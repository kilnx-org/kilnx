# `superuser`

> Identity that bypasses role checks.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
superuser: env <VAR>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `value` | `string` | yes |

## Description

Typically pulled from an env var. The matching user is treated as having every role.

## Used in

- [`auth`](../keywords/auth.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `e1d0f3f` (2026-05-08) |
| **Source last touched** | `b2cecfb` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

