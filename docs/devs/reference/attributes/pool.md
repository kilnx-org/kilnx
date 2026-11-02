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

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-21`, but this entity's spec was last edited on `2026-05-20`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `36bcf1b` (2026-05-20) |
| **Source last touched** | `9276716` (2026-05-21) |
| **Source files** | `internal/parser/parser.go` |

