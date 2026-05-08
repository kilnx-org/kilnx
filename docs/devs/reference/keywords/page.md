# `page`

> Define an HTTP route and its view.

| | |
|---|---|
| **Kind** | Keyword |
| **Since** | `0.1.0` |
| **Repeatable** | yes |

## Syntax

```
page <path> [layout <name>] [title <text>] [requires <clause>] [method <verb>]
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `path` | `path` | yes |

## Description

A `page` declares a URL path that the Kilnx runtime serves over HTTP. Its body contains the rendered view (HTML, fragments) and optional data-loading directives (queries, redirects, validations).

## Attributes

- [`method`](../attributes/method.md)
- [`requires`](../attributes/requires.md)
- [`layout`](../attributes/layout.md)
- [`title`](../attributes/title.md)
- [`redirect`](../attributes/redirect.md)

## Examples

### Hello world

```kilnx
page /
  html
    <h1>Hello World</h1>
```

### Authenticated page with layout and title

```kilnx
page /dashboard layout app title "Dashboard" requires auth
  query @user_count: SELECT count(*) FROM users
  html
    <h1>{{ .user_count }} users</h1>
```

## See also

- [`action`](action.md)
- [`fragment`](fragment.md)
- [`api`](api.md)
- [`layout`](../attributes/layout.md)

