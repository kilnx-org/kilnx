# Tutorial 3: Background jobs and schedules

This tutorial picks up the [authenticated Todo app](02-auth-and-sessions.md) and adds overdue reminders. Todos get a due date, a background `schedule` runs every minute and flags the ones that went overdue, and an overdue row offers an "email me a reminder" button that `enqueue`s a `job`. The job sends the email out of band, retries if the mail server is down, and records when the reminder went out.

References used: [`job`](../reference/keywords/job.md), [`schedule`](../reference/keywords/schedule.md), [`enqueue`](../reference/attributes/enqueue.md), [`every`](../reference/attributes/every.md), [`send`](../reference/attributes/send.md), [`retry`](../reference/attributes/retry.md), [`on`](../reference/attributes/on.md).

## What schedules and jobs can do today

Before writing code, it helps to know the shape of these blocks in the current runtime:

- A `schedule` or `job` body is a straight list of steps: `query`, `send email`, `enqueue`, `fetch`, `validate`. There is no loop and no `{{each}}` outside of HTML.
- `enqueue` parameters are literals or `:name` parameters of the surrounding request or job. They cannot read a column from a query result.

Together this means a schedule cannot fan out one job per overdue row. Instead, this tutorial splits the work: the schedule does the bulk bookkeeping in a single SQL statement, and jobs are enqueued per todo from an action, where the todo id is a request parameter.

## 1. Add due dates and bookkeeping columns

Extend the `todo` model:

```kilnx
model todo
  title: text required min 1 max 200
  done: bool default false
  owner: user required
  due_on: date
  overdue_at: timestamp
  reminded_at: timestamp
  created_at: timestamp auto
```

- `due_on` is the deadline, a plain date.
- `overdue_at` is set by the schedule the first time it sees the todo past its due date.
- `reminded_at` is set by the job after the reminder email goes out.

The server reloads and adds the three columns. Existing rows keep `NULL`, which means "no due date".

## 2. Let users set a due date

Add a date input to the new and edit forms:

```kilnx
page /todos/new title "New todo" requires auth
  html
    <h1>New todo</h1>
    <form method="POST" action="/todos/create">
      <input type="hidden" name="_csrf" value="{csrf}">
      <label>Title <input name="title" required></label>
      <label>Due on <input name="due_on" type="date"></label>
      <button type="submit">Create</button>
      <a href="/todos">Cancel</a>
    </form>

action /todos/create requires auth
  validate todo
  query: INSERT INTO todo (title, owner_id, due_on)
         VALUES (:title, :current_user.id, NULLIF(:due_on, ''))
  redirect /todos
```

An empty date input still posts `due_on=`, so `NULLIF(:due_on, '')` stores `NULL` instead of an empty string.

The edit page gets the same input, prefilled with `value="{todo.due_on}"`, and its query selects `due_on` too. The update action also clears the bookkeeping columns, so moving a deadline makes the todo eligible for a fresh overdue flag and reminder:

```kilnx
action /todos/:id/update requires auth
  validate todo
  query: UPDATE todo SET title = :title, due_on = NULLIF(:due_on, ''),
         overdue_at = NULL, reminded_at = NULL
         WHERE id = :id AND owner_id = :current_user.id
  redirect /todos
```

## 3. The schedule: flag overdue todos every minute

A [`schedule`](../reference/keywords/schedule.md) runs its body at a fixed cadence. [`every 1m`](../reference/attributes/every.md) means once a minute:

```kilnx
schedule flag-overdue every 1m
  query: UPDATE todo SET overdue_at = datetime('now')
         WHERE done = 0 AND overdue_at IS NULL
         AND due_on IS NOT NULL AND due_on < date('now', 'localtime')
```

Design notes:

- One `UPDATE` handles every overdue row at once, so the work per tick stays a single statement no matter how many todos exist.
- `overdue_at IS NULL` makes it idempotent. A restart between ticks loses nothing: the next tick picks up whatever is still unflagged.
- A todo is overdue the day after its due date. `date('now', 'localtime')` and `datetime('now')` are SQLite functions; on Postgres use `CURRENT_DATE` and `now()`.
- The first tick happens one interval after the server starts, not immediately.

## 4. Show the overdue state

Extend the row component with the due date and an overdue badge. The badge comes with a small form that posts to the remind action you will write in step 6:

```kilnx
fragment todo_row(id, label, done, due_on, overdue)
  html
    <li id="todo-{id}" class="todo">
      <input type="checkbox"
             hx-post="/todos/{id}/toggle"
             hx-vals='{"_csrf":"{csrf}"}'
             hx-target="#todo-{id}"
             hx-swap="outerHTML"
             {{if done}}checked{{end}}>
      <span class="{{if done}}done{{end}}">{label}</span>
      {{if due_on}}<small>due {due_on}</small>{{end}}
      {{if overdue}}
        <strong class="overdue">overdue</strong>
        <form method="POST" action="/todos/{id}/remind" style="display:inline">
          <input type="hidden" name="_csrf" value="{csrf}">
          <button type="submit">email me a reminder</button>
        </form>
      {{end}}
      <a href="/todos/{id}/edit">edit</a>
      <button hx-post="/todos/{id}/delete"
              hx-vals='{"_csrf":"{csrf}"}'
              hx-target="#todo-{id}"
              hx-swap="outerHTML">delete</button>
    </li>
```

The list page and the row fragment compute `overdue` in SQL, so a todo that gets checked off stops showing the badge right away, even though `overdue_at` stays set:

```kilnx
page /todos title "Todos" requires auth
  query todos: SELECT id, title, done, due_on,
               CASE WHEN overdue_at IS NOT NULL AND done = 0 THEN 1 ELSE 0 END AS overdue
               FROM todo
               WHERE owner_id = :current_user.id
               ORDER BY created_at DESC, id DESC
  html
    <h1>Todos</h1>
    <p><a href="/todos/new">New todo</a></p>
    <ul id="todo-list">
      {{each todos}}
        {{todo_row id=id label=title done=done due_on=due_on overdue=overdue}}
      {{else}}
        <li>Nothing to do yet.</li>
      {{end}}
    </ul>

fragment /todos/:id/row requires auth
  query todo: SELECT id, title, done, due_on,
              CASE WHEN overdue_at IS NOT NULL AND done = 0 THEN 1 ELSE 0 END AS overdue
              FROM todo
              WHERE id = :id AND owner_id = :current_user.id
  html
    {{todo_row id=todo.id label=todo.title done=todo.done due_on=todo.due_on overdue=todo.overdue}}
```

## 5. The job: send one reminder email

A [`job`](../reference/keywords/job.md) runs out of band, so it can be slow or fail without blocking an HTTP response. Add this near the bottom of `app.kilnx`:

```kilnx
job send-overdue-email
  retry 3
  query todo: SELECT title, due_on FROM todo
              WHERE id = :todo_id AND owner_id = :owner_id
  send email to query: SELECT u.email FROM todo t JOIN user u ON u.id = t.owner_id WHERE t.id = :todo_id AND t.owner_id = :owner_id
    subject: "Overdue: {todo.title}"
    body: "Your todo {todo.title} was due on {todo.due_on}."
  query: UPDATE todo SET reminded_at = datetime('now')
         WHERE id = :todo_id AND owner_id = :owner_id
```

Things to notice:

- The job receives `todo_id` and `owner_id` as parameters, available to its SQL as `:todo_id` and `:owner_id`.
- `send email to query: ...` takes the recipient from the first column of the first row the query returns. Recipients can also be a parameter (`send email to :email`) or a literal string. See [`send`](../reference/attributes/send.md).
- `subject:` and `body:` can use `{query.column}` placeholders from earlier `query` steps in the same job, here `{todo.title}` and `{todo.due_on}`.
- The steps run in order and stop at the first failure. If the email cannot be sent, the `UPDATE` never runs, so `reminded_at` only gets set for reminders that actually went out.
- `retry 3` means at most three attempts in total. After a failure the job is retried with an exponential backoff (2 seconds, then 4), and after the third failure it is marked `discarded` with the error saved. See [`retry`](../reference/attributes/retry.md).

Jobs live in the `_kilnx_jobs` table, so queued work survives a restart.

The SMTP transport is configured with environment variables, not in the DSL: `KILNX_SMTP_HOST` (default `localhost`), `KILNX_SMTP_PORT` (default `25`), `KILNX_SMTP_USER`, `KILNX_SMTP_PASS`, and `KILNX_SMTP_FROM` (default `noreply@localhost`). Unlike the password reset link from Tutorial 2, `send email` has no console fallback: without a reachable server every attempt fails.

## 6. Enqueue the job from an action

The remind button posts to this action:

```kilnx
action /todos/:id/remind requires auth
  query: SELECT id FROM todo WHERE id = :id AND owner_id = :current_user.id
  on not found
    respond status 404
  enqueue send-overdue-email
    todo_id: :id
    owner_id: :current_user_id
  redirect /todos
```

- The ownership `SELECT` plus [`on not found`](../reference/attributes/on.md) rejects ids that belong to someone else with a `404`, before anything is enqueued.
- `enqueue` parameters are written as `name: value` on indented lines. `:id` comes from the path and `:current_user_id` is the logged-in user's id (the underscore spelling of `current_user.id`). See [`enqueue`](../reference/attributes/enqueue.md).
- The action returns as soon as the job is queued. A background worker checks the queue every second and runs it.

## 7. Try it locally

1. Run a fake SMTP catcher such as [MailHog](https://github.com/mailhog/MailHog) or [smtp4dev](https://github.com/rnwood/smtp4dev) on port 1025, so emails are captured but never delivered.

2. Start the server pointed at it:

   ```bash
   SUPERUSER_EMAIL=you@example.com \
   KILNX_SMTP_HOST=localhost KILNX_SMTP_PORT=1025 \
   KILNX_SMTP_FROM=todos@example.com \
   kilnx run app.kilnx
   ```

   The startup log includes `schedule 'flag-overdue' running every 1m0s` and `Job queue ready (1 job type(s))`.

3. Log in and create a todo due yesterday, and another due tomorrow.

4. Wait up to a minute for the next tick, then refresh `http://localhost:8080/todos`. The first todo shows the overdue badge; the second does not.

5. Click "email me a reminder". The server log shows:

   ```text
   enqueued job 'send-overdue-email'
   running job 'send-overdue-email' (id=1)
   email sent to you@example.com: Overdue: <title>
   job 'send-overdue-email' completed
   ```

   and the message lands in the SMTP catcher inbox.

6. Inspect the bookkeeping:

   ```bash
   sqlite3 app.db "SELECT id, title, due_on, overdue_at, reminded_at FROM todo;"
   sqlite3 app.db "SELECT id, name, state, attempts, last_error FROM _kilnx_jobs;"
   ```

7. Check the box on the overdue todo. The row repaints without the badge.

To watch the retry logic, stop the SMTP catcher and click the reminder button again. The log shows `retry 1/3`, `retry 2/3`, then `discarded after 3 attempts`, and `last_error` holds the connection error.

## 8. Final source

The complete `app.kilnx` after Tutorials 1, 2, and 3:

```kilnx
model user
  email: email required unique
  password: password required
  role: option [admin, member] default "member"
  created_at: timestamp auto

model todo
  title: text required min 1 max 200
  done: bool default false
  owner: user required
  due_on: date
  overdue_at: timestamp
  reminded_at: timestamp
  created_at: timestamp auto

auth
  table: user
  identity: email
  password: password
  login: /login
  logout: /logout
  register: /register
  after login: /todos
  superuser: env SUPERUSER_EMAIL

permissions
  admin: all
  member: read todo, write todo

page /login title "Log in"
  html
    <h1>Log in</h1>
    <p class="error">{params.error}</p>
    <form method="POST" action="/login?next={params.next}">
      <input type="hidden" name="_csrf" value="{csrf}">
      <label>Email <input name="identity" type="email"></label>
      <label>Password <input name="password" type="password"></label>
      <button type="submit">Log in</button>
    </form>
    <p><a href="/register">Create an account</a> . <a href="/forgot-password">Forgot your password?</a></p>

page /register title "Sign up"
  html
    <h1>Sign up</h1>
    <p class="error">{params.error}</p>
    <form method="POST" action="/register">
      <input type="hidden" name="_csrf" value="{csrf}">
      <label>Email <input name="identity" type="email"></label>
      <label>Password <input name="password" type="password"></label>
      <button type="submit">Create account</button>
    </form>

page /forgot-password title "Forgot password"
  html
    <h1>Forgot password</h1>
    {{if params.sent}}<p>If that address has an account, a reset link is on its way.</p>{{end}}
    <p class="error">{params.error}</p>
    <form method="POST" action="/forgot-password">
      <input type="hidden" name="_csrf" value="{csrf}">
      <label>Email <input name="email" type="email"></label>
      <button type="submit">Send reset link</button>
    </form>

page /reset-password title "Reset password"
  html
    <h1>Choose a new password</h1>
    <p class="error">{params.error}</p>
    <form method="POST" action="/reset-password?token={params.token}">
      <input type="hidden" name="_csrf" value="{csrf}">
      <label>New password <input name="password" type="password"></label>
      <label>Repeat it <input name="password_confirm" type="password"></label>
      <button type="submit">Reset password</button>
    </form>

fragment todo_row(id, label, done, due_on, overdue)
  html
    <li id="todo-{id}" class="todo">
      <input type="checkbox"
             hx-post="/todos/{id}/toggle"
             hx-vals='{"_csrf":"{csrf}"}'
             hx-target="#todo-{id}"
             hx-swap="outerHTML"
             {{if done}}checked{{end}}>
      <span class="{{if done}}done{{end}}">{label}</span>
      {{if due_on}}<small>due {due_on}</small>{{end}}
      {{if overdue}}
        <strong class="overdue">overdue</strong>
        <form method="POST" action="/todos/{id}/remind" style="display:inline">
          <input type="hidden" name="_csrf" value="{csrf}">
          <button type="submit">email me a reminder</button>
        </form>
      {{end}}
      <a href="/todos/{id}/edit">edit</a>
      <button hx-post="/todos/{id}/delete"
              hx-vals='{"_csrf":"{csrf}"}'
              hx-target="#todo-{id}"
              hx-swap="outerHTML">delete</button>
    </li>

page /todos title "Todos" requires auth
  query todos: SELECT id, title, done, due_on,
               CASE WHEN overdue_at IS NOT NULL AND done = 0 THEN 1 ELSE 0 END AS overdue
               FROM todo
               WHERE owner_id = :current_user.id
               ORDER BY created_at DESC, id DESC
  html
    <h1>Todos</h1>
    <p><a href="/todos/new">New todo</a></p>
    <ul id="todo-list">
      {{each todos}}
        {{todo_row id=id label=title done=done due_on=due_on overdue=overdue}}
      {{else}}
        <li>Nothing to do yet.</li>
      {{end}}
    </ul>

fragment /todos/:id/row requires auth
  query todo: SELECT id, title, done, due_on,
              CASE WHEN overdue_at IS NOT NULL AND done = 0 THEN 1 ELSE 0 END AS overdue
              FROM todo
              WHERE id = :id AND owner_id = :current_user.id
  html
    {{todo_row id=todo.id label=todo.title done=todo.done due_on=todo.due_on overdue=todo.overdue}}

page /todos/new title "New todo" requires auth
  html
    <h1>New todo</h1>
    <form method="POST" action="/todos/create">
      <input type="hidden" name="_csrf" value="{csrf}">
      <label>Title <input name="title" required></label>
      <label>Due on <input name="due_on" type="date"></label>
      <button type="submit">Create</button>
      <a href="/todos">Cancel</a>
    </form>

action /todos/create requires auth
  validate todo
  query: INSERT INTO todo (title, owner_id, due_on)
         VALUES (:title, :current_user.id, NULLIF(:due_on, ''))
  redirect /todos

page /todos/:id/edit title "Edit todo" requires auth
  query todo: SELECT id, title, due_on FROM todo
              WHERE id = :id AND owner_id = :current_user.id
  html
    <h1>Edit todo</h1>
    <form method="POST" action="/todos/{todo.id}/update">
      <input type="hidden" name="_csrf" value="{csrf}">
      <label>Title <input name="title" value="{todo.title}" required></label>
      <label>Due on <input name="due_on" type="date" value="{todo.due_on}"></label>
      <button type="submit">Save</button>
      <a href="/todos">Cancel</a>
    </form>

action /todos/:id/update requires auth
  validate todo
  query: UPDATE todo SET title = :title, due_on = NULLIF(:due_on, ''),
         overdue_at = NULL, reminded_at = NULL
         WHERE id = :id AND owner_id = :current_user.id
  redirect /todos

action /todos/:id/toggle requires auth
  query: UPDATE todo SET done = NOT done
         WHERE id = :id AND owner_id = :current_user.id
  redirect /todos/:id/row

action /todos/:id/delete requires auth
  query: DELETE FROM todo WHERE id = :id AND owner_id = :current_user.id
  respond fragment delete

page /admin/todos title "All todos" requires admin
  query rows: SELECT t.id, t.title, t.done, u.email
              FROM todo t JOIN user u ON u.id = t.owner_id
              ORDER BY t.created_at DESC
  html
    <h1>All todos</h1>
    <table>
      <thead><tr><th>User</th><th>Title</th><th>Done</th></tr></thead>
      <tbody>
        {{each rows}}
          <tr>
            <td>{email}</td>
            <td>{title}</td>
            <td>{{if done}}yes{{else}}no{{end}}</td>
          </tr>
        {{end}}
      </tbody>
    </table>

schedule flag-overdue every 1m
  query: UPDATE todo SET overdue_at = datetime('now')
         WHERE done = 0 AND overdue_at IS NULL
         AND due_on IS NOT NULL AND due_on < date('now', 'localtime')

job send-overdue-email
  retry 3
  query todo: SELECT title, due_on FROM todo
              WHERE id = :todo_id AND owner_id = :owner_id
  send email to query: SELECT u.email FROM todo t JOIN user u ON u.id = t.owner_id WHERE t.id = :todo_id AND t.owner_id = :owner_id
    subject: "Overdue: {todo.title}"
    body: "Your todo {todo.title} was due on {todo.due_on}."
  query: UPDATE todo SET reminded_at = datetime('now')
         WHERE id = :todo_id AND owner_id = :owner_id

action /todos/:id/remind requires auth
  query: SELECT id FROM todo WHERE id = :id AND owner_id = :current_user.id
  on not found
    respond status 404
  enqueue send-overdue-email
    todo_id: :id
    owner_id: :current_user_id
  redirect /todos
```

`kilnx check app.kilnx` prints `No issues found.`

## 9. Things to consider for production

- The schedule runs in every Kilnx process. For multi-instance deploys that is harmless here, because the `UPDATE` is idempotent, but schedules with side effects should run on a single instance.
- A fully automatic reminder (no button) needs the schedule to enqueue one job per overdue row, which the current schedule body cannot express. Until it can, keep automatic work in set-based SQL, as `flag-overdue` does, and trigger per-row jobs from actions.
- Cadences can also be longer durations such as `every 5m` or `every 1h`. See [`every`](../reference/attributes/every.md).
- Add an unsubscribe path before you send reminder emails to real users.

## Wrapping up

You now have:

- a single-file Todo app with htmx ([Tutorial 1](01-todo-app.md)),
- per-user data and role gating ([Tutorial 2](02-auth-and-sessions.md)),
- a clock-driven schedule and a retrying background job that survive restarts (this tutorial).

For everything else, head to the [reference](../reference/index.md).
