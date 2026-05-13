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

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `aef0ef5` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

