# Kilnx documentation

Kilnx is a declarative backend language that compiles `.kilnx` source to a single standalone binary. Models, routes, SQL, auth, jobs, WebSockets, SSE, and tests live in one file. SQLite and PostgreSQL are first-class.

- Zero JavaScript, zero runtime dependencies for the user
- ~15MB self-contained binary
- Built for the htmx era

<div class="toc-grid">

<div class="toc-card">

### Start here

- [Install &amp; hello world](getting-started.html)
- [Design principles](principles.html)

</div>

<div class="toc-card">

### Guides

- [Models](guides/models.html)
- [Pages &amp; actions](guides/pages-actions.html)
- [Auth &amp; permissions](guides/auth-permissions.html)
- [Queries](guides/queries.html)
- [Fragments &amp; htmx](guides/fragments-htmx.html)
- [Realtime (SSE &amp; WS)](guides/realtime.html)
- [Jobs &amp; schedules](guides/jobs-schedules.html)
- [Testing](guides/testing.html)
- [Deployment](guides/deployment.html)

</div>

<div class="toc-card">

### Reference

- [Grammar](reference/grammar.html)
- [Field types](reference/field-types.html)
- [Constraints](reference/constraints.html)
- [CLI](reference/cli.html)

</div>

<div class="toc-card">

### Tutorials

- [Build a CRUD app](tutorials/crud-app.html)

</div>

</div>

## Minimal example

```kilnx
config
  database: env DATABASE_URL default "sqlite://app.db"
  port: env PORT default 8080
  secret: env SECRET_KEY required

model user
  name: text required min 2 max 100
  email: email unique
  password: password required
  created: timestamp auto

auth
  table: user
  identity: email
  password: password
  login: /login
  after login: /dashboard

page /dashboard requires auth
  query stats: SELECT count(*) as total FROM user
  html
    <h1>Welcome, {current_user.name}</h1>
    <p>Total users: {stats.total}</p>
```

One file. `kilnx run app.kilnx` starts a dev server with hot reload. `kilnx build` emits a standalone binary.
