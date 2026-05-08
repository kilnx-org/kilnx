# `unique`

> Field value must be unique across all rows.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
<field>: <type> unique
```

## Description

Single-field unique constraint. For composite uniqueness, use a model-level `unique (field1, field2)` directive instead.

## Used in

- [`model`](../keywords/model.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `5da8498` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

