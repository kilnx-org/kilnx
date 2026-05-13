# `socket`

> WebSocket endpoint for bidirectional real-time messaging.

| | |
|---|---|
| **Kind** | Keyword |
| **Since** | `0.1.0` |
| **Repeatable** | yes |

## Syntax

```
socket <path> [requires <clause>]
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `path` | `path` | yes |

## Description

A `socket` opens a WebSocket connection at the given path. Client events dispatch to `on connect`, `on message`, and `on disconnect` handlers. Use `broadcast to :<room>` from any action or socket handler to push to all connected clients in a room.

## Children

- [`requires`](../attributes/requires.md)
- [`on`](../attributes/on.md)
- [`broadcast`](../attributes/broadcast.md)

## Examples

### Chat room socket

```kilnx
socket /chat/:room requires auth
  on connect
    query: INSERT INTO chat_member (room_id, user_id) VALUES (:room, :current_user_id)
  on message
    query: INSERT INTO message (room_id, user_id, text) VALUES (:room, :current_user_id, :text)
```

## See also

- [`stream`](stream.md)
- [`broadcast`](../attributes/broadcast.md)
- [`on`](../attributes/on.md)

## Provenance

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `72e9177` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

