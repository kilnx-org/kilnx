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

| | |
|---|---|
| **Spec last touched** | `e1d0f3f` (2026-05-08) |
| **Source last touched** | `b2cecfb` (2026-05-08) |
| **Source files** | `internal/parser/parser.go`, `internal/runtime/forms.go` |

