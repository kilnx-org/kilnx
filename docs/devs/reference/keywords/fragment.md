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

### Component with slots (block-form call)

Component fragments can expose `{{slot}}` placeholders so callers supply custom markup. Default slot `{{slot}}` accepts arbitrary content; named slots use `{{slot name="X"}}`. Both forms accept fallback content via `{{slot}}fallback{{/slot}}`.

```kilnx
fragment Card
  html
    <div class="card">
      <header>{{slot name="header"}}Default Title{{/slot}}</header>
      <main>{{slot}}</main>
    </div>
```

Invoke in block form with `{{Name args}}...{{/Name}}`:

```kilnx
{{Card}}
  {{slot name="header"}}Custom Title{{/slot}}
  <p>Body content fills the default slot.</p>
{{/Card}}
```

Slots compose with `query`/`list` args. A generic kanban renders its caller's per-item markup inside `{{each items}}`:

```kilnx
fragment DvKanban(items)
  html
    <div class="kanban">
      {{each items}}<article>{{slot}}</article>{{end}}
    </div>
```

```kilnx
page /people
  query people: SELECT id, name FROM person
  html
    {{DvKanban items=people}}
      <h3>{name}</h3>
    {{/DvKanban}}
```

Notes:

- A slot the caller did not provide falls back to the fragment's fallback content (or is removed if there is none).
- Self-closing `{{Name args}}` keeps existing behavior; slot markers in the fragment fall back to their defaults.
- Slot body content is rendered in the fragment's scope, so refs like `{name}` inside `{{each items}}` resolve against the iterated row.

### Slot forwarding

A wrapper fragment can forward its own named slot content into a nested fragment call with `{{forward name="X"}}`. Use this to delegate a named slot to a child fragment without re-declaring its markup:

```kilnx
fragment DetailTopbar()
  html
    <div class="topbar">
      <div>{{slot name="breadcrumb"}}{{/slot}}</div>
      <div>{{slot name="actions"}}{{/slot}}</div>
    </div>

fragment DetailHeader()
  html
    {{DetailTopbar}}
      {{forward name="breadcrumb"}}
      {{forward name="actions"}}
    {{/DetailTopbar}}
    <div class="header-body">{{slot name="header"}}{{/slot}}</div>
```

```kilnx
{{DetailHeader}}
  {{slot name="breadcrumb"}}<a href="/back">Back</a>{{/slot}}
  {{slot name="actions"}}<button>Save</button>{{/slot}}
  {{slot name="header"}}<h1>Title</h1>{{/slot}}
{{/DetailHeader}}
```

`{{forward}}` only supports named slots: default-to-default forwarding is already covered by a bare `{{slot}}` marker inside the child call. When the outer caller did not provide a value for the forwarded slot, `{{forward}}` emits nothing so the inner fragment falls back to its own slot fallback.

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
| **Source last touched** | `72e9177` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

