# `llm`

> Call an LLM. Choose `response` (single-shot / chat) or `agent` (subprocess loop with tools).

| | |
|---|---|
| **Kind** | Keyword |
| **Since** | `0.1.3` |

## Syntax

```
llm <name>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `name` | `identifier` | yes |

## Description

Body-level keyword. Bind the result to `<name>` for use by subsequent nodes. Common settings (`model`, `system`, `temperature`, `max-tokens`) live directly under `llm`; mode-specific children (`response` or `agent`) define how the call runs. Exactly one of `response` or `agent` must be present.

## Children

- [`system`](../attributes/system.md)
- [`temperature`](../attributes/temperature.md)
- [`max-tokens`](../attributes/max-tokens.md)
- [`response`](response.md)
- [`agent`](agent.md)

## Examples

### Streaming chat (response mode)

```kilnx
action /chat method POST
  llm reply
    model: claude-sonnet-4-6
    system: You are a helpful assistant
    response
      history: SELECT papel, conteudo FROM mensagem WHERE conversa_id = :id ORDER BY criada
      stream: #chat-msgs
      stream-swap: append
```

### Agent loop (subprocess claude CLI)

```kilnx
job process-doc
  llm task
    model: claude-sonnet-4-6
    system: Code assistant
    agent
      cwd: /workspaces/jobs/:doc_id
      tools: read, write, bash
      max-turns: 10
      max-budget-usd: 0.50
      permission-mode: plan
```

## See also

- [`response`](response.md)
- [`agent`](agent.md)
- [`fetch`](../attributes/fetch.md)

## Provenance

| | |
|---|---|
| **Source last touched** | `5da8498` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

