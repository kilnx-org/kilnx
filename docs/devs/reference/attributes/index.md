# `index`

> Non-unique composite index for query performance.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
index (<field>, ...)
```

## Description

Declared at model scope. Creates a regular index covering the listed fields in order.

## Used in

- [`model`](../keywords/model.md)

## Examples

### Index on user_id + created_at

```kilnx
model order
  user_id: reference user
  created_at: timestamp auto
  index (user_id, created_at)
```

## Provenance

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `72e9177` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

