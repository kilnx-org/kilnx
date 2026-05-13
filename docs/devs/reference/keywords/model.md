# `model`

> Declare a database table with typed fields and constraints.

| | |
|---|---|
| **Kind** | Keyword |
| **Since** | `0.1.0` |
| **Repeatable** | yes |

## Syntax

```
model <name>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `name` | `identifier` | yes |

## Description

A `model` declares a persistent entity backed by a SQL table. The runtime auto-migrates the schema: adding fields, indexes, and unique constraints triggers `ALTER TABLE` on next start. Field declarations use `name: <type> [constraint...]`. Built-in types include `text`, `email`, `int`, `float`, `bool`, `timestamp`, `date`, `uuid`, `password`, `image`, `file`, `json`, `reference <model>`, `option [a, b, c]`, `tags`, and others.

## Children

- [`required`](../attributes/required.md)
- [`unique`](../attributes/unique.md)
- [`auto`](../attributes/auto.md)
- [`auto_update`](../attributes/auto_update.md)
- [`default`](../attributes/default.md)
- [`min`](../attributes/min.md)
- [`max`](../attributes/max.md)
- [`index`](../attributes/index.md)
- [`tenant`](../attributes/tenant.md)
- [`custom`](../attributes/custom.md)
- [`dynamic_fields`](../attributes/dynamic_fields.md)

## Examples

### User model with constraints

```kilnx
model user
  email: email required unique
  name: text required
  role: option [admin, editor, viewer] default "viewer"
  age: int min 18 max 120
  created_at: timestamp auto
```

### Multi-tenant model

```kilnx
model invoice
  tenant: account
  amount: decimal required
  status: option [draft, sent, paid] default "draft"
  index (tenant, status)
```

## See also

- [`query`](query.md)
- [`auth`](auth.md)

## Provenance

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `72e9177` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

