# `auto`

> Auto-populate the field on insert.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
<field>: <type> auto
```

## Description

Behavior depends on field type: `uuid` generates a v4 UUID, `timestamp` sets the current time, integer ID fields auto-increment.

## Used in

- [`model`](../keywords/model.md)

## Provenance

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `72e9177` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

