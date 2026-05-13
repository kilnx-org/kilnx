# `title`

> Set the HTML document title.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
title "<text>"
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `text` | `string` | yes |

## Description

Sets the `<title>` tag for the rendered page. Accepts a quoted string or bare identifier.

## Used in

- [`page`](../keywords/page.md)

## Examples

### Page with title

```kilnx
page / title "Home"
  html
    <h1>Welcome</h1>
```

## See also

- [`layout`](../keywords/layout.md)

## Provenance

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `87ebbf6` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

