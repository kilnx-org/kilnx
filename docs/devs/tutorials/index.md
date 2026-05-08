# Tutorials

End-to-end walkthroughs for building real apps with Kilnx. Each tutorial assumes a working `kilnx` binary and starts from an empty `.kilnx` file. Code blocks copy and paste cleanly.

For the full keyword and attribute index, see the [reference](../reference/index.md).

## Tutorials

1. [Build a Todo app](01-todo-app.md). One model, three pages, three actions, one inline htmx fragment. Covers `model`, `page`, `action`, `fragment`, and `query`.

2. [Auth and sessions](02-auth-and-sessions.md). Add login, register, logout, and role-based access to an existing app. Covers `auth`, `permissions`, and `requires`.

3. [Background jobs and schedules](03-background-jobs.md). Send overdue-todo emails from a periodic schedule that enqueues a job. Covers `job`, `schedule`, `enqueue`, `every`, and `send email`.

## Conventions

All tutorials use `app.kilnx` as the source filename and run with `kilnx run app.kilnx`. SQLite is the default database, so no extra setup is needed.
