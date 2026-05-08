# Reference

Generated from `internal/spec/entities.go`. Do not edit by hand.

## Keywords

| Name | Summary |
|------|---------|
| [`action`](keywords/action.md) | Declare a state-changing endpoint (POST/PUT/DELETE). |
| [`api`](keywords/api.md) | Declare a JSON endpoint. |
| [`auth`](keywords/auth.md) | Configure email/password authentication. |
| [`config`](keywords/config.md) | Application-wide configuration. |
| [`fragment`](keywords/fragment.md) | Return partial HTML (no document wrapper) for htmx or includes. |
| [`job`](keywords/job.md) | Asynchronous background task triggered by `enqueue`. |
| [`layout`](keywords/layout.md) | Define a reusable HTML wrapper for pages. |
| [`limit`](keywords/limit.md) | Rate-limit requests matching a path pattern. |
| [`log`](keywords/log.md) | Configure runtime logging output. |
| [`model`](keywords/model.md) | Declare a database table with typed fields and constraints. |
| [`page`](keywords/page.md) | Define an HTTP route and its view. |
| [`permissions`](keywords/permissions.md) | Role-based access control rules. |
| [`query`](keywords/query.md) | Run a SQL query and bind its result. Used both top-level (named query) and inside bodies. |
| [`schedule`](keywords/schedule.md) | Background task executed on a fixed interval or cron expression. |
| [`socket`](keywords/socket.md) | WebSocket endpoint for bidirectional real-time messaging. |
| [`stream`](keywords/stream.md) | Server-Sent Events (SSE) endpoint that pushes data on an interval. |
| [`test`](keywords/test.md) | End-to-end browser-style test scenario. |
| [`translations`](keywords/translations.md) | i18n translation strings keyed by language and key. |
| [`webhook`](keywords/webhook.md) | Receive external events (Stripe, GitHub, etc.) at a path. |


## Attributes

| Name | Summary | Used in |
|------|---------|---------|
| [`after_login`](attributes/after_login.md) | Default redirect target after successful login. | `auth` |
| [`as`](attributes/as.md) | Run subsequent steps as a given role or user. | `test` |
| [`auto`](attributes/auto.md) | Auto-populate the field on insert. | `model` |
| [`auto_update`](attributes/auto_update.md) | Auto-populate on insert and update. | `model` |
| [`broadcast`](attributes/broadcast.md) | Push a message to all clients in a websocket room. | `page`, `action`, `fragment`, `api`, `schedule`, `job`, `webhook`, `socket` |
| [`cors`](attributes/cors.md) | Comma-separated list of allowed CORS origins. | `config` |
| [`custom`](attributes/custom.md) | Load custom field definitions from an external manifest. | `model` |
| [`database`](attributes/database.md) | Database connection URL. | `config` |
| [`default`](attributes/default.md) | Default value when no value is supplied. | `model` |
| [`default_language`](attributes/default_language.md) | Fallback language code for translations. | `config` |
| [`delay`](attributes/delay.md) | Seconds to delay over-budget requests before responding. | `limit` |
| [`detect_language`](attributes/detect_language.md) | How to detect the user's language at request time. | `config` |
| [`dynamic_fields`](attributes/dynamic_fields.md) | Allow fields to be defined at runtime via the database. | `model` |
| [`enqueue`](attributes/enqueue.md) | Enqueue an asynchronous `job` with named parameters. | `page`, `action`, `fragment`, `api`, `schedule`, `job`, `webhook` |
| [`errors`](attributes/errors.md) | Error reporting strategy. Append `stacktrace` to include stacks. | `log` |
| [`event`](attributes/event.md) | SSE event name to emit on each tick. | `stream` |
| [`every`](attributes/every.md) | Cadence at which the parent task runs. | `stream`, `schedule` |
| [`expect`](attributes/expect.md) | Assert a condition on the current state. | `test` |
| [`fetch`](attributes/fetch.md) | Make an HTTP request to an external API. | `page`, `action`, `fragment`, `api`, `schedule`, `job`, `webhook` |
| [`fill`](attributes/fill.md) | Fill a form field with a value. | `test` |
| [`forgot`](attributes/forgot.md) | Path for the forgot-password flow. | `auth` |
| [`generate`](attributes/generate.md) | Generate a PDF from a template and a data query. | `page`, `action`, `fragment`, `api`, `schedule`, `job`, `webhook` |
| [`html`](attributes/html.md) | Render an inline HTML template block. | `page`, `action`, `fragment`, `api`, `schedule`, `job`, `webhook`, `layout` |
| [`identity`](attributes/identity.md) | Field used as the unique login identifier. | `auth` |
| [`index`](attributes/index.md) | Non-unique composite index for query performance. | `model` |
| [`level`](attributes/level.md) | Minimum severity to log (debug, info, warn, error). | `log` |
| [`llm`](attributes/llm.md) | Call a Large Language Model with optional history and system prompt. | `page`, `action`, `fragment`, `api`, `schedule`, `job`, `webhook` |
| [`login`](attributes/login.md) | Path served as the login form. | `auth` |
| [`logout`](attributes/logout.md) | Path that terminates the session. | `auth` |
| [`max`](attributes/max.md) | Maximum value (numeric) or length (string). | `model` |
| [`message`](attributes/message.md) | Custom error message returned when the limit is exceeded. | `limit` |
| [`method`](attributes/method.md) | Restrict the HTTP method for the parent route. | `page`, `action`, `api` |
| [`min`](attributes/min.md) | Minimum value (numeric) or length (string). | `model` |
| [`name`](attributes/name.md) | Human-readable application name. | `config` |
| [`on`](attributes/on.md) | Conditional or event handler. | `page`, `action`, `fragment`, `api`, `schedule`, `job`, `webhook`, `socket` |
| [`password`](attributes/password.md) | Field storing the bcrypt password hash. | `auth` |
| [`port`](attributes/port.md) | TCP port to listen on. | `config` |
| [`redirect`](attributes/redirect.md) | Redirect to another URL. | `page`, `action` |
| [`register`](attributes/register.md) | Path served as the registration form. | `auth` |
| [`requests`](attributes/requests.md) | Rate-limit budget (in `limit`) or request log strategy (in `log`). | `limit`, `log` |
| [`required`](attributes/required.md) | Field must have a non-null value. | `model` |
| [`requires`](attributes/requires.md) | Require authentication or a specific role/permission. | `page`, `action`, `api` |
| [`reset`](attributes/reset.md) | Path for the password-reset confirmation page. | `auth` |
| [`respond`](attributes/respond.md) | Return a partial HTML response (htmx fragment) or status code. | `page`, `action`, `fragment`, `api`, `schedule`, `job`, `webhook` |
| [`retry`](attributes/retry.md) | Maximum retry attempts before the job is marked failed. | `job` |
| [`secret`](attributes/secret.md) | Secret key for session cookies and CSRF tokens. | `config` |
| [`send`](attributes/send.md) | Send an email. | `page`, `action`, `fragment`, `api`, `schedule`, `job`, `webhook` |
| [`slow_query`](attributes/slow_query.md) | Threshold above which a SQL query is logged as slow. | `log` |
| [`static`](attributes/static.md) | Filesystem path served as static assets. | `config` |
| [`submit`](attributes/submit.md) | Submit the current form. | `test` |
| [`superuser`](attributes/superuser.md) | Identity that bypasses role checks. | `auth` |
| [`table`](attributes/table.md) | Name of the model storing user accounts. | `auth` |
| [`tenant`](attributes/tenant.md) | Scope all rows of this model to a tenant. | `model` |
| [`title`](attributes/title.md) | Set the HTML document title. | `page` |
| [`unique`](attributes/unique.md) | Field value must be unique across all rows. | `model` |
| [`uploads`](attributes/uploads.md) | Filesystem path and max size (MB) for user uploads. | `config` |
| [`validate`](attributes/validate.md) | Validate input against a model or per-field rules. | `page`, `action`, `fragment`, `api`, `schedule`, `job`, `webhook` |
| [`visit`](attributes/visit.md) | Navigate to a URL. | `test` |

