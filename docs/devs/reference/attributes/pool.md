# `pool`

> Reserved: number of warm `claude` subprocesses to keep alive (not yet implemented).

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.3` |

## Syntax

```
pool: <int>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `value` | `int` | no |

## Description

Reserved syntax in v0.1.3. Analyzer emits a `not yet implemented` warning; runtime ignores the value and always spawns per request.

## Used in

- [`agent`](../keywords/agent.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `cd7b558` (2026-05-19) |
| **Source last touched** | `d461e57` (2026-05-19) |
| **Source files** | `internal/parser/parser.go` |

