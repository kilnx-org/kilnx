# Field types

Field types map a Kilnx declaration to a SQL column. Every field has a type plus zero or more [constraints](constraints.html).

## Text types

| Kilnx type | SQLite | PostgreSQL | Notes |
|------------|--------|------------|-------|
| `text` | TEXT | TEXT | Plain string |
| `email` | TEXT | TEXT | Format-validated on form submission |
| `richtext` | TEXT | TEXT | Rendered unescaped. Use only with trusted input. |
| `password` | TEXT | TEXT | Auto-hashed with bcrypt on INSERT. Never stored plaintext. |
| `option` | TEXT | TEXT | Enum. Syntax: `field: option [a, b, c]`. Emits CHECK constraint. |
| `url` | TEXT | TEXT | Format-validated (requires scheme and host) |
| `phone` | TEXT | TEXT | Format-validated |
| `tags` | TEXT | TEXT | Multi-value. Comma-separated storage. Optional allowed-list: `field: tags [a, b, c]` |
| `uuid` | TEXT | UUID | With `auto`: SQLite uses `randomblob`, PostgreSQL uses `gen_random_uuid()` |

## Numeric types

| Kilnx type | SQLite | PostgreSQL | Notes |
|------------|--------|------------|-------|
| `int` | INTEGER | INTEGER | 32-bit integer |
| `bigint` | INTEGER | BIGINT | 64-bit integer |
| `float` | REAL | DOUBLE PRECISION | Floating point |
| `decimal` | TEXT | NUMERIC | Fixed-point. Use for money and other exact arithmetic. |

## Boolean &amp; time

| Kilnx type | SQLite | PostgreSQL | Notes |
|------------|--------|------------|-------|
| `bool` | INTEGER (0/1) | BOOLEAN | With `auto`: defaults to false |
| `timestamp` | TEXT (ISO8601) | TIMESTAMP | With `auto`: current UTC time on INSERT |
| `date` | TEXT (YYYY-MM-DD) | DATE | With `auto`: today&apos;s date on INSERT |

## Structured data

| Kilnx type | SQLite | PostgreSQL | Notes |
|------------|--------|------------|-------|
| `json` | TEXT | JSONB | Arbitrary JSON. Validated on INSERT. |

## File uploads

| Kilnx type | SQLite | PostgreSQL | Notes |
|------------|--------|------------|-------|
| `image` | TEXT | TEXT | Stores upload path. Generates `<input type="file" accept="image/*">` |
| `file` | TEXT | TEXT | Generic file upload. Stores path. |

Upload handling is configured in `config`:

```kilnx
config
  uploads: ./uploads max 50mb
```

## References

Use another model name as the type:

```kilnx
model post
  author: user required
```

Generates an `author_id` column. SQLite uses `INTEGER`, PostgreSQL uses `BIGINT`. Automatically adds a `REFERENCES user(id)` foreign key constraint.

You can also write `author: reference` explicitly (rarely needed).

## Summary table

Quick reference of all types:

```
Text:       text, email, richtext, password, option,
            url, phone, tags, uuid
Numeric:    int, bigint, float, decimal
Bool/time:  bool, timestamp, date
Structured: json
Files:      image, file
Relations:  reference (or ModelName)
```

Total: 20 field types. See [constraints](constraints.html) for modifiers that apply across types.
