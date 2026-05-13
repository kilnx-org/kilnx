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

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `72e9177` (2026-05-13) |
| **Source files** | `internal/parser/parser.go`, `internal/runtime/forms.go` |

