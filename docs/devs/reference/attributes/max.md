# `max`

> Maximum value (numeric) or length (string).

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
<field>: <type> max <n>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `n` | `int` | yes |

## Description

Mirror of `min`. For string fields, this also influences column size where applicable.

## Used in

- [`model`](../keywords/model.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `e1d0f3f` (2026-05-08) |
| **Source last touched** | `74103b0` (2026-05-08) |
| **Source files** | `internal/parser/parser.go`, `internal/runtime/forms.go` |

