# Kilnx Grammar Reference

Kilnx has 27 keywords. The entire language fits on a single page.

For comparison: Python has 35 keywords and does none of these things without importing libraries. JavaScript has 64. Java has 67. Kilnx has 27 and delivers a complete web app from database to browser.

## Hello World

```kilnx
page /
  "Hello World"
```

One useful line. That's it.

---

## Keywords

### config

Global configuration. Database, port, secrets, upload limits.

```kilnx
config
  database: env DATABASE_URL default "sqlite://app.db"
  port: env PORT default 8080
  secret: env SECRET_KEY required
  uploads: ./uploads max 50mb
```

### model

Defines data types and structure. The single source of truth. From a model, the language generates: CREATE TABLE, server validation, HTML forms, client validation, and listing fragments.

```kilnx
model user
  name: text required min 2 max 100
  email: email unique
  role: option [admin, editor, viewer] default viewer
  active: bool default true
  created: timestamp auto
```

Relationships between models:

```kilnx
model post
  title: text required min 5
  body: richtext required
  status: option [draft, published, archived] default draft
  author: user required
  created: timestamp auto
  published_at: timestamp optional

model comment
  body: text required
  post: post required
  author: user required
  created: timestamp auto
```

#### custom fields from manifest

A model can declare a versioned manifest of runtime-extensible fields. The
manifest file (`*_fields.kilnx`) is edited by the developer and parsed at
startup. Fields are stored as JSON in a `custom` column (`TEXT` on SQLite,
`JSONB` on PostgreSQL).

```kilnx
model deal
  name: text required
  custom fields from "deal_fields.kilnx"
```

Manifest syntax:
```
field revenue
  kind: number
  label: "Revenue"
  required: false
  mode: column  // promotes to a real DB column instead of JSON storage

field status
  kind: option
  option [open, won, lost]
```

Available kinds: `text`, `number`, `date`, `option`, `email`, `phone`, `bool`,
`richtext`, `reference`, `image`.

Access in templates: `{q.custom.revenue}`. Iterate: `{{each q.custom}}` (yields
`name`, `value`, `label`, `kind` per field).

Per-tenant manifests with runtime placeholders:
```kilnx
model quote
  custom fields from "{user.tenant_id}_fields.kilnx" or "default_fields.kilnx"
```

`kilnx check` validates all hardcoded `{q.custom.X}` references against the
manifest. Dynamic-path manifests are silenced.

#### dynamic fields (runtime-mutable)

A model can opt into DB-backed runtime field definitions. End-users (not
developers) can then create, edit, and delete custom fields without touching
`.kilnx` files and without an app restart.

```kilnx
model deal
  name: text required
  custom fields from "deal_base.kilnx"  // optional static baseline
  dynamic fields                         // enables DB-backed field management
```

When `dynamic fields` is declared, kilnx auto-creates a `_deal_field_defs`
table at migration time with columns: `id`, `name`, `kind`, `label`,
`required`, `options`, `reference_model`, `tenant_id`, `sort_order`.

The runtime merges the static manifest (if any) with rows from
`_deal_field_defs` at request time. Static fields always win on name collision.

The developer writes the field management UI using normal pages and actions:

```kilnx
page /admin/fields requires auth
  query fields: SELECT * FROM _deal_field_defs ORDER BY sort_order
  html
    {{each fields}}
      <div>{fields.label} ({fields.kind})</div>
    {{end}}

action /admin/fields/create method POST requires auth
  query: INSERT INTO _deal_field_defs (name, kind, label)
         VALUES (:name, :kind, :label)
  redirect /admin/fields
```

**Static analysis**: `kilnx check` emits a warning (not an error) for hardcoded
`{q.custom.X}` references on dynamic-fields models. Use `kilnx check --db
<url>` to connect to a live database and validate against actual field names.

**DB fields are JSON-only**: rows from `_field_defs` always write to the
`custom` column. Column-mode (`mode: column`) is only available for static
manifest fields.

#### tenant scoping

A model can declare that its rows belong to a tenant (another model) with
the `tenant:` directive. The directive must appear before any field.

```kilnx
model org
  name: text required unique

model user
  tenant: org
  email: email unique
  password: password required

model quote
  tenant: org
  number: text required unique
  total: float default 0
```

The compiler auto-synthesizes a required reference field for the tenant
(so `tenant: org` adds an `org_id` foreign key column) and the runtime
rewrites `SELECT` queries against a tenant-scoped table to include
`WHERE <table>.<tenant>_id = :current_user.<tenant>_id`. When the query
already has a `WHERE`, the tenant predicate is joined with `AND`.

The rewriter **fails closed**: if the SQL shape is too complex for the
built-in rewriter to verify (CTEs, JOINs, subqueries, UNION, comments,
schema-qualified tables, multi-statement queries), the query is refused
at runtime rather than silently passing through unscoped. Refactor the
query into a simpler single-table SELECT or bind the tenant predicate
yourself with `WHERE ... AND <col> = :current_user.<tenant>_id` and the
rewriter will see it.

Mutations (`INSERT`, `UPDATE`, `DELETE`) on a tenant-scoped table must
bind the tenant column textually; otherwise the runtime rejects them.
Example that passes:

```kilnx
action /quotes/create method POST requires auth
  query: INSERT INTO quote (org_id, number)
         VALUES (:current_user.org_id, :n)
```

The parser synthesizes the `<tenant>_id` column automatically, and
`kilnx check` flags references to undefined tenant models and models
that set themselves as their own tenant.

This is defense in depth, not a substitute for application-level
authorization. The rewriter closes the "forgot the tenant predicate"
failure mode; other access-control concerns remain the developer's
responsibility.

#### composite unique

For uniqueness spanning more than one field, declare a model-level
`unique (...)` directive:

```kilnx
model membership
  user: user required
  project: project required
  role: option [owner, admin, member] default member
  unique (user, project)
```

The directive takes two or more field names (single-field uniqueness
uses the field-level `unique` constraint). Reference fields resolve to
their `<name>_id` column. Migration emits `CREATE UNIQUE INDEX IF NOT
EXISTS "uq_<table>_<col>_<col>" ON "<table>" (...)`, which is
idempotent on both SQLite and PostgreSQL. Multiple `unique (...)` lines
are allowed for independent groups. The analyzer rejects unknown field
names, fields repeated within a group, and duplicate groups.

#### non-unique indexes

For query acceleration without a uniqueness requirement, declare an
`index (...)` directive:

```kilnx
model order
  customer: customer required
  created: timestamp auto
  status: option [pending, paid, shipped]
  index (customer, created)
  index (status)
```

Single-column and multi-column indexes are both supported. Migration
emits `CREATE INDEX IF NOT EXISTS "ix_<table>_<cols>" ON "<table>"
(...)`. The `ix_` prefix distinguishes non-unique indexes from the
`uq_` prefix used by composite UNIQUE constraints. The same analyzer
rules apply as for `unique (...)`.

### permissions

Access rules by role.

```kilnx
permissions
  admin: all
  editor: read post, write post where author = current_user
  viewer: read post where status = published
```

### auth

Authentication configuration. Declarative, not code.

```kilnx
auth
  table: users
  identity: email
  password: password_hash
  login: /login
  after login: /dashboard
  superuser: env SUPERUSER_EMAIL
```

`superuser` designates a platform operator identity (resolved via `env VAR` or
a literal string). A superuser bypasses all role checks — they can access any
route regardless of `requires` clauses.

#### requires clauses

`requires` accepts a comma-separated list of predicates. All must pass (AND semantics).

```kilnx
page /admin requires auth                       // any logged-in user
page /admin requires admin                      // named role
page /admin requires superuser                  // platform operator only
page /app requires auth, :current_user.plan in ['cad','full']   // expression
page /app requires admin, :current_user.active == 'true'        // combined
```

Expression predicates are prefixed with `:` and support:
- `current_user.fieldName in ['a','b','c']` — membership check
- `current_user.fieldName == 'value'` / `!=` / `>` / `<` / `>=` / `<=`
- `and` / `or` conjunctions

Fields are resolved from the session user row (`current_user.X` or bare `X`).

### layout

Page wrapper templates. Four placeholders are available:

- `{page.title}` - the page title (HTML-escaped)
- `{page.content}` - the rendered page body
- `{nav}` - auto-generated navigation bar
- `{kilnx.js}` - **required** htmx and SSE scripts. Without this, htmx functionality breaks.

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
```

### page

GET route that returns full HTML. The basic unit of the language.

```kilnx
page /users layout main title "Users"
  query users: select name, email from user
  html
    {{each users}}
    <div class="user">
      <strong>{name}</strong>
      <span>{email}</span>
    </div>
    {{end}}
```

With auth:

```kilnx
page /dashboard requires auth
  query stats: select count(*) as total from orders
  "Welcome back. You have {stats.total} orders."
```

### action

POST/PUT/DELETE route that mutates data.

```kilnx
action /users/:id/archive method POST requires auth
  query: update users set archived = true where id = :id
  respond fragment user-card query:
    select name, email from users where id = :id
```

### fragment

Reusable piece of HTML for htmx to swap in the DOM.

```kilnx
fragment /users/:id/card
  query user: select name, email from users where id = :id
  html
    <div class="card">
      <h3>{user.name}</h3>
      <p>{user.email}</p>
    </div>
```

### stream

Server-Sent Events for realtime updates.

```kilnx
stream /notifications requires auth
  query: select message, created_at from notifications
         where user_id = :current_user.id
         and seen = false
  every 5s
```

### socket

Bidirectional WebSocket.

```kilnx
socket /chat/:room requires auth
  on connect
    query: select message, author.name, created
           from chat_message
           where room = :room
           order by created desc
           limit 50
    send history

  on message
    validate
      body: required max 500
    query: insert into chat_message (body, author, room)
           values (:body, :current_user.id, :room)
    broadcast to :room fragment chat-bubble
```

### api

JSON endpoint. Same grammar as page, but returns JSON instead of HTML.

```kilnx
api /api/v1/posts requires auth
  query posts: select id, title, status, author.name, created
               from post
               where status = published
               order by created desc
               paginate 50

api /api/v1/posts method POST requires editor
  validate
    title: required min 5
    body: required
  query: insert into post (title, body, author, status)
         values (:title, :body, :current_user.id, draft)
  respond status 201
```

### webhook

Receives external events.

```kilnx
webhook /stripe/payment secret env STRIPE_SECRET
  on event payment_intent.succeeded
    query: update order set status = paid
           where stripe_id = :event.id
    send email to query: select email from user
                         where id = :event.customer_id
      template: payment-received
      subject: "Payment confirmed"
```

Use `on event *` as a catch-all to match any event type:

```kilnx
webhook /github secret env GITHUB_SECRET
  on event *
    query: insert into webhook_log (event, payload, received)
           values (:event.type, :event.body, now())
```

### schedule

Timed tasks running inside the same binary.

```kilnx
schedule cleanup every 24h
  query: delete from session where expires_at < now()

schedule report every monday at 9:00
  query stats: select count(*) as new_users from user
               where created > now() - interval 7 days
  send email to query: select email from user where role = admin
    template: weekly-report
    subject: "Weekly report: {stats.new_users} new users"
```

### job

Asynchronous background work.

```kilnx
job generate-report
  query data: select * from order
              where created > :start_date
              and created < :end_date
  generate pdf from template report data
  send email to :requested_by
    template: report-ready
    attach: generated pdf
    subject: "Your report is ready"
```

### query

SQL inline or named. A top-level `query <name>: SQL` defines a reusable named query that can be referenced by other blocks. Inside a page, action, fragment, or api body, `query <name>: SQL` binds the result for template interpolation.

```kilnx
query active-users: select u.name, u.email, count(o.id) as orders
                    from users u
                    left join orders o on o.user_id = u.id
                    where u.active = true
                    group by u.id

page /users
  query users: active-users
  html
    {{each users}}
    <div class="user">
      <strong>{name}</strong>
      <span>{orders} orders</span>
    </div>
    {{end}}
```

### validate

Declarative validation rules.

```kilnx
action /users/new method POST
  validate
    name: required
    email: required, is email
  query: insert into users (name, email) values (:name, :email)
  redirect /users
```

### paginate

Automatic pagination. The language generates pagination controls with htmx.

```kilnx
page /posts
  query posts: select title, author.name from post
               where status = published
               order by published_at desc
               paginate 20
  html
    {{each posts}}
    <article>
      <h2>{title}</h2>
      <span>{author.name}</span>
    </article>
    {{end}}
```

### send email

Declarative email sending.

```kilnx
action /users/invite method POST requires admin
  validate
    email: required, is email
  query: insert into user (email, role, active)
         values (:email, viewer, false)
  send email to :email
    template: invite
    subject: "You've been invited"
```

### redirect

Redirects to another route.

```kilnx
action /users/create method POST
  validate user
  query: insert into user (name, email) values (:name, :email)
  redirect /users
```

### on

Result handling for success, error, not found, forbidden.

```kilnx
action /users/:id/delete method POST requires auth
  query: delete from users where id = :id
  on success: redirect /users
  on error: alert "Could not delete user"
  on forbidden: redirect /login
```

### limit

Rate limiting. Declarative.

```kilnx
limit /api/*
  requests: 100 per minute per user
  on exceeded: status 429 message "Too many requests"

limit /login
  requests: 5 per minute per ip
  on exceeded: status 429 message "Too many attempts"
    delay 30s
```

### log

Observability built in.

```kilnx
log
  level: env LOG_LEVEL default info
  slow-query: 100ms
  requests: all
  errors: all stacktrace
```

### test

Declarative tests in the same language.

```kilnx
test "user can create post"
  as editor
  visit /posts/new
  fill title "Test Post"
  fill body "Content here"
  submit
  expect page /posts contains "Test Post"
  expect query: select count(*) from post
                where title = 'Test Post'
         returns 1
```

### translations

Internationalization.

```kilnx
translations
  en
    welcome: "Welcome back"
    users: "Users"
  pt
    welcome: "Bem vindo de volta"
    users: "Usuários"

config
  default language: en
  detect language: header accept-language

page /dashboard requires auth
  "{t.welcome}, {current_user.name}"
```

### enqueue

Dispatches an async job.

```kilnx
action /reports/generate method POST requires admin
  validate
    start_date: required, is date
    end_date: required, is date
  enqueue generate-report
    start_date: :start_date
    end_date: :end_date
    requested_by: :current_user.email
  respond fragment ".reports"
    alert success "Report is being generated"
```

### broadcast

Sends data to all connected WebSocket clients in a room. The fragment receives the same params that the socket handler received.

```kilnx
socket /chat/:room requires auth
  on message
    query: insert into chat_message (body, author, room)
           values (:body, :current_user.id, :room)
    broadcast to :room fragment chat-bubble
```

---

## Complete App Example

```kilnx
config
  database: env DATABASE_URL default "sqlite://app.db"
  port: 8080
  secret: env SECRET_KEY required

model user
  name: text required
  email: email unique
  password: password required
  role: option [admin, user] default user
  created: timestamp auto

model task
  title: text required
  done: bool default false
  owner: user required
  created: timestamp auto

auth
  table: user
  identity: email
  password: password
  login: /login
  after login: /tasks

layout main
  html
    <html>
    <head>
      <title>Tasks</title>
      {kilnx.js}
    </head>
    <body>
      {nav}
      {page.content}
    </body>
    </html>

page /tasks layout main requires auth
  query tasks: select id, title, done from task
               where owner = :current_user.id
               order by created desc
               paginate 20
  html
    <input type="search" name="q" placeholder="Search tasks..."
           hx-get="/tasks" hx-trigger="keyup changed delay:300ms"
           hx-target="#task-list">
    <table id="task-list">
      <tr><th>Title</th><th>Done</th><th></th></tr>
      {{each tasks}}
      <tr>
        <td>{title}</td>
        <td>{{if done}}Yes{{end}}</td>
        <td><button hx-post="/tasks/{id}/delete" hx-target="closest tr" hx-swap="outerHTML">Delete</button></td>
      </tr>
      {{end}}
    </table>

page /tasks/new layout main requires auth
  html
    <form method="POST" action="/tasks/create">
      <label>Title <input type="text" name="title" required></label>
      <button type="submit">Create</button>
    </form>

action /tasks/create method POST requires auth
  validate task
  query: insert into task (title, owner)
         values (:title, :current_user.id)
  redirect /tasks

action /tasks/:id/delete method POST requires auth
  query: delete from task where id = :id and owner = :current_user.id
  respond fragment ".task-list" query:
    select id, title, done from task
    where owner = :current_user.id
    order by created desc
```
