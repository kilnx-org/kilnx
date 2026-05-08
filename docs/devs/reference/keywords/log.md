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

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `5da8498` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

