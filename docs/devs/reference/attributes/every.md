# `every`

> Cadence at which the parent task runs.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
every <duration|cron>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `duration` | `string` | yes |

## Description

Accepts a duration like `5s`, `1m`, `1h`, `24h` or a cron expression (in `schedule`). For `stream`, controls how often the query is executed and pushed to clients.

## Used in

- [`stream`](../keywords/stream.md)
- [`schedule`](../keywords/schedule.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `5da8498` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

