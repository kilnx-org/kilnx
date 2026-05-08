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

