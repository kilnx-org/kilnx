# `tenant`

> Scope all rows of this model to a tenant.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |

## Syntax

```
tenant: <model>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `model` | `identifier` | yes |

## Description

Auto-synthesizes a required reference field to the tenant model and transparently filters queries by the current tenant.

## Used in

- [`model`](../keywords/model.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `e1d0f3f` (2026-05-08) |
| **Source last touched** | `b2cecfb` (2026-05-08) |
| **Source files** | `internal/parser/parser.go`, `internal/runtime/auth.go` |

