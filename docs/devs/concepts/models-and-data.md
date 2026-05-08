# Models and data

A [`model`](../reference/keywords/model.md) declares a table. The runtime auto-migrates the schema on startup: adding fields, indexes, or unique constraints triggers `ALTER TABLE` automatically.

## Declaring a model

```kilnx
model user
  email: email required unique
  name: text required
  role: option [admin, editor, viewer] default "viewer"
  age: int min 18 max 120
  created_at: timestamp auto
```

Each field follows the shape `name: <type> [constraint...]`. The type is required. Constraints are space-separated.

## Field types

Built-in types include:

- `text`, `email`, `password`: strings, with format validation on `email` and bcrypt hashing on `password`.
- `int`, `float`: numerics, support `min` and `max`.
- `bool`: boolean, often paired with `default false` or `default true`.
- `timestamp`, `date`: time values, support `auto` and `auto_update`.
- `uuid`: UUID v4, usually with `auto`.
- `image`, `file`: uploads, see [`uploads`](../reference/attributes/uploads.md) under `config`.
- `json`: structured data stored as JSON.
- `reference <model>`: foreign key to another model.
- `option [a, b, c]`: enum-like, restricted to the listed values.
- `tags`: list of string tags.

## Constraints

The most common constraints, all documented in the reference:

- [`required`](../reference/attributes/required.md): `NOT NULL` plus runtime validation.
- [`unique`](../reference/attributes/unique.md): single-field unique constraint.
- [`default <value>`](../reference/attributes/default.md): database-level default.
- [`min`](../reference/attributes/min.md) and [`max`](../reference/attributes/max.md): numeric range or string length.
- [`auto`](../reference/attributes/auto.md): auto-fill on insert. UUIDs get a v4, timestamps get the current time, integer IDs auto-increment.
- [`auto_update`](../reference/attributes/auto_update.md): auto-fill on insert and update. Standard for `updated_at: timestamp auto_update`.
- [`tenant: <model>`](../reference/attributes/tenant.md): scope all rows to a tenant model. Auto-synthesizes the FK and filters queries by the current tenant.

For composite indexes and uniqueness:

```kilnx
model invoice
  tenant: account
  amount: float required
  status: option [draft, sent, paid] default "draft"
  index (tenant, status)
```

See the [model reference](../reference/keywords/model.md) for the full list.

## Migrations

Migrations are not separate files. The runtime diffs the declared schema against the live database on startup and applies the changes. Adding a field, an index, or a unique constraint is automatic. Renames and destructive changes are not inferred; for those, write the SQL yourself or add the new field and migrate data with a `query` directive.

There is no `db migrate` command. `kilnx run` and the built binary both migrate on start.

## Database backends

The database is selected by `DATABASE_URL`. The default is `sqlite://app.db`, a file in the working directory.

```bash
DATABASE_URL=sqlite:///var/data/app.db kilnx run app.kilnx
```

For Postgres:

```bash
DATABASE_URL=postgres://user:pass@host:5432/dbname kilnx run app.kilnx
```

You can also pin the URL declaratively inside a [`config`](../reference/keywords/config.md) block, which keeps the deployment uniform across environments.

## Querying models

Queries are inline SQL. The analyzer validates column references against the model definitions, so a typo in a `SELECT` is a compile error, not a runtime error.

```kilnx
page /tasks
  query tasks: SELECT id, title, done FROM task ORDER BY created DESC paginate 20
  html
    {{each tasks}}
    <li>{title}</li>
    {{end}}
```

`paginate <n>` adds offset/limit handling and exposes pagination metadata to the template. The optimizer rewrites `SELECT *` to only the columns your template actually reads, so wide tables stay fast without manual column lists.

For one-shot named queries reused across pages, declare a top-level [`query`](../reference/keywords/query.md) block.

## Inserts and updates

Writes go inside an [`action`](../reference/keywords/action.md) (form posts, deletes, updates):

```kilnx
action /tasks/new method POST
  validate task
  query: INSERT INTO task (title) VALUES (:title)
  redirect /tasks
```

`validate task` runs the model's constraints (required, min, max, unique) against the form input before the insert.

Bind parameters with `:name`. The runtime does parameter binding, never string interpolation, so SQL injection is not a concern.

## Read next

- [Pages, actions, fragments](pages-actions-fragments.md): how the HTTP layer uses these models.
- [Auth and permissions](auth-and-permissions.md): the `auth` block sits on top of a `user` model.
