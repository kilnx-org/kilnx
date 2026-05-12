# `system`

> System prompt for this `llm` call.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.2.0` |

## Syntax

```
system: <text>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `text` | `string` | yes |

## Description

Free-form text. May contain `:param` substitutions resolved against the surrounding action's params.

## Used in

- [`llm`](../keywords/llm.md)

## Provenance

| | |
|---|---|
| **Source last touched** | `5da8498` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

