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

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `87ebbf6` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

