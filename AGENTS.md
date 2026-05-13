<!--
Generated file. DO NOT EDIT.
Run `go generate ./...` to update.
Source: cmd/kilnx-gendocs/templates/agents.md.tmpl + spec registry.
-->

# Kilnx for AI agents

You are an AI coding agent working on a `.kilnx` project. **Kilnx is a declarative backend language that was not in your training data.** Do not infer syntax. Do not invent keywords. The catalog at the bottom of this file is the only source of truth, regenerated from the compiler on every release.

## Hard rules

1. **Before writing any `.kilnx` code**, run `kilnx docs --list` to see what exists.
2. **Before using a keyword or attribute**, run `kilnx docs <name>` for its syntax, scope, and constraints.
3. **After every edit**, run `kilnx check <file>` and fix every error before continuing. The analyzer catches type errors, undeclared fields, illegal scopes, security misuse, and migration data-loss risks.
4. **If `kilnx docs <name>` returns "no entity named", the construct does not exist.** Do not invent it. Pick a real one from `--list` or rephrase the design.
5. **Reference docs at [`docs/devs/reference/`](docs/devs/reference/) are authoritative.** Source-driven, regenerated on every merge.

## Reading order

Read these in this order before touching code. Time investment: roughly 20 minutes total.

1. [`README.md`](README.md): one-page overview with a complete working app.
2. [`docs/devs/concepts/mental-model.md`](docs/devs/concepts/mental-model.md): how to think in Kilnx (declarative, indent-significant, AST-driven runtime).
3. [`docs/devs/concepts/models-and-data.md`](docs/devs/concepts/models-and-data.md): tables, fields, migrations.
4. [`docs/devs/concepts/pages-actions-fragments.md`](docs/devs/concepts/pages-actions-fragments.md): the HTTP layer.
5. [`docs/devs/concepts/auth-and-permissions.md`](docs/devs/concepts/auth-and-permissions.md): identity, sessions, authorization.
6. [`docs/devs/concepts/jobs-and-schedules.md`](docs/devs/concepts/jobs-and-schedules.md): background work and cron.
7. [`docs/devs/tutorials/01-todo-app.md`](docs/devs/tutorials/01-todo-app.md): hands-on Todo CRUD.
8. [`docs/devs/tutorials/02-auth-and-sessions.md`](docs/devs/tutorials/02-auth-and-sessions.md): add login and roles.
9. [`docs/devs/tutorials/03-background-jobs.md`](docs/devs/tutorials/03-background-jobs.md): add a `schedule` and a `job`.

The reference under [`docs/devs/reference/`](docs/devs/reference/) is a lookup, not a read-through. Jump to it when you need details on one keyword or attribute.

## Workflow loop

```
plan -> read concept -> kilnx docs <name> -> edit .kilnx -> kilnx check -> kilnx run
```

Repeat the loop in small steps. The compiler is your fastest feedback channel.

## Common pitfalls

- **Indentation is significant.** Two spaces per level. Mixed tabs and spaces are rejected.
- **No `main`, no router, no handler signature.** Routes exist because you wrote `page /path` or `action /path`. Adding a function in Go is the wrong instinct.
- **Templates are not Go `html/template`.** Curly-brace expressions like `{field}` are field lookups; `{{if cond}}...{{end}}` are control flow. See the `html` attribute docs.
- **Secrets via env vars only.** Templates read environment variables as `:env.NAME`. Do not paste credentials into `.kilnx` files.
- **Migrations are additive.** `kilnx migrate` never drops or alters columns destructively. Drift is reported as warnings; the author resolves manually.
- **`fetch` is the only escape hatch.** When a primitive does not exist, model the call as an outbound HTTP request via `fetch`, not as a custom Go module.

## Tooling shortcuts

- `kilnx docs --list`: every keyword and attribute with one-line summaries.
- `kilnx docs --search <query>`: full-text search across names and descriptions.
- `kilnx docs <name>`: complete entry for one construct (syntax, scope, defaults, since).
- `kilnx check <file>`: full analyzer pass, exit non-zero on diagnostics.
- `kilnx run <file>`: dev server with migrations applied on startup.
- `kilnx build <file> -o <bin>`: standalone binary, single-file deploy.
- `kilnx version`: print version and build date.

---

## Catalog

The remainder of this document is auto-generated from the compiler's spec registry.

### Keywords (22)

| Keyword | Summary | Since |
|---|---|---|
| [`action`](docs/devs/reference/keywords/action.md) | Declare a state-changing endpoint (POST/PUT/DELETE). | 0.1.0 |
| [`agent`](docs/devs/reference/keywords/agent.md) | Agentic loop running as a `claude` CLI subprocess. | 0.1.3 |
| [`api`](docs/devs/reference/keywords/api.md) | Declare a JSON endpoint. | 0.1.0 |
| [`auth`](docs/devs/reference/keywords/auth.md) | Configure email/password authentication. | 0.1.0 |
| [`config`](docs/devs/reference/keywords/config.md) | Application-wide configuration. | 0.1.0 |
| [`fragment`](docs/devs/reference/keywords/fragment.md) | Return partial HTML (no document wrapper) for htmx or includes. | 0.1.0 |
| [`job`](docs/devs/reference/keywords/job.md) | Asynchronous background task triggered by `enqueue`. | 0.1.0 |
| [`layout`](docs/devs/reference/keywords/layout.md) | Define a reusable HTML wrapper for pages. | 0.1.0 |
| [`limit`](docs/devs/reference/keywords/limit.md) | Rate-limit requests matching a path pattern. | 0.1.0 |
| [`llm`](docs/devs/reference/keywords/llm.md) | Call an LLM. Choose `response` (single-shot / chat) or `agent` (subprocess loop with tools). | 0.1.3 |
| [`log`](docs/devs/reference/keywords/log.md) | Configure runtime logging output. | 0.1.0 |
| [`model`](docs/devs/reference/keywords/model.md) | Declare a database table with typed fields and constraints. | 0.1.0 |
| [`page`](docs/devs/reference/keywords/page.md) | Define an HTTP route and its view. | 0.1.0 |
| [`permissions`](docs/devs/reference/keywords/permissions.md) | Role-based access control rules. | 0.1.0 |
| [`query`](docs/devs/reference/keywords/query.md) | Run a SQL query and bind its result. Used both top-level (named query) and inside bodies. | 0.1.0 |
| [`response`](docs/devs/reference/keywords/response.md) | Single-shot / chat / streaming Messages API call. | 0.1.3 |
| [`schedule`](docs/devs/reference/keywords/schedule.md) | Background task executed on a fixed interval or cron expression. | 0.1.0 |
| [`socket`](docs/devs/reference/keywords/socket.md) | WebSocket endpoint for bidirectional real-time messaging. | 0.1.0 |
| [`stream`](docs/devs/reference/keywords/stream.md) | Server-Sent Events (SSE) endpoint that pushes data on an interval. | 0.1.0 |
| [`test`](docs/devs/reference/keywords/test.md) | End-to-end browser-style test scenario. | 0.1.0 |
| [`translations`](docs/devs/reference/keywords/translations.md) | i18n translation strings keyed by language and key. | 0.1.0 |
| [`webhook`](docs/devs/reference/keywords/webhook.md) | Receive external events (Stripe, GitHub, etc.) at a path. | 0.1.0 |

### Field types

Recognized inside `model` blocks: `bigint`, `bool`, `date`, `decimal`, `email`, `file`, `float`, `image`, `int`, `json`, `option`, `password`, `phone`, `reference`, `richtext`, `tags`, `text`, `timestamp`, `url`, `uuid`.

### Field constraints

Recognized as field modifiers: `auto`, `auto_update`, `default`, `max`, `min`, `required`, `unique`.

### Attributes by scope

Each section lists the attributes valid inside that parent keyword. An attribute appearing in multiple scopes is listed in each.

#### `action`

| Attribute | Summary | Since |
|---|---|---|
| [`broadcast`](docs/devs/reference/attributes/broadcast.md) | Push a message to all clients in a websocket room. | 0.1.0 |
| [`enqueue`](docs/devs/reference/attributes/enqueue.md) | Enqueue an asynchronous `job` with named parameters. | 0.1.0 |
| [`fetch`](docs/devs/reference/attributes/fetch.md) | Make an HTTP request to an external API. | 0.1.0 |
| [`generate`](docs/devs/reference/attributes/generate.md) | Generate a PDF from a template and a data query. | 0.1.0 |
| [`html`](docs/devs/reference/attributes/html.md) | Render an inline HTML template block. | 0.1.0 |
| [`method`](docs/devs/reference/attributes/method.md) | Restrict the HTTP method for the parent route. | 0.1.0 |
| [`on`](docs/devs/reference/attributes/on.md) | Conditional or event handler. | 0.1.0 |
| [`redirect`](docs/devs/reference/attributes/redirect.md) | Redirect to another URL. | 0.1.0 |
| [`requires`](docs/devs/reference/attributes/requires.md) | Require authentication or a specific role/permission. | 0.1.0 |
| [`respond`](docs/devs/reference/attributes/respond.md) | Return a partial HTML response (htmx fragment) or status code. | 0.1.0 |
| [`send`](docs/devs/reference/attributes/send.md) | Send an email. | 0.1.0 |
| [`validate`](docs/devs/reference/attributes/validate.md) | Validate input against a model or per-field rules. | 0.1.0 |

#### `agent`

| Attribute | Summary | Since |
|---|---|---|
| [`cwd`](docs/devs/reference/attributes/cwd.md) | Working directory for the agent subprocess. | 0.1.3 |
| [`max-budget-usd`](docs/devs/reference/attributes/max-budget-usd.md) | Hard cost cap in USD per agent invocation. | 0.1.3 |
| [`max-turns`](docs/devs/reference/attributes/max-turns.md) | Maximum agentic turns before forced stop. | 0.1.3 |
| [`mcp`](docs/devs/reference/attributes/mcp.md) | Comma-separated names of MCP servers declared in `config mcp <name>`. | 0.1.3 |
| [`permission-mode`](docs/devs/reference/attributes/permission-mode.md) | Tool-use permission policy: `plan`, `acceptEdits`, or `bypassPermissions`. | 0.1.3 |
| [`pool`](docs/devs/reference/attributes/pool.md) | Reserved: number of warm `claude` subprocesses to keep alive (not yet implemented). | 0.1.3 |
| [`pool-idle-ttl`](docs/devs/reference/attributes/pool-idle-ttl.md) | Reserved: idle TTL before pooled subprocesses are killed (not yet implemented). | 0.1.3 |
| [`resume`](docs/devs/reference/attributes/resume.md) | Resume an existing claude session by UUID (supports `:param`). | 0.1.3 |
| [`show-tools`](docs/devs/reference/attributes/show-tools.md) | Stream tool_use/tool_result frames in addition to assistant text. | 0.1.3 |
| [`tools`](docs/devs/reference/attributes/tools.md) | Comma-separated list of tools the agent may use. | 0.1.3 |

#### `api`

| Attribute | Summary | Since |
|---|---|---|
| [`broadcast`](docs/devs/reference/attributes/broadcast.md) | Push a message to all clients in a websocket room. | 0.1.0 |
| [`enqueue`](docs/devs/reference/attributes/enqueue.md) | Enqueue an asynchronous `job` with named parameters. | 0.1.0 |
| [`fetch`](docs/devs/reference/attributes/fetch.md) | Make an HTTP request to an external API. | 0.1.0 |
| [`generate`](docs/devs/reference/attributes/generate.md) | Generate a PDF from a template and a data query. | 0.1.0 |
| [`html`](docs/devs/reference/attributes/html.md) | Render an inline HTML template block. | 0.1.0 |
| [`method`](docs/devs/reference/attributes/method.md) | Restrict the HTTP method for the parent route. | 0.1.0 |
| [`on`](docs/devs/reference/attributes/on.md) | Conditional or event handler. | 0.1.0 |
| [`requires`](docs/devs/reference/attributes/requires.md) | Require authentication or a specific role/permission. | 0.1.0 |
| [`respond`](docs/devs/reference/attributes/respond.md) | Return a partial HTML response (htmx fragment) or status code. | 0.1.0 |
| [`send`](docs/devs/reference/attributes/send.md) | Send an email. | 0.1.0 |
| [`validate`](docs/devs/reference/attributes/validate.md) | Validate input against a model or per-field rules. | 0.1.0 |

#### `auth`

| Attribute | Summary | Since |
|---|---|---|
| [`after_login`](docs/devs/reference/attributes/after_login.md) | Default redirect target after successful login. | 0.1.0 |
| [`forgot`](docs/devs/reference/attributes/forgot.md) | Path for the forgot-password flow. | 0.1.0 |
| [`identity`](docs/devs/reference/attributes/identity.md) | Field used as the unique login identifier. | 0.1.0 |
| [`login`](docs/devs/reference/attributes/login.md) | Path served as the login form. | 0.1.0 |
| [`logout`](docs/devs/reference/attributes/logout.md) | Path that terminates the session. | 0.1.0 |
| [`password`](docs/devs/reference/attributes/password.md) | Field storing the bcrypt password hash. | 0.1.0 |
| [`register`](docs/devs/reference/attributes/register.md) | Path served as the registration form. | 0.1.0 |
| [`reset`](docs/devs/reference/attributes/reset.md) | Path for the password-reset confirmation page. | 0.1.0 |
| [`superuser`](docs/devs/reference/attributes/superuser.md) | Identity that bypasses role checks. | 0.1.0 |
| [`table`](docs/devs/reference/attributes/table.md) | Name of the model storing user accounts. | 0.1.0 |

#### `config`

| Attribute | Summary | Since |
|---|---|---|
| [`cors`](docs/devs/reference/attributes/cors.md) | Comma-separated list of allowed CORS origins. | 0.1.0 |
| [`database`](docs/devs/reference/attributes/database.md) | Database connection URL. | 0.1.0 |
| [`default_language`](docs/devs/reference/attributes/default_language.md) | Fallback language code for translations. | 0.1.0 |
| [`detect_language`](docs/devs/reference/attributes/detect_language.md) | How to detect the user's language at request time. | 0.1.0 |
| [`name`](docs/devs/reference/attributes/name.md) | Human-readable application name. | 0.1.0 |
| [`port`](docs/devs/reference/attributes/port.md) | TCP port to listen on. | 0.1.0 |
| [`secret`](docs/devs/reference/attributes/secret.md) | Secret key for session cookies and CSRF tokens. | 0.1.0 |
| [`static`](docs/devs/reference/attributes/static.md) | Filesystem path served as static assets. | 0.1.0 |
| [`uploads`](docs/devs/reference/attributes/uploads.md) | Filesystem path and max size (MB) for user uploads. | 0.1.0 |
| [`workspace-root`](docs/devs/reference/attributes/workspace-root.md) | Filesystem root for agent cwd resolution and tmp dirs. | 0.1.3 |

#### `fragment`

| Attribute | Summary | Since |
|---|---|---|
| [`broadcast`](docs/devs/reference/attributes/broadcast.md) | Push a message to all clients in a websocket room. | 0.1.0 |
| [`enqueue`](docs/devs/reference/attributes/enqueue.md) | Enqueue an asynchronous `job` with named parameters. | 0.1.0 |
| [`fetch`](docs/devs/reference/attributes/fetch.md) | Make an HTTP request to an external API. | 0.1.0 |
| [`generate`](docs/devs/reference/attributes/generate.md) | Generate a PDF from a template and a data query. | 0.1.0 |
| [`html`](docs/devs/reference/attributes/html.md) | Render an inline HTML template block. | 0.1.0 |
| [`on`](docs/devs/reference/attributes/on.md) | Conditional or event handler. | 0.1.0 |
| [`respond`](docs/devs/reference/attributes/respond.md) | Return a partial HTML response (htmx fragment) or status code. | 0.1.0 |
| [`send`](docs/devs/reference/attributes/send.md) | Send an email. | 0.1.0 |
| [`validate`](docs/devs/reference/attributes/validate.md) | Validate input against a model or per-field rules. | 0.1.0 |

#### `job`

| Attribute | Summary | Since |
|---|---|---|
| [`broadcast`](docs/devs/reference/attributes/broadcast.md) | Push a message to all clients in a websocket room. | 0.1.0 |
| [`enqueue`](docs/devs/reference/attributes/enqueue.md) | Enqueue an asynchronous `job` with named parameters. | 0.1.0 |
| [`fetch`](docs/devs/reference/attributes/fetch.md) | Make an HTTP request to an external API. | 0.1.0 |
| [`generate`](docs/devs/reference/attributes/generate.md) | Generate a PDF from a template and a data query. | 0.1.0 |
| [`html`](docs/devs/reference/attributes/html.md) | Render an inline HTML template block. | 0.1.0 |
| [`on`](docs/devs/reference/attributes/on.md) | Conditional or event handler. | 0.1.0 |
| [`respond`](docs/devs/reference/attributes/respond.md) | Return a partial HTML response (htmx fragment) or status code. | 0.1.0 |
| [`retry`](docs/devs/reference/attributes/retry.md) | Maximum retry attempts before the job is marked failed. | 0.1.0 |
| [`send`](docs/devs/reference/attributes/send.md) | Send an email. | 0.1.0 |
| [`validate`](docs/devs/reference/attributes/validate.md) | Validate input against a model or per-field rules. | 0.1.0 |

#### `layout`

| Attribute | Summary | Since |
|---|---|---|
| [`html`](docs/devs/reference/attributes/html.md) | Render an inline HTML template block. | 0.1.0 |

#### `limit`

| Attribute | Summary | Since |
|---|---|---|
| [`delay`](docs/devs/reference/attributes/delay.md) | Seconds to delay over-budget requests before responding. | 0.1.0 |
| [`message`](docs/devs/reference/attributes/message.md) | Custom error message returned when the limit is exceeded. | 0.1.0 |
| [`requests`](docs/devs/reference/attributes/requests.md) | Rate-limit budget (in `limit`) or request log strategy (in `log`). | 0.1.0 |

#### `llm`

| Attribute | Summary | Since |
|---|---|---|
| [`max-tokens`](docs/devs/reference/attributes/max-tokens.md) | Maximum tokens to generate in the response. | 0.1.3 |
| [`system`](docs/devs/reference/attributes/system.md) | System prompt for this `llm` call. | 0.1.3 |
| [`temperature`](docs/devs/reference/attributes/temperature.md) | Sampling temperature (0.0-1.0). | 0.1.3 |

#### `log`

| Attribute | Summary | Since |
|---|---|---|
| [`errors`](docs/devs/reference/attributes/errors.md) | Error reporting strategy. Append `stacktrace` to include stacks. | 0.1.0 |
| [`level`](docs/devs/reference/attributes/level.md) | Minimum severity to log (debug, info, warn, error). | 0.1.0 |
| [`requests`](docs/devs/reference/attributes/requests.md) | Rate-limit budget (in `limit`) or request log strategy (in `log`). | 0.1.0 |
| [`slow_query`](docs/devs/reference/attributes/slow_query.md) | Threshold above which a SQL query is logged as slow. | 0.1.0 |

#### `model`

| Attribute | Summary | Since |
|---|---|---|
| [`auto`](docs/devs/reference/attributes/auto.md) | Auto-populate the field on insert. | 0.1.0 |
| [`auto_update`](docs/devs/reference/attributes/auto_update.md) | Auto-populate on insert and update. | 0.1.0 |
| [`custom`](docs/devs/reference/attributes/custom.md) | Load custom field definitions from an external manifest. | 0.1.0 |
| [`default`](docs/devs/reference/attributes/default.md) | Default value when no value is supplied. | 0.1.0 |
| [`dynamic_fields`](docs/devs/reference/attributes/dynamic_fields.md) | Allow fields to be defined at runtime via the database. | 0.1.0 |
| [`index`](docs/devs/reference/attributes/index.md) | Non-unique composite index for query performance. | 0.1.0 |
| [`max`](docs/devs/reference/attributes/max.md) | Maximum value (numeric) or length (string). | 0.1.0 |
| [`min`](docs/devs/reference/attributes/min.md) | Minimum value (numeric) or length (string). | 0.1.0 |
| [`required`](docs/devs/reference/attributes/required.md) | Field must have a non-null value. | 0.1.0 |
| [`tenant`](docs/devs/reference/attributes/tenant.md) | Scope all rows of this model to a tenant. | 0.1.0 |
| [`unique`](docs/devs/reference/attributes/unique.md) | Field value must be unique across all rows. | 0.1.0 |

#### `page`

| Attribute | Summary | Since |
|---|---|---|
| [`broadcast`](docs/devs/reference/attributes/broadcast.md) | Push a message to all clients in a websocket room. | 0.1.0 |
| [`enqueue`](docs/devs/reference/attributes/enqueue.md) | Enqueue an asynchronous `job` with named parameters. | 0.1.0 |
| [`fetch`](docs/devs/reference/attributes/fetch.md) | Make an HTTP request to an external API. | 0.1.0 |
| [`generate`](docs/devs/reference/attributes/generate.md) | Generate a PDF from a template and a data query. | 0.1.0 |
| [`html`](docs/devs/reference/attributes/html.md) | Render an inline HTML template block. | 0.1.0 |
| [`method`](docs/devs/reference/attributes/method.md) | Restrict the HTTP method for the parent route. | 0.1.0 |
| [`on`](docs/devs/reference/attributes/on.md) | Conditional or event handler. | 0.1.0 |
| [`redirect`](docs/devs/reference/attributes/redirect.md) | Redirect to another URL. | 0.1.0 |
| [`requires`](docs/devs/reference/attributes/requires.md) | Require authentication or a specific role/permission. | 0.1.0 |
| [`respond`](docs/devs/reference/attributes/respond.md) | Return a partial HTML response (htmx fragment) or status code. | 0.1.0 |
| [`send`](docs/devs/reference/attributes/send.md) | Send an email. | 0.1.0 |
| [`title`](docs/devs/reference/attributes/title.md) | Set the HTML document title. | 0.1.0 |
| [`validate`](docs/devs/reference/attributes/validate.md) | Validate input against a model or per-field rules. | 0.1.0 |

#### `response`

| Attribute | Summary | Since |
|---|---|---|
| [`history`](docs/devs/reference/attributes/history.md) | SQL query whose rows become the message history. | 0.1.3 |
| [`stream-swap`](docs/devs/reference/attributes/stream-swap.md) | Hyperstream swap style (`append`, `inner`, `outer`, ...). | 0.1.3 |

#### `schedule`

| Attribute | Summary | Since |
|---|---|---|
| [`broadcast`](docs/devs/reference/attributes/broadcast.md) | Push a message to all clients in a websocket room. | 0.1.0 |
| [`enqueue`](docs/devs/reference/attributes/enqueue.md) | Enqueue an asynchronous `job` with named parameters. | 0.1.0 |
| [`every`](docs/devs/reference/attributes/every.md) | Cadence at which the parent task runs. | 0.1.0 |
| [`fetch`](docs/devs/reference/attributes/fetch.md) | Make an HTTP request to an external API. | 0.1.0 |
| [`generate`](docs/devs/reference/attributes/generate.md) | Generate a PDF from a template and a data query. | 0.1.0 |
| [`html`](docs/devs/reference/attributes/html.md) | Render an inline HTML template block. | 0.1.0 |
| [`on`](docs/devs/reference/attributes/on.md) | Conditional or event handler. | 0.1.0 |
| [`respond`](docs/devs/reference/attributes/respond.md) | Return a partial HTML response (htmx fragment) or status code. | 0.1.0 |
| [`send`](docs/devs/reference/attributes/send.md) | Send an email. | 0.1.0 |
| [`validate`](docs/devs/reference/attributes/validate.md) | Validate input against a model or per-field rules. | 0.1.0 |

#### `socket`

| Attribute | Summary | Since |
|---|---|---|
| [`broadcast`](docs/devs/reference/attributes/broadcast.md) | Push a message to all clients in a websocket room. | 0.1.0 |
| [`on`](docs/devs/reference/attributes/on.md) | Conditional or event handler. | 0.1.0 |

#### `stream`

| Attribute | Summary | Since |
|---|---|---|
| [`event`](docs/devs/reference/attributes/event.md) | SSE event name to emit on each tick. | 0.1.0 |
| [`every`](docs/devs/reference/attributes/every.md) | Cadence at which the parent task runs. | 0.1.0 |

#### `test`

| Attribute | Summary | Since |
|---|---|---|
| [`as`](docs/devs/reference/attributes/as.md) | Run subsequent steps as a given role or user. | 0.1.0 |
| [`expect`](docs/devs/reference/attributes/expect.md) | Assert a condition on the current state. | 0.1.0 |
| [`fill`](docs/devs/reference/attributes/fill.md) | Fill a form field with a value. | 0.1.0 |
| [`submit`](docs/devs/reference/attributes/submit.md) | Submit the current form. | 0.1.0 |
| [`visit`](docs/devs/reference/attributes/visit.md) | Navigate to a URL. | 0.1.0 |

#### `webhook`

| Attribute | Summary | Since |
|---|---|---|
| [`broadcast`](docs/devs/reference/attributes/broadcast.md) | Push a message to all clients in a websocket room. | 0.1.0 |
| [`enqueue`](docs/devs/reference/attributes/enqueue.md) | Enqueue an asynchronous `job` with named parameters. | 0.1.0 |
| [`fetch`](docs/devs/reference/attributes/fetch.md) | Make an HTTP request to an external API. | 0.1.0 |
| [`generate`](docs/devs/reference/attributes/generate.md) | Generate a PDF from a template and a data query. | 0.1.0 |
| [`html`](docs/devs/reference/attributes/html.md) | Render an inline HTML template block. | 0.1.0 |
| [`on`](docs/devs/reference/attributes/on.md) | Conditional or event handler. | 0.1.0 |
| [`respond`](docs/devs/reference/attributes/respond.md) | Return a partial HTML response (htmx fragment) or status code. | 0.1.0 |
| [`send`](docs/devs/reference/attributes/send.md) | Send an email. | 0.1.0 |
| [`validate`](docs/devs/reference/attributes/validate.md) | Validate input against a model or per-field rules. | 0.1.0 |

### What changed by version

Most recent first. Use this to understand the surface area added in each release.

#### 0.1.3

**Keywords**: [`agent`](docs/devs/reference/keywords/agent.md), [`llm`](docs/devs/reference/keywords/llm.md), [`response`](docs/devs/reference/keywords/response.md)

**Attributes**: [`cwd`](docs/devs/reference/attributes/cwd.md), [`history`](docs/devs/reference/attributes/history.md), [`max-budget-usd`](docs/devs/reference/attributes/max-budget-usd.md), [`max-tokens`](docs/devs/reference/attributes/max-tokens.md), [`max-turns`](docs/devs/reference/attributes/max-turns.md), [`mcp`](docs/devs/reference/attributes/mcp.md), [`permission-mode`](docs/devs/reference/attributes/permission-mode.md), [`pool`](docs/devs/reference/attributes/pool.md), [`pool-idle-ttl`](docs/devs/reference/attributes/pool-idle-ttl.md), [`resume`](docs/devs/reference/attributes/resume.md), [`show-tools`](docs/devs/reference/attributes/show-tools.md), [`stream-swap`](docs/devs/reference/attributes/stream-swap.md), [`system`](docs/devs/reference/attributes/system.md), [`temperature`](docs/devs/reference/attributes/temperature.md), [`tools`](docs/devs/reference/attributes/tools.md), [`workspace-root`](docs/devs/reference/attributes/workspace-root.md)

#### 0.1.0

**Keywords**: [`action`](docs/devs/reference/keywords/action.md), [`api`](docs/devs/reference/keywords/api.md), [`auth`](docs/devs/reference/keywords/auth.md), [`config`](docs/devs/reference/keywords/config.md), [`fragment`](docs/devs/reference/keywords/fragment.md), [`job`](docs/devs/reference/keywords/job.md), [`layout`](docs/devs/reference/keywords/layout.md), [`limit`](docs/devs/reference/keywords/limit.md), [`log`](docs/devs/reference/keywords/log.md), [`model`](docs/devs/reference/keywords/model.md), [`page`](docs/devs/reference/keywords/page.md), [`permissions`](docs/devs/reference/keywords/permissions.md), [`query`](docs/devs/reference/keywords/query.md), [`schedule`](docs/devs/reference/keywords/schedule.md), [`socket`](docs/devs/reference/keywords/socket.md), [`stream`](docs/devs/reference/keywords/stream.md), [`test`](docs/devs/reference/keywords/test.md), [`translations`](docs/devs/reference/keywords/translations.md), [`webhook`](docs/devs/reference/keywords/webhook.md)

**Attributes**: [`after_login`](docs/devs/reference/attributes/after_login.md), [`as`](docs/devs/reference/attributes/as.md), [`auto`](docs/devs/reference/attributes/auto.md), [`auto_update`](docs/devs/reference/attributes/auto_update.md), [`broadcast`](docs/devs/reference/attributes/broadcast.md), [`cors`](docs/devs/reference/attributes/cors.md), [`custom`](docs/devs/reference/attributes/custom.md), [`database`](docs/devs/reference/attributes/database.md), [`default`](docs/devs/reference/attributes/default.md), [`default_language`](docs/devs/reference/attributes/default_language.md), [`delay`](docs/devs/reference/attributes/delay.md), [`detect_language`](docs/devs/reference/attributes/detect_language.md), [`dynamic_fields`](docs/devs/reference/attributes/dynamic_fields.md), [`enqueue`](docs/devs/reference/attributes/enqueue.md), [`errors`](docs/devs/reference/attributes/errors.md), [`event`](docs/devs/reference/attributes/event.md), [`every`](docs/devs/reference/attributes/every.md), [`expect`](docs/devs/reference/attributes/expect.md), [`fetch`](docs/devs/reference/attributes/fetch.md), [`fill`](docs/devs/reference/attributes/fill.md), [`forgot`](docs/devs/reference/attributes/forgot.md), [`generate`](docs/devs/reference/attributes/generate.md), [`html`](docs/devs/reference/attributes/html.md), [`identity`](docs/devs/reference/attributes/identity.md), [`index`](docs/devs/reference/attributes/index.md), [`level`](docs/devs/reference/attributes/level.md), [`login`](docs/devs/reference/attributes/login.md), [`logout`](docs/devs/reference/attributes/logout.md), [`max`](docs/devs/reference/attributes/max.md), [`message`](docs/devs/reference/attributes/message.md), [`method`](docs/devs/reference/attributes/method.md), [`min`](docs/devs/reference/attributes/min.md), [`name`](docs/devs/reference/attributes/name.md), [`on`](docs/devs/reference/attributes/on.md), [`password`](docs/devs/reference/attributes/password.md), [`port`](docs/devs/reference/attributes/port.md), [`redirect`](docs/devs/reference/attributes/redirect.md), [`register`](docs/devs/reference/attributes/register.md), [`requests`](docs/devs/reference/attributes/requests.md), [`required`](docs/devs/reference/attributes/required.md), [`requires`](docs/devs/reference/attributes/requires.md), [`reset`](docs/devs/reference/attributes/reset.md), [`respond`](docs/devs/reference/attributes/respond.md), [`retry`](docs/devs/reference/attributes/retry.md), [`secret`](docs/devs/reference/attributes/secret.md), [`send`](docs/devs/reference/attributes/send.md), [`slow_query`](docs/devs/reference/attributes/slow_query.md), [`static`](docs/devs/reference/attributes/static.md), [`submit`](docs/devs/reference/attributes/submit.md), [`superuser`](docs/devs/reference/attributes/superuser.md), [`table`](docs/devs/reference/attributes/table.md), [`tenant`](docs/devs/reference/attributes/tenant.md), [`title`](docs/devs/reference/attributes/title.md), [`unique`](docs/devs/reference/attributes/unique.md), [`uploads`](docs/devs/reference/attributes/uploads.md), [`validate`](docs/devs/reference/attributes/validate.md), [`visit`](docs/devs/reference/attributes/visit.md)

