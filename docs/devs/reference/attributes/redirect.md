# `redirect`

> Redirect to another URL.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
redirect <path>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `path` | `path` | yes |

## Description

Issues an HTTP redirect from inside a page or action body. Used in flows that change state and then send the user elsewhere.

## Used in

- [`page`](../keywords/page.md)
- [`action`](../keywords/action.md)

## Examples

### Redirect after successful action

```kilnx
action /logout method POST
  redirect /entrar
```

## See also

- [`action`](../keywords/action.md)

