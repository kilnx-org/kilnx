<p align="center">
  <img src=".github/banner.svg" alt="kilnx" width="900"/>
</p>

<p align="center">
  <a href="https://github.com/kilnx-org/kilnx/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="MIT License"></a>
  <a href="https://github.com/kilnx-org/kilnx/actions"><img src="https://img.shields.io/github/actions/workflow/status/kilnx-org/kilnx/ci.yml?style=flat-square&label=CI" alt="CI"></a>
  <a href="https://htmx.org"><img src="https://img.shields.io/badge/htmx-2.x-blue?style=flat-square" alt="htmx 2.x"></a>
  <img src="https://img.shields.io/badge/dependencies-2-brightgreen?style=flat-square" alt="2 dependencies">
  <img src="https://img.shields.io/badge/keywords-27-e64a19?style=flat-square" alt="27 keywords">
  <img src="https://img.shields.io/badge/version-1.0.1-a855f7?style=flat-square" alt="v1.0.1">
</p>

<p align="center">
  Declarative backend language that compiles to a single binary.<br>
  27 keywords. Zero framework. SQL is the query language. HTML is the output.<br>
  <strong>htmx completed HTML. Kilnx completes htmx.</strong>
</p>

<p align="center">
  <a href="https://kilnx.dev">Website</a> &nbsp;&bull;&nbsp;
  <a href="GRAMMAR.md">Grammar Reference</a> &nbsp;&bull;&nbsp;
  <a href="PRINCIPLES.md">Design Principles</a> &nbsp;&bull;&nbsp;
  <a href="https://github.com/kilnx-org/kilnx/issues">Roadmap</a>
</p>

<br>

<img src=".github/divider.svg" width="100%"/>

<br>

## Why

Most web apps are lists, forms, dashboards, CRUDs. The complexity comes from the tools, not the problem. You shouldn't need a framework, an ORM, a template engine, and 200 files to show a list from a database.

Kilnx exists to prove this.

<br>

## Hello World

```kilnx
page /
  "Hello World"
```

```bash
$ kilnx run app.kilnx
kilnx serving on http://localhost:8080
```

One useful line. A running web server with htmx linked automatically.

<br>

<img src=".github/divider.svg" width="100%"/>

<br>

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
        <td><button hx-post="/tasks/{id}/delete"
                    hx-target="closest tr" hx-swap="outerHTML">Delete</button></td>
      </tr>
      {{end}}
    </table>

action /tasks/new requires auth
  validate task
  query: INSERT INTO task (title, owner) VALUES (:title, :current_user.id)
  redirect /tasks

action /tasks/:id/delete requires auth
  query: DELETE FROM task WHERE id = :id AND owner = :current_user.id
  respond fragment delete
```

This gives you: user registration, login with bcrypt, session management, CSRF protection, a searchable paginated table, form validation, inline delete via htmx, and a SQLite database. All from declarations.

<br>

<img src=".github/divider.svg" width="100%"/>

<br>

## Features

<table>
<tr>
<td width="50%">

### <img src=".github/icons/database.svg" width="18"/> Models as Single Source of Truth

Define data once. Kilnx generates the database table, server validation, HTML form, and client-side validation attributes.

```kilnx
model user
  name: text required min 2 max 100
  email: email unique
  role: option [admin, editor, viewer] default viewer
  active: bool default true
  created: timestamp auto
```

</td>
<td width="50%">

### <img src=".github/icons/lock.svg" width="18"/> Declarative Auth

Six lines. Registration, login, logout, bcrypt, sessions, and `current_user` available everywhere.

```kilnx
auth
  table: user
  identity: email
  password: password
  login: /login
  after login: /dashboard
```

Protect any route with `requires auth` or `requires admin`.

</td>
</tr>
<tr>
<td>

### <img src=".github/icons/code.svg" width="18"/> SQL is First-Class

No ORM. No query builder. SQL lives inline, right where you use it.

```kilnx
page /dashboard requires auth
  query stats: SELECT count(*) as total FROM user
  query posts: SELECT p.title, u.name as author
               FROM post p
               LEFT JOIN user u ON u.id = p.author_id
               WHERE p.status = 'published'
               ORDER BY p.created DESC
               paginate 10
  html
    <p>Total users: {stats.total}</p>
    {{each posts}}
    <article>
      <h3>{title}</h3>
      <span>by {author}</span>
    </article>
    {{end}}
```

</td>
<td>

### <img src=".github/icons/globe.svg" width="18"/> htmx-Native Fragments

Fragments return partial HTML that htmx swaps into the DOM. No JavaScript required.

```kilnx
action /users/:id/delete requires auth
  query: DELETE FROM user WHERE id = :id
  respond fragment delete

fragment /users/:id/card
  query user: SELECT name, email FROM user
              WHERE id = :id
  html
    <div class="card">
      <strong>{user.name}</strong>
      <span>{user.email}</span>
    </div>
```

</td>
</tr>
<tr>
<td>

### <img src=".github/icons/bolt.svg" width="18"/> Realtime

Server-Sent Events and WebSockets built in.

```kilnx
stream /notifications requires auth
  query: SELECT message FROM notification
         WHERE user_id = :current_user.id
         AND seen = false
  every 5s

socket /chat/:room requires auth
  on message
    query: INSERT INTO chat_message (body, author, room)
           VALUES (:body, :current_user.id, :room)
    broadcast to :room
```

</td>
<td>

### <img src=".github/icons/rocket.svg" width="18"/> Everything Else

```kilnx
# JSON API
api /api/v1/users requires auth
  query: SELECT id, name FROM user paginate 50

# Webhooks with HMAC verification
webhook /stripe secret env STRIPE_SECRET
  on event payment_intent.succeeded
    query: UPDATE order SET status = 'paid'
           WHERE stripe_id = :event_id

# Background jobs
schedule cleanup every 24h
  query: DELETE FROM session
         WHERE expires_at < datetime('now')

# Rate limiting
limit /api/*
  requests: 100 per minute per user

# i18n
translations
  en
    welcome: "Welcome back"
  pt
    welcome: "Bem vindo de volta"

# Declarative tests
test "user can register"
  visit /register
  fill name with "Alice"
  submit
  expect page /login contains "Log in"
```

</td>
</tr>
</table>

<br>

<img src=".github/divider.svg" width="100%"/>

<br>

## Installation

```bash
git clone https://github.com/kilnx-org/kilnx.git
cd kilnx
go build -o kilnx ./cmd/kilnx/
sudo mv kilnx /usr/local/bin/
```

Requires Go 1.24+. No other dependencies.

## Commands

```
kilnx run <file>       Start server with hot reload
kilnx build <file>     Compile to standalone binary (~15MB)
kilnx check <file>     Run static analysis
kilnx migrate <file>   Apply database migrations
kilnx test <file>      Run declarative tests
kilnx lsp              Start Language Server Protocol server
kilnx version          Print version
```

## Deploy

```bash
# Development
kilnx run app.kilnx

# Production: compile and ship
kilnx build app.kilnx -o myapp
scp myapp server:~/
ssh server './myapp --port 3000 --db /data/app.db'
```

The binary contains everything: HTTP server, htmx.js, SQLite, bcrypt, your app. Copy to any machine and run. No Docker, no Node, no Python, no runtime.

<br>

<img src=".github/divider.svg" width="100%"/>

<br>

## The 27 Keywords

| Category | Keywords |
|----------|----------|
| **Routes** | `page` `action` `fragment` `api` `stream` `socket` |
| **Data** | `model` `query` `queries` `validate` |
| **Auth** | `auth` `permissions` `limit` `requires` |
| **Flow** | `on` `redirect` `enqueue` `broadcast` `send` |
| **Infrastructure** | `config` `layout` `log` `schedule` `job` `webhook` `test` `translations` |

For comparison: Python has 35 keywords and does none of this without importing libraries. JavaScript has 64. Java has 67.

<br>

## Architecture

```
app.kilnx ─→ Lexer ─→ Parser ─→ Analyzer ─→ Optimizer ─→ Runtime
                                    │            │            │
                               Type checks   SELECT *    HTTP server
                               SQL checks    rewriting   SQLite
                               Security      JOIN        Sessions
                               warnings      pruning     htmx
```

| | |
|---|---|
| **Implementation** | ~19,000 lines of Go |
| **Dependencies** | 2 (SQLite + bcrypt) |
| **Binary size** | ~15MB |
| **Test suite** | 311 tests, race-tested |

<br>

## Design Principles

> **0. The complexity is the tool's fault, not the problem's.**
>
> Most web apps are not complex. The tools are.

See [PRINCIPLES.md](PRINCIPLES.md) for all 11 constitutional principles and [GRAMMAR.md](GRAMMAR.md) for the complete language reference.

<br>

<img src=".github/divider.svg" width="100%"/>

<br>

## Examples

| Example | LOC | Features |
|---------|-----|----------|
| [Hello World](examples/hello.kilnx) | 3 | Minimal page |
| [Blog](examples/blog.kilnx) | 94 | Auth, pagination, validation, fragments, tests |
| [CRM](../examples/crm/app.kilnx) | 813 | Models, JOINs, layouts, Tailwind, full CRUD |

<br>

## License

[MIT](LICENSE) &copy; 2026 Andre Ahlert Junior
