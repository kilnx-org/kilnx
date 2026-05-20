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

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-19`, but this entity's spec was last edited on `2026-05-13`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `72e9177` (2026-05-13) |
| **Source last touched** | `d461e57` (2026-05-19) |
| **Source files** | `internal/parser/parser.go` |

