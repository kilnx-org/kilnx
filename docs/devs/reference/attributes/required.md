# `required`

> Field must have a non-null value.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
<field>: <type> required
```

## Description

Maps to a `NOT NULL` column in the database; also enforced by `validate`.

## Used in

- [`model`](../keywords/model.md)

## Examples

### Required email

```kilnx
model user
  email: email required
```

## Provenance

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `2a440f8` (2026-05-13) |
| **Source files** | `internal/parser/parser.go`, `internal/runtime/forms.go` |

