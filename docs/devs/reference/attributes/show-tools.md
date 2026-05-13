# `show-tools`

> Stream tool_use/tool_result frames in addition to assistant text.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.3` |
| **Default** | `false` |

## Syntax

```
show-tools: <bool>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `value` | `bool` | yes |

## Description

When true and the surrounding `response` block has `stream:` set, tool-call and tool-result events are emitted on a separate hyperstream channel (`tools-<name>`). Default `false`: tool frames are silently dropped.

## Used in

- [`agent`](../keywords/agent.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `7df9033` (2026-05-13) |

