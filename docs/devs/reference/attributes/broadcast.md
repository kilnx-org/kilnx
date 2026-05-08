# `broadcast`

> Push a message to all clients in a websocket room.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
broadcast to :<room> [fragment <name>]
```

## Description

Renders the named fragment server-side and sends the resulting HTML to all socket clients subscribed to the room.

## Used in

- [`page`](../keywords/page.md)
- [`action`](../keywords/action.md)
- [`fragment`](../keywords/fragment.md)
- [`api`](../keywords/api.md)
- [`schedule`](../keywords/schedule.md)
- [`job`](../keywords/job.md)
- [`webhook`](../keywords/webhook.md)
- [`socket`](../keywords/socket.md)

## Examples

### Broadcast a chat fragment

```kilnx
socket /chat/:room
  on message
    query: INSERT INTO message ...
    broadcast to :room fragment chat-message
```

## Provenance

| | |
|---|---|
| **Spec last touched** | `e1d0f3f` (2026-05-08) |
| **Source last touched** | `b2cecfb` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

