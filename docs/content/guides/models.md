# Models

A `model` block defines a data type. From one declaration, Kilnx generates: a database table, server-side validation, HTML form generation, client-side validation attributes, and a listing fragment.

## Basic model

```kilnx
model user
  name: text required min 2 max 100
  email: email unique
  role: option [admin, editor, viewer] default viewer
  active: bool default true
  created: timestamp auto
```

Every model automatically gets an `id` column (INTEGER PRIMARY KEY AUTOINCREMENT in SQLite, BIGSERIAL in PostgreSQL). You do not declare it.

## Field types

See the [complete field types reference](../reference/field-types.html). Summary:

- **Text:** `text`, `email`, `richtext`, `url`, `phone`, `password`, `option`, `tags`
- **Numeric:** `int`, `float`, `decimal`, `bigint`
- **Boolean &amp; time:** `bool`, `timestamp`, `date`
- **Structured:** `json`, `uuid`
- **Files:** `image`, `file`
- **Relations:** `reference` (or use another model name)

## Constraints

| Constraint | Meaning |
|------------|---------|
| `required` | Column is NOT NULL |
| `unique` | Column has UNIQUE index |
| `default <val>` | Default value for INSERT |
| `auto` | Auto-generated on INSERT (for `timestamp`, `date`, `uuid`) |
| `auto_update` | Auto-set on every UPDATE (DB trigger) |
| `min <n>` | Minimum length (text) or value (numeric) |
| `max <n>` | Maximum length (text) or value (numeric) |

```kilnx
model post
  title: text required min 5 max 200
  views: int default 0 min 0
  published_at: timestamp
  updated_at: timestamp auto_update
```

`auto_update` emits a database trigger so the column auto-updates on every UPDATE statement. No application code required.

## Composite unique

For uniqueness that spans two or more columns, declare a model-level `unique (...)` directive:

```kilnx
model membership
  user: user required
  project: project required
  role: option [owner, admin, member] default member
  unique (user, project)
```

Rules:

- At least two fields. Use the field-level `unique` constraint for single-column uniqueness.
- Reference fields resolve to their `<name>_id` column automatically (above: `user_id`, `project_id`).
- Multiple `unique (...)` lines are allowed for independent groups on the same model.

Migration emits `CREATE UNIQUE INDEX IF NOT EXISTS "uq_<table>_<col>_<col>" ON "<table>" (...)`, which is idempotent on SQLite and PostgreSQL. `kilnx check` rejects unknown field names, fields repeated within a group, and duplicated groups.

## Non-unique indexes

For query acceleration without uniqueness, declare an `index (...)` directive. Single-column and multi-column both work:

```kilnx
model order
  customer: customer required
  created: timestamp auto
  status: option [pending, paid, shipped]
  index (customer, created)
  index (status)
```

Migration emits `CREATE INDEX IF NOT EXISTS "ix_<table>_<cols>" ON "<table>" (...)`. The `ix_` prefix keeps non-unique indexes separate from composite UNIQUE constraints (`uq_`). The analyzer applies the same validation rules as `unique (...)`.

## References (foreign keys)

Use another model's name as the field type:

```kilnx
model post
  title: text required
  author: user required
  created: timestamp auto
```

This creates an `author_id` column as a foreign key to `user(id)`. In queries, use `author_id` for the column and `author.name` for JOINed fields.

## Options (enums)

```kilnx
field: option [val1, val2, val3]
```

Kilnx creates a `CHECK` constraint so the database rejects invalid values. HTML form generation produces a `<select>` automatically.

## The `auto` constraint in detail

| Field type | Behavior with `auto` |
|------------|----------------------|
| `timestamp` | Inserts current UTC time on INSERT |
| `date` | Inserts today&apos;s date on INSERT |
| `uuid` | Generates a UUID v4 on INSERT |
| `bool` | Defaults to `false` |

```kilnx
model order
  reference: uuid auto
  placed_at: timestamp auto
  delivery_date: date
  status: option [pending, shipped, delivered] default pending
```

## Multi-tenant scoping

A model can declare that its rows belong to a tenant (another model):

```kilnx
model org
  name: text required unique

model user
  tenant: org
  email: email unique
  password: password required

model quote
  tenant: org
  number: text required unique
```

The compiler auto-synthesizes a required `org_id` foreign key. The runtime rewrites `SELECT` queries to include `WHERE quote.org_id = :current_user.org_id`.

The tenant rewriter **fails closed**: if the SQL shape is too complex to verify safely (CTEs, JOINs, subqueries, UNION, schema-qualified tables, multi-statement queries), the query is refused at runtime rather than passing through unscoped.

## Custom fields

Runtime-extensible fields stored as JSON in a `custom` column:

```kilnx
model deal
  name: text required
  custom fields from "deal_fields.kilnx"
```

See the [grammar reference](../reference/grammar.html#model) for the manifest syntax and per-tenant variants.

## Migrations

Schema changes detected from `model` edits. Applied on `kilnx run` (dev) or via `kilnx migrate` (prod). No manual migration files.

```bash
kilnx migrate app.kilnx --dry-run   # show SQL without applying
kilnx migrate app.kilnx --status    # show applied migrations
kilnx migrate app.kilnx             # apply pending migrations
```

The compiler detects added fields (emits `ALTER TABLE ADD COLUMN`). Removed or renamed fields are not auto-detected — handle those explicitly.
