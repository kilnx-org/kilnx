# `password`

> Field storing the bcrypt password hash.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |
| **Required** | yes |

## Syntax

```
password: <field>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `field` | `identifier` | yes |

## Description

Field name on the auth table holding hashed passwords. Type must be `password` in the model.

## Used in

- [`auth`](../keywords/auth.md)

## Provenance

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `87ebbf6` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

