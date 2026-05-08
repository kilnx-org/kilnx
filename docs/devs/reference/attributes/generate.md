# `generate`

> Generate a PDF from a template and a data query.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
generate pdf from template <name> data <query-name>
```

## Description

Renders an HTML template to PDF using the internal pdf engine. The `data` argument names a previously-bound query result.

## Used in

- [`page`](../keywords/page.md)
- [`action`](../keywords/action.md)
- [`fragment`](../keywords/fragment.md)
- [`api`](../keywords/api.md)
- [`schedule`](../keywords/schedule.md)
- [`job`](../keywords/job.md)
- [`webhook`](../keywords/webhook.md)

## Examples

### Render an invoice

```kilnx
action /invoices/:id/pdf
  query invoice_data: SELECT * FROM invoice WHERE id = :id
  generate pdf from template invoice data invoice_data
```

## Provenance

| | |
|---|---|
| **Spec last touched** | `e1d0f3f` (2026-05-08) |
| **Source last touched** | `b2cecfb` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

