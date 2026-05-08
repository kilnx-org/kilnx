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

## Provenance

| | |
|---|---|
| **Spec last touched** | `e1d0f3f` (2026-05-08) |
| **Source last touched** | `b2cecfb` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

