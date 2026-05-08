# Auth and permissions

Kilnx ships with email/password authentication. You declare an [`auth`](../reference/keywords/auth.md) block and the runtime wires up login, logout, registration, password reset, sessions, CSRF protection, and bcrypt hashing. Authorization, when needed, is layered on top with [`permissions`](../reference/keywords/permissions.md).

## The auth block

`auth` points at a model and tells the runtime which fields hold the identity and the password hash.

```kilnx
model user
  email: email required unique
  password: password required
  name: text
  role: option [admin, editor, viewer] default "viewer"
  created_at: timestamp auto

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

The runtime serves login, logout, and registration forms at the configured paths. Submitted passwords are hashed with bcrypt before they hit the database. Sessions are HMAC-signed cookies; the signing secret comes from the `SECRET_KEY` env var or from a `secret` attribute inside [`config`](../reference/keywords/config.md).

The `superuser` line designates an identity (here, an email read from `SUPERUSER_EMAIL`) that bypasses role checks. Useful for the bootstrap admin.

## Protecting routes

Any [`page`](../reference/keywords/page.md), [`action`](../reference/keywords/action.md), [`fragment`](../reference/keywords/fragment.md), or [`api`](../reference/keywords/api.md) can require auth with the [`requires`](../reference/attributes/requires.md) attribute:

```kilnx
page /dashboard requires auth
  html
    <h1>Welcome, {current_user.name}</h1>
```

`requires auth` accepts any logged-in user. If the request is not authenticated, the runtime redirects to the login path declared in the `auth` block.

`current_user` is bound automatically inside protected routes. Use it in queries (`WHERE owner = :current_user.id`) and templates (`{current_user.name}`).

## CSRF and form posts

CSRF protection is on by default. Every action that accepts form input expects a CSRF token. The runtime injects the token into rendered forms via the standard template helpers, so for htmx and regular form posts it is automatic. You only need to think about CSRF if you build a custom form outside the template engine.

Sessions are HTTP-only and signed. There is no client-side session handling.

## Permissions: roles and rules

`requires auth` is binary. For role-based rules, declare a [`permissions`](../reference/keywords/permissions.md) block:

```kilnx
permissions
  admin: all
  editor: read post, write post where author = current_user
  viewer: read post where status = published
```

Each role gets a list of rules. The vocabulary is small:

- `all`: every action on every resource.
- `read <resource>`: SELECTs against the named model.
- `write <resource>`: INSERT, UPDATE, DELETE.
- `where <expression>`: an optional filter, evaluated against the row and `current_user`.

Routes opt in by name:

```kilnx
page /admin requires admin
  html
    <h1>Admin</h1>

action /posts/:id/publish requires editor
  query: UPDATE post SET status = 'published' WHERE id = :id
  redirect /posts
```

If the current user's role does not satisfy the rule, the runtime returns 403.

The `where` clause is enforced both at the route layer and inside model-bound queries. A viewer hitting `SELECT * FROM post` only sees rows where `status = 'published'`, even if the SQL did not include that filter explicitly. The analyzer rewrites the query to add the rule's predicate.

## A complete slice

```kilnx
model user
  email: email required unique
  password: password required
  role: option [admin, editor, viewer] default "viewer"

model post
  title: text required
  body: text
  author: reference user required
  status: option [draft, published] default "draft"

auth
  table: user
  identity: email
  password: password
  login: /login
  after login: /

permissions
  admin: all
  editor: read post, write post where author = current_user
  viewer: read post where status = published

page / requires auth
  query posts: SELECT id, title, status FROM post ORDER BY id DESC
  html
    {{each posts}}
    <li>{title} ({status})</li>
    {{end}}
```

A viewer sees only published posts. An editor sees their own drafts plus all published. An admin sees everything.

## Read next

- [Pages, actions, fragments](pages-actions-fragments.md): the routes you are protecting.
- [Models and data](models-and-data.md): the `user` model under `auth`.
