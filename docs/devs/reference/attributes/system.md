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

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-21`, but this entity's spec was last edited on `2026-05-20`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `36bcf1b` (2026-05-20) |
| **Source last touched** | `9276716` (2026-05-21) |
| **Source files** | `internal/parser/parser.go`, `internal/runtime/llm_agent.go` |

