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

## Children

- [`method`](../attributes/method.md)
- [`requires`](../attributes/requires.md)
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
- [`layout`](layout.md)
- [`stream`](stream.md)
- [`query`](query.md)

## Provenance

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `c92fd85` (2026-05-13) |
| **Source files** | `internal/parser/parser.go`, `internal/runtime/render.go` |

