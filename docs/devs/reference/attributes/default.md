# `default`

> Default value when no value is supplied.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
<field>: <type> default <value>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `value` | `literal` | yes |

## Description

Applied at the database level (`DEFAULT` clause). The literal must match the field type.

## Used in

- [`model`](../keywords/model.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `e1d0f3f` (2026-05-08) |
| **Source last touched** | `b2cecfb` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

