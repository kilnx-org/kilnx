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

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `69981b8` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

