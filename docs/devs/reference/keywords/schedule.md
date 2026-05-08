# `schedule`

> Background task executed on a fixed interval or cron expression.

| | |
|---|---|
| **Kind** | Keyword |
| **Since** | `0.1.0` |
| **Repeatable** | yes |

## Syntax

```
schedule <name> every <duration|cron>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `name` | `identifier` | yes |
| `duration` | `string` | yes |

## Description

The runtime invokes the schedule's body at the cadence declared by `every`. Bodies typically run cleanup queries, send periodic emails, or enqueue jobs. Schedules execute in-process; for distributed setups, see `webhook` or external schedulers.

## Children

- [`every`](../attributes/every.md)
- [`redirect`](../attributes/redirect.md)

## Examples

### Hourly session cleanup

```kilnx
schedule cleanup-sessions every 1h
  query: DELETE FROM session WHERE expires_at < now()
```

## See also

- [`job`](job.md)
- [`stream`](stream.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `5da8498` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

