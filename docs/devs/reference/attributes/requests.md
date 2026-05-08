# `requests`

> Rate-limit budget (in `limit`) or request log strategy (in `log`).

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |
| **Required** | yes |

## Syntax

```
requests: <n> per <window> per <scope>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `n` | `int` | yes |

## Description

Inside a `limit` block: maximum number of allowed requests per window per scope, e.g. `requests: 100 per minute per user`. Excess requests return HTTP 429 (or are delayed if `delay` is set).

When used in `log`: Inside a `log` block: which requests to log. One of `all`, `errors`, or `none`.

## Used in

- [`limit`](../keywords/limit.md)
- [`log`](../keywords/log.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `e1d0f3f` (2026-05-08) |
| **Source last touched** | `b2cecfb` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

