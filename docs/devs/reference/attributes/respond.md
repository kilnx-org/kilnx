# `respond`

> Return a partial HTML response, a status code, or a file from disk.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
respond [fragment <selector> | status <code> | file <path> [as <name>] [inline|attachment]]
```

## Description

Three forms:

- Without arguments: renders the body's HTML normally.
- `fragment <selector>`: returns an htmx swap.
- `status <code>`: returns an explicit HTTP status.
- `file <path> [as <name>] [inline | attachment]`: streams a disk file. `<path>` accepts `:param` substitution. Default disposition is `attachment`; `as <name>` overrides the `filename=` portion of `Content-Disposition`. Requires [`serve-roots`](serve-roots.md) in the `config` block listing allowed directories.

## Used in

- [`page`](../keywords/page.md)
- [`action`](../keywords/action.md)
- [`fragment`](../keywords/fragment.md)
- [`api`](../keywords/api.md)
- [`schedule`](../keywords/schedule.md)
- [`job`](../keywords/job.md)
- [`webhook`](../keywords/webhook.md)

## Examples

### Replace a row via htmx

```kilnx
action /users/:id/toggle method POST
  query: UPDATE user SET active = NOT active WHERE id = :id
  respond fragment ".user-row" query: SELECT * FROM user WHERE id = :id
```

### Serve a generated PDF as download

```kilnx
action get /review/:id/pdf
  query: SELECT pdf_path FROM review_job WHERE id = :id
  respond file :pdf_path as relatorio.pdf
```

## Provenance

| | |
|---|---|
| **Spec last touched** | `9276716` (2026-05-21) |
| **Source last touched** | `9276716` (2026-05-21) |
| **Source files** | `internal/parser/parser.go` |

