<p align="center">
  <br>
  <a href="https://kilnx.dev"><img src=".github/banner.svg" alt="kilnx" width="800"/></a>
  <br><br>
  <a href="https://kilnx.dev">Website</a> &nbsp;&bull;&nbsp;
  <a href="GRAMMAR.md">Grammar</a> &nbsp;&bull;&nbsp;
  <a href="FEATURES.md">Features</a> &nbsp;&bull;&nbsp;
  <a href="PRINCIPLES.md">Principles</a> &nbsp;&bull;&nbsp;
  <a href="https://github.com/kilnx-org/kilnx/issues">Roadmap</a>
  <br><br>
  <a href="https://github.com/kilnx-org/kilnx/actions"><img src="https://img.shields.io/github/actions/workflow/status/kilnx-org/kilnx/ci.yml?style=flat-square&label=CI" alt="CI"></a>
  <a href="https://github.com/kilnx-org/kilnx/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="MIT License"></a>
  <img src="https://img.shields.io/badge/keywords-27-e64a19?style=flat-square" alt="27 keywords">
  <img src="https://img.shields.io/badge/dependencies-2-brightgreen?style=flat-square" alt="2 dependencies">
</p>

<img src=".github/divider.svg" width="100%"/>

Kilnx is a declarative backend language that compiles to a single binary. You define models, routes, queries, and auth in a `.kilnx` file. The compiler handles the rest: database migrations, HTTP routing, template rendering, session management, CSRF protection, and htmx integration.

27 keywords. 2 dependencies (SQLite + bcrypt). Zero JavaScript.

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
               ORDER BY created DESC paginate 20
  html
    {{each tasks}}
    <tr>
      <td>{title}</td>
      <td>{{if done}}Yes{{end}}</td>
      <td><button hx-post="/tasks/{id}/delete"
                  hx-target="closest tr" hx-swap="outerHTML">Delete</button></td>
    </tr>
    {{end}}

action /tasks/new requires auth
  validate task
  query: INSERT INTO task (title, owner) VALUES (:title, :current_user.id)
  redirect /tasks

action /tasks/:id/delete requires auth
  query: DELETE FROM task WHERE id = :id AND owner = :current_user.id
  respond fragment delete
```

This gives you: registration, login with bcrypt, sessions, CSRF, paginated search, validation, inline htmx delete, and a SQLite database.

## Why Kilnx?

**Zero decisions before the first useful line.** Create a file, write a page, run it. No framework to choose, no ORM to configure, no dependencies to install. `page / "Hello"` is a complete app.

**SQL is a first-class citizen.** Queries live inline, not in strings, not behind an ORM. The analyzer validates column references against your models at compile time. The optimizer rewrites `SELECT *` to only the columns your templates actually use.

**The binary is the deploy.** `kilnx build app.kilnx -o myapp` produces a single ~15MB executable that embeds your app, the HTTP server, htmx.js, SQLite, and bcrypt. Copy it to a server and run it.

**Security is default, not opt-in.** CSRF tokens, SQL parameter binding, HTML escaping, bcrypt hashing, HMAC-signed sessions, and webhook signature verification are all automatic. You have to make effort to be insecure.

## Quick Start

```bash
git clone https://github.com/kilnx-org/kilnx.git
cd kilnx && go build -o kilnx ./cmd/kilnx/
```

```bash
kilnx run app.kilnx           # dev server with hot reload
kilnx build app.kilnx -o app  # compile to standalone binary
kilnx check app.kilnx         # static analysis
kilnx test app.kilnx          # run declarative tests
```

## What It Covers

Models, pages, actions, fragments, JSON APIs, Server-Sent Events, WebSockets, webhooks with HMAC verification, background jobs, scheduled tasks, rate limiting, i18n, email sending, PDF generation, declarative tests, and an LSP server.

See [FEATURES.md](FEATURES.md) for examples of each, and [GRAMMAR.md](GRAMMAR.md) for the complete language reference.

## Examples

| Example | LOC | What it demonstrates |
|---------|-----|---------------------|
| [hello.kilnx](examples/hello.kilnx) | 3 | Minimal app |
| [blog.kilnx](examples/blog.kilnx) | 94 | Auth, pagination, validation, fragments, declarative tests |
| [CRM](https://github.com/kilnx-org/examples/blob/main/crm/app.kilnx) | 813 | Full CRUD with JOINs, layouts, Tailwind, 3 related models |

## Architecture

```
.kilnx → Lexer → Parser → Analyzer → Optimizer → Runtime
                              │           │          │
                         type/SQL      SELECT *    HTTP server
                         checks        rewriting   SQLite + htmx
```

~19,000 lines of Go. 311 tests with race detection. [PRINCIPLES.md](PRINCIPLES.md) documents the 11 constitutional design principles.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

[MIT](LICENSE) &copy; 2026 Andre Ahlert Junior
