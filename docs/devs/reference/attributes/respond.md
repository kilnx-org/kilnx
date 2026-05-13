# `respond`

> Return a partial HTML response (htmx fragment) or status code.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
respond [fragment <selector> | status <code>]
```

## Description

Without arguments, renders the body's HTML normally. With `fragment <selector>` returns an htmx swap. With `status <code>` returns an explicit HTTP status.

## Used in

- [`page`](../keywords/page.md)
- [`action`](../keywords/action.md)
- [`fragment`](../keywords/fragment.md)
- [`api`](../keywords/api.md)
- [`schedule`](../keywords/schedule.md)
- [`job`](../keywords/job.md)
- [`webhook`](../keywords/webhook.md)

## Examples

### Replace a row via htmx

```kilnx
action /users/:id/toggle method POST
  query: UPDATE user SET active = NOT active WHERE id = :id
  respond fragment ".user-row" query: SELECT * FROM user WHERE id = :id
```

## Provenance

| | |
|---|---|
| **Spec last touched** | `56b81a7` (2026-05-13) |
| **Source last touched** | `2a440f8` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

