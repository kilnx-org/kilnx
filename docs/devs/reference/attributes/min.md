# `min`

> Minimum value (numeric) or length (string).

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
<field>: <type> min <n>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `n` | `int` | yes |

## Description

Numeric fields enforce `value >= n`; string-like fields enforce length >= n. Validation happens at `validate` time and at the runtime layer.

## Used in

- [`model`](../keywords/model.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `e1d0f3f` (2026-05-08) |
| **Source last touched** | `b2cecfb` (2026-05-08) |
| **Source files** | `internal/parser/parser.go`, `internal/runtime/forms.go` |

