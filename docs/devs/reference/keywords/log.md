# `log`

> Configure runtime logging output.

| | |
|---|---|
| **Kind** | Keyword |
| **Since** | `0.1.0` |

## Syntax

```
log
```

## Description

The `log` block sets log verbosity, slow-query threshold, request log level, and error reporting (with optional stack traces). Logs are emitted in a structured format suitable for ingestion by stdout collectors.

## Children

- [`level`](../attributes/level.md)
- [`slow_query`](../attributes/slow_query.md)
- [`requests`](../attributes/requests.md)
- [`errors`](../attributes/errors.md)

## Examples

### Verbose dev logging

```kilnx
log
  level: debug
  slow-query: 100ms
  requests: all
  errors: all stacktrace
```

## Provenance

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `2a440f8` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

