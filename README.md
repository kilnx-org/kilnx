<p align="center">
  <br>
  <strong style="font-size: 3rem;">kilnx</strong>
  <br>
  <em>The backend language for the htmx era.</em>
  <br><br>
</p>

<p align="center">
  <a href="https://github.com/kilnx-org/kilnx/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue" alt="MIT License"></a>
  <a href="https://htmx.org"><img src="https://img.shields.io/badge/htmx-2.x-blue" alt="htmx 2.x"></a>
  <img src="https://img.shields.io/badge/dependencies-2-brightgreen" alt="2 dependencies">
  <img src="https://img.shields.io/badge/keywords-22-orange" alt="22 keywords">
  <img src="https://img.shields.io/badge/version-1.0.0-purple" alt="v1.0.0">
</p>

<p align="center">
  Kilnx is a declarative backend language that compiles to a single binary.<br>
  22 keywords. Zero framework. SQL is the query language. HTML is the output.<br>
  <strong>htmx completed HTML. Kilnx completes htmx.</strong>
</p>

---

## Why

Most web apps are lists, forms, dashboards, CRUDs. The complexity comes from the tools, not from the problem. You shouldn't need a framework, an ORM, a template engine, and 200 files to show a list from a database.

Kilnx exists to prove this.

## Hello World

```kilnx
page /
  "Hello World"
```

```bash
$ kilnx run app.kilnx
kilnx serving on http://localhost:8080
```

That's it. One useful line. A running web server with htmx linked automatically.

## A Complete App in 30 Lines

```kilnx
model task
  title: text required
  done: bool default false
  created: timestamp auto

auth
  table: user
  identity: email
  password: password
  login: /login
  after login: /tasks

page /tasks requires auth
  query tasks: SELECT id, title, done FROM task
               WHERE owner = :current_user.id
               ORDER BY created DESC
               paginate 20
  html
    <input type="search" name="q" placeholder="Search..."
           hx-get="/tasks" hx-trigger="keyup changed delay:300ms"
           hx-target="#task-list">
    <table id="task-list">
      {{each tasks}}
      <tr>
        <td>{title}</td>
        <td>{{if done}}Yes{{end}}</td>
        <td><button hx-post="/tasks/{id}/delete" hx-target="closest tr" hx-swap="outerHTML">Delete</button></td>
      </tr>
      {{end}}
    </table>

page /tasks/new requires auth
  html
    <form method="POST" action="/tasks/new">
      <label>Title <input type="text" name="title" required></label>
      <button type="submit">Create</button>
    </form>

action /tasks/new requires auth
  validate task
  query: INSERT INTO task (title, owner) VALUES (:title, :current_user.id)
  redirect /tasks

action /tasks/:id/delete requires auth
  query: DELETE FROM task WHERE id = :id AND owner = :current_user.id
  respond fragment delete
```

This gives you: user registration, login with bcrypt, session management, CSRF protection, a searchable paginated table, form with validation, inline delete via htmx, and a SQLite database. All auto-generated from the declarations.

---

## Features

### Models as Single Source of Truth

Define your data once. Kilnx generates the database table, server validation, HTML form, and client-side validation attributes from a single declaration.

```kilnx
model user
  name: text required min 2 max 100
  email: email unique
  role: option [admin, editor, viewer] default viewer
  active: bool default true
  created: timestamp auto
```

Running `kilnx migrate` creates the SQLite table. `validate user` checks all constraints server-side. Build forms with `html` blocks using standard HTML inputs and htmx attributes.

### Declarative Auth

Six lines. Registration, login, logout, bcrypt hashing, session cookies, and `current_user` available everywhere.

```kilnx
auth
  table: user
  identity: email
  password: password
  login: /login
  after login: /dashboard
```

Protect any page or action with `requires auth` or `requires admin`.

### SQL is First-Class

No ORM. No query builder. SQL lives inline, right where you use it. The language preserves raw SQL including `COUNT(*)`, `LEFT JOIN`, subqueries, and `GROUP BY`.

```kilnx
page /dashboard requires auth
  query stats: SELECT count(*) as total_users FROM user
  query posts: SELECT p.title, u.name as author
               FROM post p
               LEFT JOIN user u ON u.id = p.author_id
               WHERE p.status = 'published'
               ORDER BY p.created DESC
               paginate 10
  "Total users: {stats.total_users}"
  html
    {{each posts}}
    <article><h3>{title}</h3><span>by {author}</span></article>
    {{end}}
```

### htmx-Native Fragments

Fragments return partial HTML that htmx swaps into the DOM. Inline delete, live search, partial updates, all without writing JavaScript.

```kilnx
action /users/:id/delete requires auth
  query: DELETE FROM user WHERE id = :id
  respond fragment delete

fragment /users/:id/card
  query user: SELECT name, email FROM user WHERE id = :id
  html
    <div class="card">
      <strong>{user.name}</strong>
      <span>{user.email}</span>
    </div>
```

### HTML Blocks

Use `html` blocks to render any UI with standard HTML, htmx attributes, `{{each}}` loops, and `{{if}}` conditionals. No proprietary component abstraction.

```kilnx
page /users
  query users: SELECT name, email, role FROM user ORDER BY id DESC paginate 10
  html
    {{each users}}
    <div class="user-row" hx-get="/users/{id}" hx-target="#detail">
      <strong>{name}</strong>
      <span>{email}</span>
      <span>{role}</span>
      <button hx-post="/users/{id}/delete" hx-confirm="Sure?">Delete</button>
    </div>
    {{end}}
```

You control the markup. The language handles routing, queries, auth, and htmx wiring.

### JSON API

Same grammar, JSON output. One keyword difference.

```kilnx
api /api/v1/users requires auth
  query users: SELECT id, name, email, role FROM user
               ORDER BY id DESC
               paginate 50
```

Returns `{"data": [...], "pagination": {"page": 1, "total": 42, ...}}`.

### Realtime with SSE

Server-Sent Events with polling. htmx SSE extension is embedded in the binary.

```kilnx
stream /notifications requires auth
  query: SELECT message FROM notification
         WHERE user_id = :current_user.id AND seen = false
  every 5s
```

### WebSockets

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

### Webhooks

Receive external events with HMAC signature verification. Supports GitHub and Stripe formats.

```kilnx
webhook /stripe/payment secret env STRIPE_SECRET
  on event payment_intent.succeeded
    query: UPDATE order SET status = 'paid' WHERE stripe_id = :event_id
    send email to :event_customer_email
      subject: "Payment confirmed"
```

### Background Jobs and Schedules

Timed tasks and async jobs run inside the same binary. No Redis, no Celery, no Sidekiq.

```kilnx
schedule cleanup every 24h
  query: DELETE FROM session WHERE expires_at < datetime('now')

job welcome-email
  query data: SELECT name, email FROM user WHERE id = :user_id
  generate pdf from template welcome with data
  send email to :email
    subject: "Welcome"
    attach: _generated_pdf
```

### Rate Limiting

Declarative. Per user or per IP. Wildcard paths.

```kilnx
limit /api/*
  requests: 100 per minute per user

limit /login
  requests: 5 per minute per ip
```

### Internationalization

Translations with `{t.key}` interpolation. Language detected from `Accept-Language` header or `?lang=` param.

```kilnx
translations
  en
    welcome: "Welcome back"
  pt
    welcome: "Bem vindo de volta"

page /dashboard requires auth
  "{t.welcome}, {current_user.name}"
```

### Declarative Tests

Test your app in the same language. No Selenium, no Cypress, no test framework.

```kilnx
test "user can register"
  visit /register
  fill name with "Alice"
  fill identity with "alice@test.com"
  fill password with "secret123"
  submit
  expect page /login contains "Log in"
```

```bash
$ kilnx test app.kilnx
Running 2 test(s):
  PASS  user can register
  PASS  about page loads
All 2 test(s) passed.
```

---

## Installation

```bash
git clone https://github.com/kilnx-org/kilnx.git
cd kilnx
go build -o kilnx ./cmd/kilnx/
sudo mv kilnx /usr/local/bin/
```

Requires Go 1.24+. No other dependencies.

## Commands

| Command | Description |
|---------|-------------|
| `kilnx run <file.kilnx>` | Start the server with hot reload and auto-migration |
| `kilnx build <file.kilnx>` | Compile to a standalone binary (~15MB) |
| `kilnx migrate <file.kilnx>` | Apply database migrations |
| `kilnx test <file.kilnx>` | Run declarative tests |
| `kilnx check <file.kilnx>` | Run static analysis |
| `kilnx version` | Print version |

## Deploy

```bash
# Development
kilnx run app.kilnx

# Production: compile and ship a single binary
kilnx build app.kilnx -o myapp
scp myapp server:~/
ssh server './myapp --port 3000 --db /data/app.db'
```

The binary contains everything: the HTTP server, htmx.js, SQLite, bcrypt, your app code. Copy it to any Linux/Mac/Windows machine and run. No Docker, no Node, no Python, no runtime.

## The 22 Keywords

| Keyword | What it does |
|---------|-------------|
| `config` | Database, port, env vars |
| `model` | Define types, generate tables |
| `auth` | Login, register, sessions, bcrypt |
| `permissions` | Role-based access control |
| `layout` | Page wrapper with `{page.content}` |
| `page` | GET route returning HTML |
| `action` | POST/PUT/DELETE mutations |
| `fragment` | Partial HTML for htmx |
| `api` | JSON endpoint |
| `stream` | Server-Sent Events |
| `socket` | Bidirectional WebSocket |
| `webhook` | External event receiver |
| `schedule` | Timed background tasks |
| `job` | Async background work |
| `query` | Inline SQL |
| `validate` | Server-side validation |
| `paginate` | Automatic pagination |
| `send email` | SMTP with templates |
| `redirect` | Route redirection |
| `on` | Result branching |
| `limit` | Rate limiting |
| `log` | Observability |
| `test` | Declarative tests |
| `translations` | i18n with `{t.key}` |
| `enqueue` | Dispatch async jobs |
| `broadcast` | WebSocket room messaging |

For comparison: Python has 35 keywords and does none of this without importing libraries. JavaScript has 64. Java has 67.

## Design Principles

**0. The complexity is the tool's fault, not the problem's.**

Most web apps are not complex. The tools are.

See [PRINCIPLES.md](PRINCIPLES.md) for all 11 principles and [GRAMMAR.md](GRAMMAR.md) for the complete language reference.

## Architecture

```
app.kilnx --> Lexer --> Tokens --> Parser --> AST
                                              |
                                    +---------+---------+
                                    |         |         |
                                 Models    Pages    Actions
                                    |         |         |
                                SQLite    HTML+htmx  SQL exec
                                (migrate) (render)   (mutate)
```

- **2 dependencies**: SQLite (pure Go) + bcrypt
- **~15,400 lines** of Go across 30 files
- **Single binary** output (~15MB)
- **Zero JavaScript** written by the developer

## License

[MIT](LICENSE) - (c) 2026 Andre Ahlert Junior
