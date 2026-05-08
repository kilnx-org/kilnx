# `on`

> Conditional or event handler.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |
| **Repeatable** | yes |

## Syntax

```
on <condition-or-event>
```

## Description

Inside a body, branches on the result of the previous step: `on success`, `on error`, `on not found`, `on forbidden`. Inside a `socket` or `webhook`, dispatches on connection events (`on connect`, `on message`, `on disconnect`) or named webhook events (`on event <name>`).

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

### Branch on action outcome

```kilnx
action /users/create method POST
  validate user
  query: INSERT INTO user (...) VALUES (...)
  on success
    redirect /users
  on error
    respond fragment ".form" query: SELECT * FROM user WHERE id = :id
```

