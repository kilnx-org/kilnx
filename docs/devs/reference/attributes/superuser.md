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

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `69981b8` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

