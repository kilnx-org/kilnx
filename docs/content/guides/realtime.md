# Realtime

> This page is a stub. Full content coming soon.

## Server-Sent Events (SSE)

`stream` creates an SSE endpoint that polls a SQL query at a fixed interval:

```kilnx
stream /notifications requires auth every 5s
  query: SELECT message FROM notification
         WHERE user_id = :current_user.id AND seen = false
```

The client connects via the embedded htmx SSE extension. Updates stream automatically.

## WebSockets

`socket` creates a bidirectional WebSocket endpoint with rooms and broadcast:

```kilnx
socket /chat/:room requires auth
  on connect
    query: SELECT body, author FROM message
           WHERE room = :room ORDER BY created DESC LIMIT 50
  on message
    query: INSERT INTO message (body, author, room)
           VALUES (:body, :current_user.id, :room)
    broadcast to :room
```

Handlers: `on connect`, `on message`, `on disconnect`. Each runs SQL and optionally broadcasts to other clients in the same room.
