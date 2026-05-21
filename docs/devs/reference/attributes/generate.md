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
| **Spec last touched** | `18cf488` (2026-05-20) |
| **Source last touched** | `af278bb` (2026-05-20) |
| **Source files** | `internal/parser/parser.go` |

