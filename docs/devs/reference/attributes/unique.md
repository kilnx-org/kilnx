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

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `aef0ef5` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

