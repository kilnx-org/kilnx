# Auth &amp; permissions

Kilnx ships with declarative auth and role-based permissions. No middleware, no passport, no session library.

## Auth block

```kilnx
auth
  table: user
  identity: email
  password: password
  login: /login
  after login: /dashboard
```

Six lines. You get:

- `GET /login` — login form (auto-generated HTML)
- `POST /login` — verifies credentials, creates session
- `GET /register` — registration form
- `POST /register` — creates user (password auto-hashed with bcrypt)
- `POST /logout` — clears session (CSRF-protected)
- Session cookie (HMAC-signed)
- `current_user.id`, `current_user.name`, `current_user.email`, `current_user.role` available in every query and template

## Protecting routes

```kilnx
page /dashboard requires auth
  ...

page /admin requires admin
  ...

action /posts/create method POST requires auth
  ...
```

`requires auth` redirects unauthenticated users to the configured login path. `requires <role>` additionally checks the user&apos;s `role` column. Both work on pages, actions, APIs, streams, and sockets.

## Password hashing

Passwords are hashed with bcrypt (cost 10) on INSERT. The hash is never returned to templates. The `password` field type is recognized specifically and stripped from `SELECT *` results by the query optimizer.

## Current user in queries

```kilnx
page /my/posts requires auth
  query posts: SELECT title FROM post WHERE author_id = :current_user.id
  html
    {{each posts}}<div>{title}</div>{{end}}
```

Available fields: `id`, `identity`, any columns on the auth table.

## Permissions (role-based)

```kilnx
permissions
  admin: all
  editor: read post, write post where author = current_user
  viewer: read post where status = published
```

Each line declares what a role can read and/or write. Supports ownership checks (`where author = current_user`) and column predicates (`where status = published`).

Kilnx rewrites queries at runtime based on the current user&apos;s role. A `viewer` hitting a page that queries `post` only sees published rows. No `WHERE` clause required in the SQL.

## Session configuration

```kilnx
config
  secret: env SECRET_KEY required
  session: 7d                      # default 24h
```

Sessions are signed with HMAC-SHA256 using the `secret`. Rotating the secret invalidates all sessions. Set a strong, random `SECRET_KEY` in production.

## Password reset

```kilnx
auth
  table: user
  identity: email
  password: password
  login: /login
  after login: /dashboard
  reset: /forgot
```

Adds `GET /forgot`, `POST /forgot` (sends reset email), `GET /reset/:token`, `POST /reset/:token`. Tokens expire after 1 hour.

## What is NOT built in

Kilnx does not ship OAuth, SSO, 2FA, or magic links. For those, write custom actions against the auth table. The declarative auth covers the 80% case (email + password + sessions) in six lines.

## Security defaults

CSRF protection, SQL injection binding, XSS escaping, and session HMAC are all built in. You have to work to disable them, not to enable them.

- CSRF tokens embedded in every form, verified on every POST
- SQL parameters bound via `:name` — never string-interpolated
- Template output HTML-escaped by default. `{field | raw}` is explicit opt-out for trusted content.
- Passwords never stored or transmitted plaintext
- Session cookies are `HttpOnly`, `Secure` (in production), `SameSite=Lax`
