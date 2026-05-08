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

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `5da8498` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

