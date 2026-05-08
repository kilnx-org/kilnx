# Tutorial 3: Background jobs and schedules

This tutorial picks up the [authenticated Todo app](02-auth-and-sessions.md) and adds an overdue-reminder feature. A background `schedule` runs every minute, finds todos that just went overdue, and `enqueue`s a `job` that sends one email per user. We also mark each todo so we never email twice for the same row.

References used: [`job`](../reference/keywords/job.md), [`schedule`](../reference/keywords/schedule.md), [`enqueue`](../reference/attributes/enqueue.md), [`every`](../reference/attributes/every.md), [`send`](../reference/attributes/send.md), [`retry`](../reference/attributes/retry.md).

## 1. Add a `due_at` and `notified_at` to the model

We already added `due_at` in Tutorial 2. Now add a `notified_at` flag so the schedule can tell which overdue todos have already triggered an email:

```kilnx
model todo
  id: uuid auto
  user_id: reference user required
  title: text required min 1 max 200
  done: bool default false
  due_at: timestamp
  notified_at: timestamp
  created_at: timestamp auto
```

Restart the server. Kilnx adds the `notified_at` column. Existing rows keep `NULL` so they will be evaluated by the schedule on the next tick.

## 2. Let users set a due date

Update the new and edit forms so users can pick a deadline. Only the form changes here, the action SQL already handles the parameter as `:due_at`:

```kilnx
page /todos/new title "New todo" requires auth
  html
    <h1>New todo</h1>
    <form method="post" action="/todos/create">
      <label>Title <input name="title" required></label>
      <label>Due at <input type="datetime-local" name="due_at"></label>
      <button type="submit">Create</button>
      <a href="/todos">Cancel</a>
    </form>

action /todos/create method POST requires auth
  validate todo
  query: INSERT INTO todo (title, user_id, due_at)
         VALUES (:title, :current_user_id, :due_at)
  redirect /todos
```

Mirror the same `due_at` field and SQL update in the edit page and update action.

## 3. The job: send one overdue email

A [`job`](../reference/keywords/job.md) runs out of band, so it can be slow without blocking HTTP responses. Add this near the bottom of `app.kilnx`:

```kilnx
job send-overdue-email
  retry 3
  query todo: SELECT t.id, t.title, t.due_at, u.email
              FROM todo t JOIN user u ON u.id = t.user_id
              WHERE t.id = :todo_id
  send email to :todo.email
    subject: "Your todo is overdue: {todo.title}"
    body: "Your todo \"{todo.title}\" was due at {todo.due_at}."
  query: UPDATE todo SET notified_at = now() WHERE id = :todo_id
```

Things to notice:

- `retry 3` retries up to three times if the email transport fails. See [`retry`](../reference/attributes/retry.md).
- The job receives `:todo_id` as a parameter, fetches the todo with its owner email, sends the message, then stamps `notified_at` so we never re-send.
- `send email to <expression>` followed by indented `subject:` and `body:` is the canonical form. See [`send`](../reference/attributes/send.md) for `template:`, `attach:`, and other sub-fields.

The transport (SMTP host, port, credentials) is configured at runtime via env vars, not in the DSL. Check the runtime config docs for the exact variable names.

## 4. The schedule: tick every minute

A [`schedule`](../reference/keywords/schedule.md) runs at a fixed cadence. Use [`every 1m`](../reference/attributes/every.md) for one minute. The body finds every todo that is due, not done, and not yet notified, then enqueues one job per row:

```kilnx
schedule notify-overdue-todos every 1m
  query overdue: SELECT id FROM todo
                 WHERE done = false
                   AND due_at IS NOT NULL
                   AND due_at <= now()
                   AND notified_at IS NULL
  {#each overdue}
    enqueue send-overdue-email
      todo_id: this.id
  {/each}
```

`enqueue` parameters are written as `name: value` on indented lines. See [`enqueue`](../reference/attributes/enqueue.md).

A few design notes worth understanding before moving on:

- The schedule does not send the email itself. It only enqueues, which keeps the per-tick work bounded and lets the job retry independently.
- Filtering by `notified_at IS NULL` makes the schedule idempotent. If the runtime restarts between two ticks, the next tick will still see un-notified rows and pick them up.
- One job per row, not one job for the whole batch, gives each email its own retry budget.

## 5. Try it locally

1. Start the server with email transport set:

   ```bash
   SUPERUSER_EMAIL=you@example.com \
   SMTP_HOST=localhost SMTP_PORT=1025 \
   kilnx run app.kilnx
   ```

   For local testing, run a fake SMTP catcher like [MailHog](https://github.com/mailhog/MailHog) or [smtp4dev](https://github.com/rnwood/smtp4dev) on port 1025 so emails are captured but never actually delivered.

2. Log in, create a todo with a due date one minute in the past.

3. Wait up to 60 seconds. The next schedule tick fires, the row is selected, the job runs, and an email lands in the SMTP catcher inbox.

4. Refresh `http://localhost:8080/todos`. The row's `notified_at` is now set, so the next tick ignores it. You can confirm in the database:

   ```bash
   sqlite3 adequa.db "SELECT id, title, due_at, notified_at FROM todo;"
   ```

If nothing arrives, run `kilnx run app.kilnx` with the log level turned up so failed jobs surface their error. See [`log`](../reference/keywords/log.md).

## 6. Manual trigger for ad-hoc sends

Sometimes you want to enqueue the job from a button rather than waiting for the schedule. Add an action that any owner can hit on a single todo:

```kilnx
action /todos/:id/remind method POST requires auth
  query owns: SELECT id FROM todo
              WHERE id = :id AND user_id = :current_user_id
  enqueue send-overdue-email
    todo_id: :id
  redirect /todos
```

Wire it into the row fragment if you want a "remind me" button:

```kilnx
<form method="post" action="/todos/{todo.id}/remind" style="display:inline">
  <button type="submit">remind</button>
</form>
```

## 7. Full source for the new pieces

Drop these blocks into `app.kilnx` (next to the existing models and routes from Tutorial 2):

```kilnx
model todo
  id: uuid auto
  user_id: reference user required
  title: text required min 1 max 200
  done: bool default false
  due_at: timestamp
  notified_at: timestamp
  created_at: timestamp auto

job send-overdue-email
  retry 3
  query todo: SELECT t.id, t.title, t.due_at, u.email
              FROM todo t JOIN user u ON u.id = t.user_id
              WHERE t.id = :todo_id
  send email to :todo.email
    subject: "Your todo is overdue: {todo.title}"
    body: "Your todo \"{todo.title}\" was due at {todo.due_at}."
  query: UPDATE todo SET notified_at = now() WHERE id = :todo_id

schedule notify-overdue-todos every 1m
  query overdue: SELECT id FROM todo
                 WHERE done = false
                   AND due_at IS NOT NULL
                   AND due_at <= now()
                   AND notified_at IS NULL
  {#each overdue}
    enqueue send-overdue-email
      todo_id: this.id
  {/each}

action /todos/:id/remind method POST requires auth
  query owns: SELECT id FROM todo
              WHERE id = :id AND user_id = :current_user_id
  enqueue send-overdue-email
    todo_id: :id
  redirect /todos
```

## 8. Things to consider for production

- Replace the per-minute tick with a less aggressive cadence once you have many rows. `every 5m` or a cron expression like `every "*/15 * * * *"` keeps load low.
- For multi-instance deploys, only one Kilnx process should run the schedule. Use a leader lock or run a dedicated worker instance.
- Wrap the SQL update in the same transaction as the email send if your transport supports a sync confirmation. The current shape sends first and updates second, which is the right trade-off when retries are cheap and double-sends are not catastrophic.
- Add an unsubscribe path before you ship reminder emails to real users.

## Wrapping up

You now have:

- a single-file Todo app with htmx ([Tutorial 1](01-todo-app.md)),
- per-user data and role gating ([Tutorial 2](02-auth-and-sessions.md)),
- background reminders that run on a clock and survive restarts (this tutorial).

For everything else, head to the [reference](../reference/index.md).
