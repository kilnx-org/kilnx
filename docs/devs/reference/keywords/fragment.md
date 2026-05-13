# `fragment`

> Return partial HTML (no document wrapper) for htmx or includes.

| | |
|---|---|
| **Kind** | Keyword |
| **Since** | `0.1.0` |
| **Repeatable** | yes |

## Syntax

```
fragment <path-or-name> [(<args>)] [requires <clause>]
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `path-or-name` | `string` | yes |

## Description

A `fragment` returns a slice of HTML without `<html>`/`<body>` wrapping. Fragments come in two flavors: route-based (`fragment <path>`), used as htmx targets that respond to AJAX requests, and component-based (`fragment <name>(<args>)`), used as reusable template includes invoked from other fragments or pages.

## Children

- [`requires`](../attributes/requires.md)
- [`redirect`](../attributes/redirect.md)

## Examples

### Route-based htmx fragment

```kilnx
fragment /users/:id/card
  query user: SELECT name, email FROM user WHERE id = :id
  html
    <div class="card">{user.name}</div>
```

### Reusable component

```kilnx
fragment badge(status, color="blue")
  html
    <span class="{color}">{status}</span>
```

## See also

- [`page`](page.md)
- [`action`](action.md)
- [`html`](../attributes/html.md)
- [`api`](api.md)
- [`query`](query.md)

## Provenance

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `69981b8` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

