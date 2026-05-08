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

