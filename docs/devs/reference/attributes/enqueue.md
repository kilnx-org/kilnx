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

## See also

- [`job`](../keywords/job.md)

## Provenance

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-20`, but this entity's spec was last edited on `2026-05-13`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `72e9177` (2026-05-13) |
| **Source last touched** | `af278bb` (2026-05-20) |
| **Source files** | `internal/parser/parser.go` |

