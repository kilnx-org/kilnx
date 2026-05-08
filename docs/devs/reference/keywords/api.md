# `api`

> Declare a JSON endpoint.

| | |
|---|---|
| **Kind** | Keyword |
| **Since** | `0.1.0` |
| **Repeatable** | yes |

## Syntax

```
api <path> [method <verb>] [requires <clause>]
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `path` | `path` | yes |

## Description

An `api` block defines a JSON-returning HTTP endpoint. Query results and computed values are serialized to JSON automatically. Use `api` for machine-to-machine integrations or SPA backends; use `page` for HTML, `action` for state-changing flows, and `fragment` for htmx.

## Children

- [`method`](../attributes/method.md)
- [`requires`](../attributes/requires.md)
- [`redirect`](../attributes/redirect.md)

## Examples

### List users as JSON

```kilnx
api /api/v1/users requires auth
  query users: SELECT id, name, email FROM user ORDER BY id
```

## See also

- [`page`](page.md)
- [`action`](action.md)
- [`fragment`](fragment.md)

