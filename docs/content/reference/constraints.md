# Constraints

Constraints modify field behavior. They attach to [field types](field-types.html) in a model block.

## required

Column is NOT NULL. Empty form submissions are rejected with a validation error.

```kilnx
name: text required
```

## unique

Column has a UNIQUE index. Duplicate INSERT/UPDATE is rejected.

```kilnx
email: email unique
slug: text required unique
```

Typical pairing: `required unique` for identity fields like username or slug.

## default &lt;value&gt;

Inserts the given value when the field is unset.

```kilnx
role: option [admin, editor, viewer] default viewer
active: bool default true
views: int default 0
```

Quoted strings are supported for text defaults:

```kilnx
status: text default "pending"
```

## auto

Auto-generates a value on INSERT. Behavior depends on field type:

| Field type | Behavior |
|------------|----------|
| `timestamp` | Current UTC time |
| `date` | Today&apos;s date |
| `uuid` | UUID v4 |
| `bool` | `false` |

```kilnx
model order
  created: timestamp auto
  placed_on: date auto
  reference: uuid auto
```

## auto_update

Emits a database trigger that auto-updates the column on every UPDATE statement. Used for `updated_at` patterns.

```kilnx
model post
  title: text required
  updated: timestamp auto_update
```

SQLite generates an `AFTER UPDATE` trigger. PostgreSQL generates a `BEFORE UPDATE` trigger with a `plpgsql` function. The trigger is created/replaced automatically during migration. Applies to `timestamp` fields; other types will have the trigger defined but may not produce meaningful values.

`auto_update` differs from `auto` — `auto` runs only on INSERT, `auto_update` runs on every UPDATE.

## min &lt;n&gt;

Minimum length (for text types) or minimum value (for numeric types).

```kilnx
name: text required min 2
age: int min 18
```

Form validation rejects values below the minimum. For text, measured in characters.

## max &lt;n&gt;

Maximum length or value.

```kilnx
title: text required max 200
score: int max 100
```

## Composite unique

For uniqueness across two or more fields, declare a model-level `unique (...)` directive instead of marking a single field `unique`:

```kilnx
model membership
  user: user required
  project: project required
  role: option [owner, admin, member] default member
  unique (user, project)
```

Rules:

- At least two fields. Single-column uniqueness uses the field-level `unique` constraint.
- Fields must be declared on the same model. Reference fields resolve to their `<name>_id` column automatically.
- Multiple `unique (...)` lines are allowed for independent composite groups.

Migration emits `CREATE UNIQUE INDEX IF NOT EXISTS "uq_&lt;table&gt;_&lt;col&gt;_&lt;col&gt;" ON "&lt;table&gt;" (...)`, safe to rerun on both SQLite and PostgreSQL.

## Combining constraints

Constraints appear space-separated after the field type:

```kilnx
model user
  email: email required unique
  age: int required min 13 max 120
  bio: text max 500 default ""
  role: option [admin, user] default user required
  created: timestamp auto
  updated: timestamp auto_update
```

Order is not significant.

## Summary

| Constraint | Applies to | Effect |
|------------|------------|--------|
| `required` | All | NOT NULL |
| `unique` | All | UNIQUE index |
| `default <val>` | All | Default value |
| `auto` | `timestamp`, `date`, `uuid`, `bool` | Auto-generate on INSERT |
| `auto_update` | `timestamp` | DB trigger: update on every UPDATE |
| `min <n>` | Text (length) or numeric (value) | Lower bound |
| `max <n>` | Text (length) or numeric (value) | Upper bound |
| `unique (a, b, ...)` | Model-level, 2+ fields | Composite UNIQUE index |

Total: 7 field constraints plus 1 model-level directive.
