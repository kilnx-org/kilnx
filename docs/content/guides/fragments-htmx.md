# Fragments &amp; htmx

> This page is a stub. Full content coming soon. For now see [FEATURES.md &sect; Fragments](https://github.com/kilnx-org/kilnx/blob/main/FEATURES.md) and the [grammar reference](../reference/grammar.html).

Fragments are partial HTML responses designed for htmx swaps. Not a full page — just a piece the browser inserts into the existing DOM.

```kilnx
fragment /users/:id/card
  query user: SELECT name, email FROM user WHERE id = :id
  html
    <div class="card">
      <strong>{user.name}</strong>
      <span>{user.email}</span>
    </div>
```

## Swap targets

Actions can respond with a fragment to drive htmx swaps:

```kilnx
action /posts/:id/delete method POST requires auth
  query: DELETE FROM post WHERE id = :id
  respond fragment delete
```

`respond fragment delete` tells htmx to remove the target element. Also supports:

- `respond fragment <selector>` — swap the matched element
- `respond fragment <selector> query: <SQL>` — swap with fresh data

## Layouts do not apply

Fragments bypass `layout` blocks. Full HTML shell is pages-only.
