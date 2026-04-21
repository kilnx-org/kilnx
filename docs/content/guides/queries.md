# Queries

> This page is a stub. Full content coming soon. For now see [pages &amp; actions](pages-actions.html#implicit-queries) and the [grammar reference](../reference/grammar.html).

Kilnx queries are inline SQL. The database is first-class — not accessed through an ORM, not hidden behind abstractions.

```kilnx
page /users
  query users: SELECT id, name, email FROM user ORDER BY created DESC
  html
    {{each users}}
      <div>{name} ({email})</div>
    {{end}}
```

## Parameter binding

Always use `:name` placeholders. Never string-interpolate SQL.

```kilnx
query user: SELECT * FROM user WHERE id = :id AND active = :active
```

## Named queries (reusable)

Top-level:

```kilnx
query active-users: SELECT name FROM user WHERE active = true

page /users
  query users: active-users
```

Reference by name inside any page/action/fragment/api body.

## Transactions

All queries inside a single `action` block run in an implicit transaction. Any failure rolls back all prior writes.

## Pagination

Add `paginate N` to a SELECT. Kilnx reads `?page=N` from the request and injects LIMIT/OFFSET automatically.

```kilnx
query posts: SELECT * FROM post ORDER BY created DESC paginate 20
```
