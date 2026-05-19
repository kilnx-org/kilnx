# `on progress`

> Handler block run on each progress event while an `agent` runs.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.4` |

## Syntax

```
on progress
  <body>
```

## Description

Block-form child of [`agent`](../keywords/agent.md). The body is a normal kilnx body (typically a `query` or `broadcast`). It runs once per progress event emitted while the agent subprocess runs. Inside the body these bindings are available, in addition to the surrounding params:

- `:progress`, short human-readable line.
- `:progress_kind`, one of `session`, `text`, `assistant`, `tool`, `done`.
- `:cost_usd`, running cost in USD.
- `:session_id`, claude session id, populated once the `session` event fires (the first event the subprocess emits, before any text/tool).

A failing handler body is logged and does not abort the agent or the job.

## Used in

- [`agent`](../keywords/agent.md)

## Examples

### Persist progress for a polling UI

```kilnx
job run-review
  llm review
    model: claude-sonnet-4-6
    agent
      cwd: :bundle_dir
      max-budget-usd: 2.00
      on progress
        query: UPDATE review_job SET progress = :progress WHERE id = :job_id
```

## Provenance

| | |
|---|---|
| **Spec last touched** | `d050f4c` (2026-05-19) |
| **Source last touched** | `d050f4c` (2026-05-19) |
| **Source files** | `internal/parser/parser.go`, `internal/runtime/server.go` |
