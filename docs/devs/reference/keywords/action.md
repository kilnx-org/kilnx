# `action`

> Declare a state-changing endpoint (POST/PUT/DELETE).

| | |
|---|---|
| **Kind** | Keyword |
| **Since** | `0.1.0` |
| **Repeatable** | yes |

## Syntax

```
action <path> [method <verb>] [requires <clause>]
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `path` | `path` | yes |

## Description

An `action` is a non-GET endpoint that executes side effects: writing to the database, sending email, enqueuing jobs, or redirecting. Unlike `page`, actions do not return a full HTML document by default; they typically end with a `redirect` or a `respond` (for htmx fragments).

## Children

- [`method`](../attributes/method.md)
- [`requires`](../attributes/requires.md)
- [`redirect`](../attributes/redirect.md)

## Examples

### Create a user, then redirect

```kilnx
action /users/create method POST requires auth
  validate user
  query: INSERT INTO user (email, name) VALUES (:email, :name)
  redirect /users
```

## See also

- [`page`](page.md)
- [`fragment`](fragment.md)
- [`api`](api.md)
- [`query`](query.md)
- [`redirect`](../attributes/redirect.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `5da8498` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

