# `query`

> Run a SQL query and bind its result. Used both top-level (named query) and inside bodies.

| | |
|---|---|
| **Kind** | Keyword |
| **Since** | `0.1.0` |
| **Repeatable** | yes |

## Syntax

```
query [<name>]: <SQL>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `name` | `identifier` | no |
| `SQL` | `string` | yes |

## Description

Top-level `query <name>: <SQL>` declares a reusable named query that other pages, actions, or fragments may reference. Inside a page/action/fragment/api/job/schedule body, `query [<name>]: <SQL>` runs the SQL and binds rows to a template variable. Add `paginate <n>` to paginate. Parameters are referenced via `:name` (URL/form values) or `:current_user_id` (session).

## Examples

### Named top-level query

```kilnx
query active_users:
  SELECT id, name, email FROM user WHERE status = 'active'
```

### Inline query inside a page body

```kilnx
page /users
  query users: SELECT id, name FROM user ORDER BY id paginate 20
  html
    {#each users}
      <li>{name}</li>
    {/each}
```

## See also

- [`page`](page.md)
- [`action`](action.md)
- [`fragment`](fragment.md)
- [`model`](model.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `e1d0f3f` (2026-05-08) |
| **Source last touched** | `b2cecfb` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

