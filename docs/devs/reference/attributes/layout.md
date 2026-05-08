# `layout`

> Render the page inside a named layout.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
layout <name>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `name` | `identifier` | yes |

## Description

References a top-level `layout` block by name. The page's HTML is rendered inside the layout's `{{ content }}` slot.

## Used in

- [`page`](../keywords/page.md)

## Examples

### Use the app layout

```kilnx
page /dashboard layout app
  html
    <h1>Dashboard</h1>
```

## See also

- [`title`](title.md)

