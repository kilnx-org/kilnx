# `method`

> Restrict the HTTP method for the parent route.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |
| **Default** | `GET` |

## Syntax

```
method <verb>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `verb` | `string` | yes |

## Description

Specifies which HTTP verb the parent endpoint accepts. When omitted, defaults to `GET`.

## Used in

- [`page`](../keywords/page.md)
- [`action`](../keywords/action.md)
- [`api`](../keywords/api.md)

## Examples

### POST page

```kilnx
page /contact method POST
  html
    <p>Form submitted.</p>
```

## See also

- [`requires`](requires.md)

## Provenance

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `2a440f8` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

