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

## Provenance

| | |
|---|---|
| **Spec last touched** | `e1d0f3f` (2026-05-08) |
| **Source last touched** | `b2cecfb` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

