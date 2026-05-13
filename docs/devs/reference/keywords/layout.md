# `layout`

> Define a reusable HTML wrapper for pages.

| | |
|---|---|
| **Kind** | Keyword |
| **Since** | `0.1.0` |
| **Repeatable** | yes |

## Syntax

```
layout <name>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `name` | `identifier` | yes |

## Description

A `layout` block declares a named HTML template that wraps the body of any page that opts into it via `page <path> layout <name>`. The layout's body usually contains `<html>`, `<head>`, and a placeholder for page content (`{page.content}`). Layouts may also load shared data via `query` nodes.

## Examples

### Define and use a layout

```kilnx
layout main
  html
    <html>
      <head><title>{page.title}</title></head>
      <body>{page.content}</body>
    </html>

page /dashboard layout main title "Dashboard"
  html
    <h1>Welcome</h1>
```

## See also

- [`page`](page.md)
- [`html`](../attributes/html.md)
- [`title`](../attributes/title.md)

## Provenance

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `87ebbf6` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

