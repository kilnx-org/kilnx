# Tutorials

End-to-end walkthroughs for building real apps with Kilnx. Each tutorial assumes a working `kilnx` binary and starts from an empty `.kilnx` file. Each tutorial ends with the complete app, and CI compiles it on every commit.

For the full keyword and attribute index, see the [reference](../reference/index.md).

## Tutorials

1. [Build a Todo app](01-todo-app.md). One model, a list with inline htmx toggle and delete, and a reusable row fragment. Covers `model`, `page`, `action`, `fragment`, and `query`.

2. [Auth and sessions](02-auth-and-sessions.md). Add the login, register and password reset pages, per-user data, and role-based access to an existing app. Covers `auth`, `permissions`, and `requires`.

3. [Background jobs and schedules](03-background-jobs.md). Flag overdue todos from a periodic schedule and send reminder emails from a retried background job. Covers `job`, `schedule`, `enqueue`, `every`, and `send email`.

## Conventions

All tutorials use `app.kilnx` as the source filename and run with `kilnx run app.kilnx`. SQLite is the default database, so no extra setup is needed.
