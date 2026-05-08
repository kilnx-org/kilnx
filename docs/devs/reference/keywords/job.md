# `job`

> Asynchronous background task triggered by `enqueue`.

| | |
|---|---|
| **Kind** | Keyword |
| **Since** | `0.1.0` |
| **Repeatable** | yes |

## Syntax

```
job <name>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `name` | `identifier` | yes |

## Description

A `job` runs asynchronously when called via `enqueue <job>`. Jobs are useful for slow operations (sending email batches, generating reports) that should not block HTTP responses. Failed jobs retry up to `retry <n>` times.

## Children

- [`retry`](../attributes/retry.md)
- [`redirect`](../attributes/redirect.md)

## Examples

### Generate and email a report

```kilnx
job generate-report
  retry 3
  query: SELECT * FROM order WHERE created > :start_date
  send email to :requested_by
    subject: "Your report is ready"
```

## See also

- [`schedule`](schedule.md)
- [`enqueue`](../attributes/enqueue.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `e1d0f3f` (2026-05-08) |
| **Source last touched** | `b2cecfb` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

