# `enqueue`

> Enqueue an asynchronous `job` with named parameters.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
enqueue <job-name>
```

## Description

Indented children are `param: value` pairs. Parameters can be literals, `:bindings`, or query results.

## Used in

- [`page`](../keywords/page.md)
- [`action`](../keywords/action.md)
- [`fragment`](../keywords/fragment.md)
- [`api`](../keywords/api.md)
- [`schedule`](../keywords/schedule.md)
- [`job`](../keywords/job.md)
- [`webhook`](../keywords/webhook.md)

## Examples

### Enqueue a report job

```kilnx
action /reports method POST
  enqueue generate-report
    start_date: :start_date
    requested_by: :current_user_email
```

