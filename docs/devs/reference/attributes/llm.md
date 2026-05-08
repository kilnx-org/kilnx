# `llm`

> Call a Large Language Model with optional history and system prompt.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
llm [<name>:] <model>
```

## Description

Sub-keywords: `history: <SQL>` (rows become messages), `system: <text>` (system prompt). The named result is bound for use in subsequent nodes. Model providers are configured via env vars at runtime.

## Used in

- [`page`](../keywords/page.md)
- [`action`](../keywords/action.md)
- [`fragment`](../keywords/fragment.md)
- [`api`](../keywords/api.md)
- [`schedule`](../keywords/schedule.md)
- [`job`](../keywords/job.md)
- [`webhook`](../keywords/webhook.md)

## Examples

### Customer support chat

```kilnx
action /chat method POST
  llm chat: claude-sonnet-4-6
    history: SELECT role, content FROM message WHERE conversation_id = :id ORDER BY created
    system: You are a helpful customer support agent
```

