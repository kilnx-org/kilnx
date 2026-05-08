# `stream`

> Server-Sent Events (SSE) endpoint that pushes data on an interval.

| | |
|---|---|
| **Kind** | Keyword |
| **Since** | `0.1.0` |
| **Repeatable** | yes |

## Syntax

```
stream <path> [requires <clause>]
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `path` | `path` | yes |

## Description

A `stream` is an HTTP endpoint that holds the connection open and pushes events to the client at the cadence given by `every`. On each tick the declared `query` runs and its results are emitted as the named `event`. Useful for live counters, notification feeds, and polling dashboards without writing client-side JavaScript.

## Children

- [`every`](../attributes/every.md)
- [`event`](../attributes/event.md)
- [`requires`](../attributes/requires.md)

## Examples

### Notifications feed

```kilnx
stream /notifications requires auth
  query: SELECT message, created_at FROM notifications WHERE seen = false
  every 5s
  event: message
```

## See also

- [`socket`](socket.md)
- [`page`](page.md)

