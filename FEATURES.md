# Features

Every feature below works today. See [GRAMMAR.md](GRAMMAR.md) for the full syntax reference.

## Models

Define data once. Kilnx generates the database table, server validation, and client-side validation attributes.

```kilnx
model user
  name: text required min 2 max 100
  email: email unique
  role: option [admin, editor, viewer] default viewer
  active: bool default true
  created: timestamp auto

model post
  title: text required min 5
  body: richtext required
  author: user required
  status: option [draft, published] default draft
  created: timestamp auto
```

Field types: `text`, `email`, `int`, `float`, `bool`, `timestamp`, `richtext`, `option`, `password`, `image`, `phone`. Constraints: `required`, `unique`, `default`, `auto`, `min`, `max`.

## Auth

Six lines. Registration, login, logout, bcrypt hashing, session cookies, and `current_user` available everywhere.

```kilnx
auth
  table: user
  identity: email
  password: password
  login: /login
  after login: /dashboard
```

Protect any route with `requires auth` or `requires admin`.

## Permissions

```kilnx
permissions
  admin: all
  editor: read post, write post where author = current_user
  viewer: read post where status = published
```

## Pages

GET routes returning full HTML. Queries run inline. Templates loop and branch.

```kilnx
page /dashboard requires auth
  query stats: SELECT count(*) as total_users FROM user
  query posts: SELECT p.title, u.name as author
               FROM post p
               LEFT JOIN user u ON u.id = p.author_id
               WHERE p.status = 'published'
               ORDER BY p.created DESC
               paginate 10
  html
    <p>Total users: {stats.total_users}</p>
    {{each posts}}
    <article><h3>{title}</h3><span>by {author}</span></article>
    {{end}}
```

## Actions

POST/PUT/DELETE mutations with validation, branching, and redirects.

```kilnx
action /posts/create method POST requires auth
  validate
    title: required min 5
    body: required
  query: INSERT INTO post (title, body, author)
         VALUES (:title, :body, :current_user.id)
  on success
    redirect /posts
  on error
    alert "Could not create post"
```

## Fragments

Partial HTML for htmx to swap into the DOM. No JavaScript.

```kilnx
fragment /users/:id/card
  query user: SELECT name, email FROM user WHERE id = :id
  html
    <div class="card">
      <strong>{user.name}</strong>
      <span>{user.email}</span>
    </div>
```

## JSON API

Same grammar, JSON output. One keyword difference.

```kilnx
api /api/v1/users requires auth
  query users: SELECT id, name, email FROM user
               ORDER BY id DESC paginate 50
```

Returns `{"data": [...], "pagination": {"page": 1, "total": 42, ...}}`.

## Server-Sent Events

Realtime polling with the embedded htmx SSE extension.

```kilnx
stream /notifications requires auth
  query: SELECT message FROM notification
         WHERE user_id = :current_user.id AND seen = false
  every 5s
```

## WebSockets

Bidirectional communication with rooms and broadcast.

```kilnx
socket /chat/:room requires auth
  on connect
    query: SELECT message, author FROM chat_message
           WHERE room = :room ORDER BY created DESC LIMIT 50
  on message
    query: INSERT INTO chat_message (body, author, room)
           VALUES (:body, :current_user.id, :room)
    broadcast to :room
```

## Webhooks

External event receivers with HMAC signature verification. Supports GitHub and Stripe formats.

```kilnx
webhook /stripe secret env STRIPE_SECRET
  on event payment_intent.succeeded
    query: UPDATE order SET status = 'paid'
           WHERE stripe_id = :event_id
  on event *
    query: INSERT INTO webhook_log (event, payload)
           VALUES (:event.type, :event.body)
```

## Background Jobs

Async work dispatched from actions and executed in the same binary.

```kilnx
job send-welcome
  query data: SELECT name, email FROM user WHERE id = :user_id
  send email to :email
    subject: "Welcome {data.name}"

action /users/create method POST
  query: INSERT INTO user (name, email) VALUES (:name, :email)
  enqueue send-welcome with user_id: :id
  redirect /users
```

## Schedules

Timed tasks running inside the same binary. No Redis, no cron.

```kilnx
schedule cleanup every 24h
  query: DELETE FROM session WHERE expires_at < datetime('now')

schedule report every monday at 9:00
  query stats: SELECT count(*) as new_users FROM user
               WHERE created > datetime('now', '-7 days')
  send email to "admin@example.com"
    subject: "Weekly report: {stats.new_users} new users"
```

## Rate Limiting

Declarative. Per user or per IP. Wildcard paths.

```kilnx
limit /api/*
  requests: 100 per minute per user

limit /login
  requests: 5 per minute per ip
  on exceeded: status 429 message "Too many attempts"
```

## Internationalization

Translations with `{t.key}` interpolation. Language detected from `Accept-Language` header or `?lang=` param.

```kilnx
translations
  en
    welcome: "Welcome back"
  pt
    welcome: "Bem vindo de volta"

config
  default language: en
  detect language: header accept-language

page /dashboard requires auth
  "{t.welcome}, {current_user.name}"
```

## Email

SMTP with templates and attachments.

```kilnx
action /users/invite method POST requires admin
  validate
    email: required, is email
  query: INSERT INTO user (email, role) VALUES (:email, 'viewer')
  send email to :email
    template: invite
    subject: "You've been invited"
```

## PDF Generation

Generate PDFs from templates with query data.

```kilnx
job generate-report
  query data: SELECT * FROM order WHERE created > :start_date
  generate pdf from template report with data
  send email to :requested_by
    attach: generated pdf
    subject: "Your report is ready"
```

## Declarative Tests

Test your app in the same language. No Selenium, no Cypress.

```kilnx
test "user can register"
  visit /register
  fill name with "Alice"
  fill identity with "alice@test.com"
  fill password with "secret123"
  submit
  expect page /login contains "Log in"

test "homepage loads"
  visit /
  expect status 200
  expect page / contains "Blog"
```

```bash
$ kilnx test app.kilnx
Running 2 test(s):
  PASS  user can register
  PASS  homepage loads
All 2 test(s) passed.
```

## Template Filters

Built-in filters for formatting output in `html` blocks.

```
{name|upcase}                    → ALICE
{name|truncate:20}               → Alice Wonderla...
{created|date:"Jan 02, 2006"}   → Mar 27, 2026
{created|timeago}                → 3 hours ago
{price|currency:"$"}             → $1,234.56
{count|pluralize:"item","items"} → 3 items
{bio|raw}                        → unescaped HTML
{role|default:"viewer"}          → viewer (if empty)
```

## Layouts

Page wrapper templates.

```kilnx
layout main
  html
    <html>
    <head>
      <title>{page.title}</title>
      {kilnx.js}
    </head>
    <body>
      {nav}
      {page.content}
    </body>
    </html>

page /users layout main title "Users"
  query users: SELECT name FROM user
  html
    {{each users}}<div>{name}</div>{{end}}
```

## Query Optimization

The optimizer rewrites queries based on template usage:

- **SELECT \* rewriting**: replaces `SELECT *` with only the columns referenced in `{field}` interpolations
- **JOIN pruning**: removes JOINs when no columns from the joined table are used
- **Query deduplication**: identical named queries are executed once

## Static Analysis

`kilnx check` runs compile-time validation:

- SQL column and table references match declared models
- Type compatibility in WHERE clauses (text vs numeric vs bool)
- Unprotected mutations without auth
- Password fields exposed in public queries
- Missing CSRF protection
- Webhooks without signature verification

## LSP Server

`kilnx lsp` provides IDE integration with completions, diagnostics, and hover documentation.
