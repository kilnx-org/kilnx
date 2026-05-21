# `custom`

> Load custom field definitions from an external manifest.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
custom "<path>"
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `path` | `string` | yes |

## Description

Resolves at parse time from a path that may contain `{placeholder}` segments.

## Used in

- [`model`](../keywords/model.md)

## Provenance

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-20`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `af278bb` (2026-05-20) |
| **Source files** | `internal/parser/parser.go` |

