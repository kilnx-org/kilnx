# Jobs &amp; schedules

> This page is a stub. Full content coming soon.

## Jobs (async work)

```kilnx
job send-welcome
  retry 3
  query data: SELECT name, email FROM user WHERE id = :user_id
  send email to :email
    subject: "Welcome {data.name}"
```

Dispatch from an action:

```kilnx
action /users/create method POST
  query: INSERT INTO user (name, email) VALUES (:name, :email)
  enqueue send-welcome
    user_id: :id
  redirect /users
```

Jobs run in the same binary, persisted in `_kilnx_jobs` (SQLite or PostgreSQL). `retry N` configures automatic retries with exponential backoff.

## Schedules (cron)

```kilnx
schedule cleanup every 24h
  query: DELETE FROM session WHERE expires_at < datetime('now')

schedule weekly-report every monday at 9:00
  query stats: SELECT count(*) as new_users FROM user
               WHERE created > datetime('now', '-7 days')
  send email to "admin@example.com"
    subject: "Weekly report: {stats.new_users} new users"
```

Supports intervals (`every 5m`, `every 24h`) and cron-style expressions (`every monday at 9:00`).
