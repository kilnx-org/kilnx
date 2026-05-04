---
name: kilnx
description: "Write, read, debug, and refactor Kilnx code (.kilnx files). Kilnx is a declarative backend language that compiles to a standalone Go binary, pairs with htmx, and covers database, auth, pages, actions, fragments, SSE, WebSockets, jobs, schedules, webhooks, tests, i18n, LLM integration, and PDF generation in ~29 keywords. Use this skill whenever the user writes, edits, reviews, or asks about .kilnx files, or invokes the kilnx CLI (run/build/check/test/migrate/lsp)."
user-invocable: true
---

# Kilnx

You are programming in **Kilnx**, a declarative backend language. Ground truth is the compiler source at `/home/ahlert/Dev/kilnx-org/kilnx/` (lexer, parser, analyzer, runtime). When a feature's exact behavior, syntax edge case, or keyword is in doubt, read the Go source — not the markdown docs — since docs may lag behind code.

## Golden rules (do not violate)

1. **Declarative first.** If a keyword exists (`requires auth`, `validate`, `paginate`, `limit`, `schedule`), use it. Do not hand-roll middleware, loops, or glue.
2. **SQL is inline, not strings.** `query name: SELECT ...` directly in the block. No ORM, no string concat. Use `:param` binding, never format literals into SQL.
3. **HTML is the default output.** `page` returns full HTML, `fragment` returns a partial for htmx swap, `api` returns JSON. Pick the right one, don't fake JSON with `html`.
4. **Security is default, not opt-in.** CSRF, bcrypt, session HMAC, HTML escaping, SQL binding are already on. Never disable or bypass. Always use `:param` (bound), never `{param}` inside SQL.
5. **One file can be a full app.** Don't split prematurely. No imports, no modules — the language has none.
6. **Every layout MUST include `{kilnx.js}` in `<head>`.** Without it htmx + SSE break silently.
7. **Zero decisions / zero dependencies.** Never suggest adding JS frameworks, bundlers, npm, Redis, cron. Kilnx has all of this built in (`schedule`, `job`, `enqueue`, SSE, WebSocket).
8. **No comments explaining what.** Kilnx is already high-level. Only comment a non-obvious *why*. Comments start with `#`.

## The ~30 keywords (memorize)

`config`, `model`, `permissions`, `auth`, `layout`, `page`, `action`, `fragment`, `stream`, `socket`, `api`, `webhook`, `schedule`, `job`, `query`, `validate`, `paginate`, `send email`, `redirect`, `on`, `limit`, `log`, `test`, `translations`, `enqueue`, `broadcast`, `html`, `respond`, `llm`, `generate`.

If the user's instinct needs a keyword not on this list (middleware, controller, router, service), they are thinking in a framework, not in Kilnx. Reframe using a keyword above.

## Field types and constraints

- **Types:** `text`, `email`, `int`, `float`, `bool`, `timestamp`, `date`, `richtext`, `option [a, b]`, `password`, `image`, `phone`, `reference`, `url`, `decimal`, `file`, `tags`, `json`, `uuid`, `bigint`.
- **Constraints:** `required`, `unique`, `default <value>`, `auto` (for timestamps, dates, uuids), `auto_update` (for timestamps), `min <n>`, `max <n>`.
- **Relations:** `author: user required` declares a foreign key to `user`. Use the relation, not an `_id` column.

## Template syntax inside `html` blocks

- Interpolate: `{var}` (escaped), `{var|raw}` (unescaped, dangerous).
- Filters: `|upcase`, `|truncate:20`, `|date:"Jan 02, 2006"`, `|timeago`, `|currency:"$"`, `|pluralize:"item","items"`, `|default:"x"`, `|raw`.
- Control flow: `{{each items}}...{{else}}...{{end}}`, `{{if cond}}...{{end}}`.
- Layout placeholders: `{page.title}`, `{page.content}`, `{nav}`, `{kilnx.js}` (required).

## SQL parameter binding

- Inside SQL, always bind: `WHERE id = :id`, `VALUES (:title, :current_user.id)`.
- `:current_user.id`, `:current_user.email`, `:current_user.role` are always available in `requires auth` blocks.
- Route params (`/users/:id`) are available as `:id`.
- Form/JSON fields from the request are available as `:field_name`.
- **Never** put `{...}` template syntax inside a SQL string — it bypasses binding.

## Common shapes

### Minimal app
```kilnx
config
  database: env DATABASE_URL default "sqlite://app.db"
  port: env PORT default 8080
  secret: env SECRET_KEY required

page /
  "Hello World"
```

### CRUD page + action + fragment (htmx swap)
```kilnx
page /tasks layout main requires auth
  query tasks: select id, title, done from task
               where owner = :current_user.id
               order by created desc
               paginate 20
  html
    <ul id="task-list">
      {{each tasks}}
      <li>
        {title}
        <button hx-post="/tasks/{id}/delete" hx-target="closest li" hx-swap="outerHTML">x</button>
      </li>
      {{end}}
    </ul>

action /tasks/create method POST requires auth
  validate
    title: required min 2
  query: insert into task (title, owner) values (:title, :current_user.id)
  redirect /tasks

action /tasks/:id/delete method POST requires auth
  query: delete from task where id = :id and owner = :current_user.id
  respond fragment "#task-list" with query:
    select id, title, done from task
    where owner = :current_user.id
    order by created desc
```

### Realtime (SSE)
```kilnx
stream /notifications requires auth
  query: select message from notification
         where user_id = :current_user.id and seen = false
  every 5s
  event: update
```

### Realtime (WebSocket)
```kilnx
socket /chat/:room requires auth
  on connect
    query: select body, author.name, created from chat_message
           where room = :room order by created desc limit 50
    send history
  on message
    validate
      body: required max 500
    query: insert into chat_message (body, author, room)
           values (:body, :current_user.id, :room)
    broadcast to :room fragment chat-bubble
```

### Background work
```kilnx
job send-welcome
  query data: select name, email from user where id = :user_id
  send email to :data.email
    template: welcome
    subject: "Welcome {data.name}"

action /users/create method POST
  validate
    name: required
    email: required, is email
  query: insert into user (name, email) values (:name, :email)
  enqueue send-welcome with user_id: :id
  redirect /users

schedule cleanup every 24h
  query: delete from session where expires_at < datetime('now')
```

### Webhook
```kilnx
webhook /stripe secret env STRIPE_SECRET
  on event payment_intent.succeeded
    query: update orders set status = 'paid' where stripe_id = :event.id
  on event *
    query: insert into webhook_log (type, body) values (:event.type, :event.body)
```

### Test
```kilnx
test "user can create task"
  as editor
  visit /tasks/new
  fill title "Buy milk"
  submit
  expect page /tasks contains "Buy milk"
```

## `on` result handlers (inside action / api)

```kilnx
on success: redirect /users
on error: alert "Could not delete user"
on not found: status 404
on forbidden: redirect /login
```

## CLI

Working directory for the compiler is `/home/ahlert/Dev/kilnx-org/kilnx/`. Rebuild with `go build -o kilnx ./cmd/kilnx/`.

- `kilnx run app.kilnx` - dev server with hot reload
- `kilnx check app.kilnx` - static analysis (run this before declaring work done)
- `kilnx test app.kilnx` - run declarative tests
- `kilnx build app.kilnx -o myapp` - compile to standalone binary
- `kilnx migrate app.kilnx [--dry-run|--status]` - schema migrations
- `kilnx lsp` - start the language server
- `kilnx mcp` - start the Model Context Protocol server

## Workflow when editing a `.kilnx` file

1. **Read the whole file first.** Kilnx files declare a world; local edits can break global assumptions (a renamed model breaks every query).
2. **Prefer extending an existing block** over adding new top-level blocks. If a `page` already queries `user`, add the new column to its query rather than duplicating.
3. **After any non-trivial edit, run `kilnx check <file>`** from the project root (or the path the user is working in). Surface any warnings rather than ignoring them.
4. **If tests exist in the file, run `kilnx test <file>`** after changes.
5. **For UI changes, spin up `kilnx run`** and use the Playwright MCP to verify the page actually renders — `go build` passing does not mean the page works.

## Anti-patterns (reject these)

- Using `{:param}` or `'{param}'` inside SQL (string-interpolation is a vulnerability; use `:param` binding).
- Writing a `page` that returns raw JSON — use `api`.
- Writing an `action` with no `validate` block on user input.
- Writing a `page` that mutates data — mutations go in `action`.
- Adding a layout without `{kilnx.js}`.
- Introducing JS frameworks, bundlers, or a separate frontend — Kilnx targets htmx.
- Suggesting Redis/cron/Celery — use `schedule`, `job`, `enqueue` instead.
- Comments that restate the code. Only comment non-obvious *why*.

## When the user asks for something Kilnx cannot do

Kilnx is deliberately narrow. If the request genuinely cannot be expressed (custom binary protocols, heavy numerical compute, GPU work), say so plainly and suggest calling out to an external service via `webhook` or `api`. Do not invent syntax.

## Reference source (read when stuck — prefer code over docs)

Compiler lives at `/home/ahlert/Dev/kilnx-org/kilnx/`. Read the Go source directly; it is authoritative.

- **Lexer** (tokens, keywords): `internal/lexer/` — grep here to confirm a keyword exists and how it tokenizes.
- **Parser** (grammar, block structure): `internal/parser/` — authoritative for valid syntax shapes; supersedes any markdown grammar doc.
- **Analyzer** (semantic rules, validation, required/optional fields): `internal/analyzer/` — read to understand what `kilnx check` will accept or reject.
- **Runtime** (request handling, SSE, WebSocket, jobs, schedules, sessions, CSRF): `internal/runtime/` — authoritative for actual behavior at runtime.
- **Database** (migrations, query execution, binding): `internal/database/`.
- **Build / optimizer / LSP**: `internal/build/`, `internal/optimizer/`, `internal/lsp/`.
- **CLI entrypoint**: `cmd/kilnx/` — flags and subcommand behavior.
- **Runnable examples**: `/home/ahlert/Dev/kilnx-org/examples/` (`chat`, `hello`) — working `.kilnx` programs to pattern-match against.

Workflow when unsure about syntax or behavior:
1. `grep -r "keyword" /home/ahlert/Dev/kilnx-org/kilnx/internal/lexer/` to confirm the keyword exists.
2. Read the relevant `internal/parser/` file to see how it parses.
3. Read `internal/analyzer/` for semantic constraints.
4. Read `internal/runtime/` for execution behavior.

Markdown docs (`GRAMMAR.md`, `FEATURES.md`, `PRINCIPLES.md`) at the repo root may exist but can drift; source code wins on conflicts.
