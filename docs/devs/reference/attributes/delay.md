# `delay`

> Seconds to delay over-budget requests before responding.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
delay: <seconds>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `seconds` | `int` | yes |

## Description

If set, requests over the limit are held for `delay` seconds before responding. Useful as a soft limit.

## Used in

- [`limit`](../keywords/limit.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `5da8498` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

