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

> ⚠ **Implementation touched after spec.** Source code changed on `2026-11-02`, but this entity's spec was last edited on `2026-05-20`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `36bcf1b` (2026-05-20) |
| **Source last touched** | `abd91de` (2026-11-02) |
| **Source files** | `internal/parser/parser.go`, `internal/runtime/scheduler.go`, `internal/runtime/server.go` |

