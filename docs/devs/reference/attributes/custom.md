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

| | |
|---|---|
| **Spec last touched** | `e1d0f3f` (2026-05-08) |
| **Source last touched** | `b2cecfb` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

