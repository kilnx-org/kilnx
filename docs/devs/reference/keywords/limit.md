# `limit`

> Rate-limit requests matching a path pattern.

| | |
|---|---|
| **Kind** | Keyword |
| **Since** | `0.1.0` |
| **Repeatable** | yes |

## Syntax

```
limit <path-pattern>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `path-pattern` | `string` | yes |

## Description

A `limit` block applies a rate limit to all routes matching the pattern. Limits are scoped per `user`, per `ip`, or globally. Exceeding the budget returns HTTP 429 with the configured `message` (or a default).

## Children

- [`requests`](../attributes/requests.md)
- [`delay`](../attributes/delay.md)
- [`message`](../attributes/message.md)

## Examples

### Throttle the API per user

```kilnx
limit /api/*
  requests: 100 per minute per user
  delay: 10
  message: "Slow down."
```

## See also

- [`requires`](../attributes/requires.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `5da8498` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

