# `response`

> Single-shot / chat / streaming Messages API call.

| | |
|---|---|
| **Kind** | Keyword |
| **Since** | `0.1.3` |

## Syntax

```
response
```

## Description

Block-form attribute inside `llm`. Children: `history` (SQL that yields message rows), `stream` (CSS selector to enable streaming via hyperstream envelopes), `stream-swap` (hyperstream swap style).

## Children

- [`history`](../attributes/history.md)
- [`stream-swap`](../attributes/stream-swap.md)

## See also

- [`llm`](llm.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `72e9177` (2026-05-13) |
| **Source last touched** | `72e9177` (2026-05-13) |
| **Source files** | `internal/parser/parser.go`, `internal/runtime/server.go` |

