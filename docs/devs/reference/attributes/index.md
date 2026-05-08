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

| | |
|---|---|
| **Spec last touched** | `e1d0f3f` (2026-05-08) |
| **Source last touched** | `b2cecfb` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

