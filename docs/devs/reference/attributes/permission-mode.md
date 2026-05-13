# `permission-mode`

> Tool-use permission policy: `plan`, `acceptEdits`, or `bypassPermissions`.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.3` |
| **Default** | `plan` |

## Syntax

```
permission-mode: <mode>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `mode` | `enum` | yes |

## Description

Default `plan` (no side effects until user confirms). `acceptEdits` auto-approves file edits inside `cwd`. `bypassPermissions` skips all prompts (dangerous; analyzer warns).

## Used in

- [`agent`](../keywords/agent.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `56b81a7` (2026-05-13) |

