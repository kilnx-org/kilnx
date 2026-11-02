<p align="center">
  <br>
  <a href="https://kilnx.org"><img src=".github/banner.svg" alt="kilnx" width="800"/></a>
  <br><br>
  <a href="https://kilnx.org">Website</a> &nbsp;&bull;&nbsp;
  <a href="docs/devs/reference/index.md">Reference</a> &nbsp;&bull;&nbsp;
  <a href="docs/devs/index.md">Docs</a> &nbsp;&bull;&nbsp;
  <a href="PRINCIPLES.md">Principles</a> &nbsp;&bull;&nbsp;
  <a href="https://github.com/kilnx-org/kilnx/issues">Roadmap</a>
  <br><br>
  <a href="https://github.com/kilnx-org/kilnx/actions"><img src="https://img.shields.io/github/actions/workflow/status/kilnx-org/kilnx/ci.yml?style=flat-square&label=CI" alt="CI"></a>
  <a href="https://github.com/kilnx-org/kilnx/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="MIT License"></a>
  <img src="https://img.shields.io/badge/dependencies-2-brightgreen?style=flat-square" alt="2 dependencies">
</p>

<img src=".github/divider.svg" width="100%"/>

Kilnx is a declarative backend language that compiles to a single binary. You define models, routes, queries, and auth in a `.kilnx` file. The compiler handles the rest: database migrations, HTTP routing, template rendering, session management, CSRF protection, and htmx integration.

One file. One binary. Zero JavaScript to write.

```kilnx
model user
  email: email required unique
  password: password required

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

page /login
  html
    <p>{params.error}</p>
    <form method="POST" action="/login">
      <input type="hidden" name="_csrf" value="{csrf}">
      <input name="identity" placeholder="Email">
      <input name="password" type="password" placeholder="Password">
      <button>Log in</button>
    </form>
    <a href="/register">Create an account</a>

page /register
  html
    <p>{params.error}</p>
    <form method="POST" action="/register">
      <input type="hidden" name="_csrf" value="{csrf}">
      <input name="identity" placeholder="Email">
      <input name="password" type="password" placeholder="Password">
      <button>Sign up</button>
    </form>

page /forgot-password
  html
    <form method="POST" action="/forgot-password">
      <input type="hidden" name="_csrf" value="{csrf}">
      <input name="email" placeholder="Email">
      <button>Send reset link</button>
    </form>

page /reset-password
  html
    <form method="POST" action="/reset-password?token={params.token}">
      <input type="hidden" name="_csrf" value="{csrf}">
      <input name="password" type="password" placeholder="New password">
      <input name="password_confirm" type="password" placeholder="Repeat it">
      <button>Reset password</button>
    </form>

page /tasks requires auth
  query tasks: SELECT id, title, done FROM task
               WHERE owner_id = :current_user.id
               ORDER BY created DESC paginate 20
  html
    <form method="POST" action="/tasks/new">
      <input type="hidden" name="_csrf" value="{csrf}">
      <input name="title" placeholder="New task">
      <button>Add</button>
    </form>
    <table>
    {{each tasks}}
    <tr>
      <td>{title}</td>
      <td>{{if done}}Yes{{end}}</td>
      <td><button hx-post="/tasks/{id}/delete" hx-vals='{"_csrf":"{csrf}"}'
                  hx-target="closest tr" hx-swap="outerHTML">Delete</button></td>
    </tr>
    {{end}}
    </table>

action /tasks/new requires auth
  validate task
  query: INSERT INTO task (title, owner_id) VALUES (:title, :current_user.id)
  redirect /tasks

action /tasks/:id/delete requires auth
  query: DELETE FROM task WHERE id = :id AND owner_id = :current_user.id
  respond fragment delete
```

Save it as `app.kilnx` and run `kilnx run app.kilnx`. You get registration, login with bcrypt, password reset, sessions, CSRF, per-user data, pagination, validation, inline htmx delete, and a SQLite database. Kilnx owns the auth logic; you own the HTML of every page.

## Why Kilnx?

**Zero decisions before the first useful line.** Create a file, write a page, run it. No framework to choose, no ORM to configure, no dependencies to install. `page / "Hello"` is a complete app.

**SQL is a first-class citizen.** Queries live inline, not in strings, not behind an ORM. The analyzer validates column references against your models at compile time. The optimizer rewrites `SELECT *` to only the columns your templates actually use.

**The binary is the deploy.** `kilnx build app.kilnx -o myapp` produces a single ~15MB executable that embeds your app, the HTTP server, htmx.js, SQLite, and bcrypt. Copy it to a server and run it.

**Security is default, not opt-in.** CSRF tokens, SQL parameter binding, HTML escaping, bcrypt hashing, HMAC-signed sessions, and webhook signature verification are all automatic. You have to make effort to be insecure.

## Quick Start

```bash
# macOS / Linux
brew install kilnx-org/tap/kilnx

# or via shell script
curl -fsSL https://raw.githubusercontent.com/kilnx-org/kilnx/main/install.sh | sh
```

Or build from source:

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

## Deploy

<a href="https://railway.com/deploy/kilnx"><img src="https://railway.com/button.svg" alt="Deploy on Railway" height="36"></a>

Or compile and copy a binary anywhere:

```bash
kilnx build app.kilnx -o myapp
scp myapp server:~/
ssh server './myapp'
```

## Editor Support

[VS Code extension](https://marketplace.visualstudio.com/items?itemName=atoolz.kilnx-vscode-toolkit) with syntax highlighting, diagnostics, completions, hover docs, and go-to-definition via the built-in LSP server.

## Try It Online

[kilnx.org/playground](https://kilnx.org/playground.html)

## What It Covers

Models, pages, actions, fragments, JSON APIs, Server-Sent Events, WebSockets, webhooks with HMAC verification, background jobs, scheduled tasks, rate limiting, i18n, email sending, PDF generation, declarative tests, and an LSP server.

Start with the [quickstart](docs/devs/quickstart.md) and [tutorials](docs/devs/tutorials/index.md). Every keyword and attribute is documented in the [reference](docs/devs/reference/index.md), generated from the compiler on every release (also available offline via `kilnx docs --list`).

## Examples

| Example | What it demonstrates |
|---------|---------------------|
| [hello](https://github.com/kilnx-org/examples/tree/main/hello) | Smallest possible app |
| [tasks](https://github.com/kilnx-org/examples/tree/main/tasks) | The app above: auth, per-user data, pagination, htmx delete |
| [chat](https://github.com/kilnx-org/examples/tree/main/chat) | Slack-style chat: channels, threads, DMs, WebSockets, SSE, uploads, jobs |

## Architecture

```
.kilnx → Lexer → Parser → Analyzer → Optimizer → Runtime
                              │           │          │
                         type/SQL      SELECT *    HTTP server
                         checks        rewriting   SQLite + htmx
```

Written in Go. The full test suite runs with race detection on every commit, and CI compiles the example above. [PRINCIPLES.md](PRINCIPLES.md) documents the 11 constitutional design principles.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

[MIT](LICENSE) &copy; 2026 Andre Ahlert Junior
