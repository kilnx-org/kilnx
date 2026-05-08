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

| | |
|---|---|
| **Spec last touched** | `e1d0f3f` (2026-05-08) |
| **Source last touched** | `b2cecfb` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

