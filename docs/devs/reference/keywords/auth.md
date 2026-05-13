# `auth`

> Configure email/password authentication.

| | |
|---|---|
| **Kind** | Keyword |
| **Since** | `0.1.0` |

## Syntax

```
auth
```

## Description

The `auth` block enables built-in authentication: login, logout, registration, password reset, and session management. Kilnx auto-wires CSRF protection, password hashing (bcrypt), and HTTP-only session cookies. To protect routes, use `requires auth` (any logged-in user) or `requires <role>` on pages, actions, fragments, or APIs.

## Children

- [`table`](../attributes/table.md)
- [`identity`](../attributes/identity.md)
- [`password`](../attributes/password.md)
- [`login`](../attributes/login.md)
- [`logout`](../attributes/logout.md)
- [`register`](../attributes/register.md)
- [`forgot`](../attributes/forgot.md)
- [`reset`](../attributes/reset.md)
- [`after_login`](../attributes/after_login.md)
- [`superuser`](../attributes/superuser.md)

## Examples

### Standard email/password setup

```kilnx
auth
  table: user
  identity: email
  password: password
  login: /login
  logout: /logout
  register: /register
  after login: /dashboard
  superuser: env SUPERUSER_EMAIL
```

## See also

- [`permissions`](permissions.md)
- [`requires`](../attributes/requires.md)
- [`model`](model.md)

## Provenance

> ⚠ **Implementation touched after spec.** Source code changed on `2026-05-13`, but this entity's spec was last edited on `2026-05-08`. The description may be out of date.

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `aef0ef5` (2026-05-13) |
| **Source files** | `internal/parser/parser.go` |

