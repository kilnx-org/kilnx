# `requires`

> Require authentication or a specific role/permission.

| | |
|---|---|
| **Kind** | Attribute |
| **Since** | `0.1.0` |
| **Repeatable** | yes |

## Syntax

```
requires <clause>
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `clause` | `identifier` | yes |

## Description

Gates the parent endpoint behind authentication. Accepts `auth` (any logged-in user), a role name, or a permission expression.

## Used in

- [`page`](../keywords/page.md)
- [`action`](../keywords/action.md)
- [`api`](../keywords/api.md)

## Examples

### Require any logged-in user

```kilnx
page /me requires auth
```

### Require a specific role

```kilnx
page /admin requires admin
```

## See also

- [`method`](method.md)
- [`permissions`](../keywords/permissions.md)
- [`auth`](../keywords/auth.md)
- [`limit`](../keywords/limit.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `e1d0f3f` (2026-05-08) |
| **Source last touched** | `b2cecfb` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

