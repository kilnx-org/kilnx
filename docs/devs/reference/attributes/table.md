# `table`

> Name of the model storing user accounts.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |
| **Default** | `user` |

## Syntax

```
table: <name>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `name` | `identifier` | yes |

## Used in

- [`auth`](../keywords/auth.md)

## Examples

### Use a custom user model

```kilnx
auth
  table: account
  identity: email
```

## Provenance

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `aef0ef5` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

