# Concepts

These pages explain how Kilnx is meant to be used, not just what each keyword does. If you want a syntax lookup, jump to the [reference](../reference/index.md).

Read them in order the first time. Each page assumes the previous ones.

## The pages

- [Mental model](mental-model.md): what kind of language Kilnx is, how `kilnx run` and `kilnx build` differ, and why files are indent-significant.
- [Models and data](models-and-data.md): declaring tables with `model`, field types and constraints, automatic migrations, SQLite versus Postgres.
- [Pages, actions, fragments](pages-actions-fragments.md): the three building blocks of the HTTP layer, and where `api` fits.
- [Auth and permissions](auth-and-permissions.md): the `auth` block, sessions, CSRF, and role-based rules with `permissions`.
- [Jobs and schedules](jobs-and-schedules.md): asynchronous work with `job` plus `enqueue`, recurring tasks with `schedule`, inbound events with `webhook`.

## Reading path by goal

If you are evaluating Kilnx, read the [mental model](mental-model.md) first. It is the shortest answer to "what is this".

If you are porting an existing CRUD app, [models and data](models-and-data.md) plus [pages, actions, fragments](pages-actions-fragments.md) cover most of what you need.

If you need login and roles, go straight to [auth and permissions](auth-and-permissions.md). It assumes you have a `user` model.

If your app needs background work or third-party integrations, [jobs and schedules](jobs-and-schedules.md) covers `job`, `schedule`, and `webhook`.
