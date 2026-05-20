# `system`

> System prompt for this `llm` call.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.3` |

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
| **Spec last touched** | `cd7b558` (2026-05-19) |
| **Source last touched** | `173d8b6` (2026-05-19) |
| **Source files** | `internal/parser/parser.go`, `internal/runtime/llm_agent.go` |

