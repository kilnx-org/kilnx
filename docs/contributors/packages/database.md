# `internal/database`

> Package database provides the persistence layer for the Kilnx runtime: connection management, schema migration, and dialect-aware query execution against SQLite or PostgreSQL.

| | |
|---|---|
| **Import path** | `github.com/kilnx-org/kilnx/internal/database` |
| **Source last touched** | `69981b8` (2026-05-13) |
| **Doc last touched** | `44eea80` (2026-05-13) |


## Overview

The package is organized around three concerns:

  - Driver (driver.go, driver_postgres.go): opens connections from a
    DATABASE_URL and exposes a *sql.DB.
  - Dialect (dialect.go, dialect_sqlite.go, dialect_postgres.go):
    emits dialect-specific SQL for CREATE TABLE, ALTER, parameter
    binding, and identifier quoting.
  - Migration (database.go): derives the target schema from the
    parser AST, diffs it against the live schema, and applies the
    necessary CREATE/ALTER statements transactionally. Migration
    history is recorded in a kilnx-managed metadata table.
    Read-only drift detection (DetectColumnDrift) reports columns
    whose live schema diverges from the model along orphan, type,
    NOT NULL, UNIQUE, and DEFAULT-presence axes. Drift is advisory:
    PlanMigration is intentionally additive and never emits ALTER
    or DROP COLUMN to reconcile.

Query execution helpers live in query.go. Public types in
interfaces.go define what the runtime depends on so tests can
substitute fakes.

## Files

| File | Summary |
|------|---------|
| [`database.go`](../../../internal/database/database.go) | _no file-level doc_ |
| [`dialect.go`](../../../internal/database/dialect.go) | _no file-level doc_ |
| [`dialect_postgres.go`](../../../internal/database/dialect_postgres.go) | _no file-level doc_ |
| [`dialect_sqlite.go`](../../../internal/database/dialect_sqlite.go) | _no file-level doc_ |
| [`driver.go`](../../../internal/database/driver.go) | _no file-level doc_ |
| [`driver_postgres.go`](../../../internal/database/driver_postgres.go) | _no file-level doc_ |
| [`interfaces.go`](../../../internal/database/interfaces.go) | _no file-level doc_ |
| [`query.go`](../../../internal/database/query.go) | _no file-level doc_ |

## Types

### `ColumnDrift`

```go
type ColumnDrift struct {
	Kind		ColumnDriftKind
	TableName	string
	Column		string
	Detail		string	// human-readable, e.g. "model=TEXT db=INTEGER"
}
```

ColumnDrift describes one divergence between a declared model field and
the live database schema. PlanMigration is intentionally additive (model
-> DB) and never emits DROP COLUMN or ALTER COLUMN, so these warnings are
the only signal that the schemas have diverged. `kilnx migrate` reports
them as warnings; the migration itself is not blocked.

### `ColumnDriftKind`

```go
type ColumnDriftKind string
```

ColumnDriftKind classifies how a database column diverges from its model
declaration. Drift detection is read-only; PlanMigration never emits ALTER
COLUMN or DROP COLUMN automatically, so this is purely advisory.

### `DB`

```go
type DB struct {
	conn		*sql.DB
	dialect		Dialect
	OnSlowQuery	func(sql string, d time.Duration)	// optional callback for slow query logging
}
```

DB is a thin wrapper around *sql.DB that selects between SQLite and Postgres
at runtime via a Dialect. It also exposes named-parameter execution helpers
and an optional slow-query callback.

### `DataLossRisk`

```go
type DataLossRisk struct {
	TableName	string
	RowCount	int
	Suggestion	string	// human-readable hint, e.g. an ALTER TABLE RENAME statement
}
```

DataLossRisk describes a table in the database that is no longer represented
by any model in the current schema. If it contains rows, the migration may
leave data behind.

### `Dialect`

```go
type Dialect interface {
	// DriverName returns the Go sql.Open driver name (e.g. "sqlite", "pgx").
	DriverName() string

	// DSN transforms a user-provided URL into a driver-specific DSN.
	// For SQLite: strips "sqlite://" prefix.
	// For PostgreSQL: passes through as-is.
	DSN(url string) string

	// InitStatements returns SQL statements to run immediately after connecting.
	// SQLite: PRAGMA journal_mode=WAL, PRAGMA foreign_keys=ON.
	// PostgreSQL: (none needed).
	InitStatements() []string

	// Placeholder returns the parameter placeholder for the given 1-based index.
	// SQLite: always "?".
	// PostgreSQL: "$1", "$2", etc.
	Placeholder(index int) string

	// TableExistsSQL returns a query that yields a count > 0 if the table exists.
	// The single ? / $1 parameter is the table name.
	TableExistsSQL() string

	// ListTablesSQL returns a query that yields (name TEXT) rows for all user
	// tables in the database. Internal tables (starting with _kilnx_ and
	// _<model>_field_defs) must be excluded by the query itself.
	ListTablesSQL() string

	// ColumnsSQL returns a query that yields (column_name TEXT) rows for a table.
	// Receives the table name as a literal (not a parameter) for PRAGMA compat.
	ColumnsSQL(table string) string

	// ColumnsInfoSQL returns a query that yields (name TEXT, type TEXT,
	// notnull INT, has_default INT) rows for a table. Used by drift
	// detection to compare DB columns against model field declarations
	// across type, NOT NULL, and default-presence dimensions.
	ColumnsInfoSQL(table string) string

	// UniqueColumnsSQL returns a query that yields (column_name TEXT) rows
	// for each column that has a single-column UNIQUE index/constraint on
	// the given table. Composite unique constraints are excluded.
	UniqueColumnsSQL(table string) string

	// AutoIncrementPK returns the PRIMARY KEY column definition for an
	// auto-incrementing integer id.
	// SQLite: "id INTEGER PRIMARY KEY AUTOINCREMENT"
	// PostgreSQL: "id BIGSERIAL PRIMARY KEY"
	AutoIncrementPK() string

	// FieldToSQLType maps a Kilnx field type to a SQL column type.
	FieldToSQLType(f parser.Field) string

	// FieldToDefault returns the DEFAULT clause for a field.
	FieldToDefault(f parser.Field) string

	// BoolTrue returns the SQL literal for true (1 for SQLite, TRUE for PG).
	BoolTrue() string

	// BoolFalse returns the SQL literal for false (0 for SQLite, FALSE for PG).
	BoolFalse() string

	// InternalTableDDL returns CREATE TABLE IF NOT EXISTS statements
	// for Kilnx internal tables (sessions, password_resets, migrations, jobs).
	InternalTableDDL() []string

	// AutoUpdateTriggerDDL returns statements that create a trigger to
	// automatically update the given field to the current timestamp on UPDATE.
	AutoUpdateTriggerDDL(table, field string) []string
}
```

Dialect abstracts database-specific SQL generation and introspection.
Each supported database (SQLite, PostgreSQL) implements this interface.

### `Executor`

```go
type Executor interface {
	QueryRows(sqlStr string) ([]Row, error)
	QueryRowsWithParams(sqlStr string, params map[string]string) ([]Row, error)
	ExecWithParams(sqlStr string, params map[string]string) error
	BeginTxHandle() (*TxHandle, error)
	Dialect() Dialect
}
```

Executor is the interface used by the runtime to execute SQL.
It allows the runtime to work with a real database or a test fake.

### `MigrationRecord`

```go
type MigrationRecord struct {
	ID		int
	SchemaHash	string
	Statements	string
	AppliedAt	string
}
```

MigrationRecord represents a recorded migration entry

### `PostgresDialect`

```go
type PostgresDialect struct{}
```

PostgresDialect implements Dialect for PostgreSQL via pgx.

### `Row`

```go
type Row map[string]string
```

Row is a single result row as a map of column name to string value.
All values are stringified via fmt for uniform consumption by the runtime;
NULL columns become "".

### `SqliteDialect`

```go
type SqliteDialect struct{}
```

SqliteDialect implements Dialect for SQLite via modernc.org/sqlite.

### `TxHandle`

```go
type TxHandle struct {
	tx		*sql.Tx
	dialect		Dialect
	committed	bool
}
```

TxHandle wraps a *sql.Tx with named-parameter execution and idempotent
commit/rollback. It is created by [DB.BeginTxHandle].

### `columnInfo`

```go
type columnInfo struct {
	Type		string
	NotNull		bool
	HasDefault	bool
}
```

columnInfo captures the per-column data returned by Dialect.ColumnsInfoSQL.

### `querier`

```go
type querier interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
}
```

querier abstracts *sql.DB and *sql.Tx for shared query logic

## Functions

### `DetectDialect`

```go
func DetectDialect(url string) Dialect
```

DetectDialect returns the appropriate Dialect for a database URL.
URLs starting with "postgres://" or "postgresql://" use PostgreSQL.
Everything else defaults to SQLite.

### `ExecWithParamsTx`

```go
func ExecWithParamsTx(tx *sql.Tx, dialect Dialect, sqlStr string, params map[string]string) error
```

ExecWithParamsTx executes a mutation within a transaction

### `IsValidIdentifier`

```go
func IsValidIdentifier(name string) bool
```

IsValidIdentifier reports whether name is a safe SQL identifier.

### `Open`

```go
func Open(url string) (*DB, error)
```

Open connects to the database identified by url.
The driver is auto-detected from the URL scheme:
  - postgres://... or postgresql://... -> PostgreSQL (pgx)
  - sqlite://... or file:... or bare path -> SQLite

### `QueryRowsWithParamsTx`

```go
func QueryRowsWithParamsTx(tx *sql.Tx, dialect Dialect, sqlStr string, params map[string]string) ([]Row, error)
```

QueryRowsWithParamsTx executes a SELECT within a transaction

### `RewriteCustomFieldShorthand`

```go
func RewriteCustomFieldShorthand(sqlStr string, isPostgres bool) string
```

RewriteCustomFieldShorthand rewrites the "custom.fieldName" shorthand to the
dialect-appropriate JSON extraction syntax. Only call for models with custom fields.
SQLite: custom.field -> json_extract("custom", '$.field')
PostgreSQL: custom.field -> "custom"->>'field'

### `bindParams`

```go
func bindParams(dialect Dialect, sqlStr string, params map[string]string) (string, []interface{})
```
### `customKindToSQL`

```go
func customKindToSQL(kind parser.CustomFieldKind, isPostgres bool) string
```

customKindToSQL maps a manifest field kind to a SQL column type.

### `fieldDefsTableDDL`

```go
func fieldDefsTableDDL(modelName string, isPostgres bool) string
```

fieldDefsTableDDL returns the CREATE TABLE IF NOT EXISTS for _<model>_field_defs.

### `fieldToColumnName`

```go
func fieldToColumnName(f parser.Field) string
```
### `indexDDLs`

```go
func indexDDLs(model parser.Model) []string
```

indexDDLs emits `CREATE INDEX IF NOT EXISTS` statements for each
non-unique `index (...)` group declared on the model. Index names
are prefixed with `ix_` to stay distinguishable from composite
UNIQUE indexes (`uq_`). Groups referencing unknown fields are
skipped; the analyzer surfaces the error.

### `isValidIdentifier`

```go
func isValidIdentifier(name string) bool
```
### `normalizeSQLType`

```go
func normalizeSQLType(t string) string
```

normalizeSQLType maps a declared or introspected SQL type to a canonical
form for cross-comparison. Strips parenthesized length/precision and
resolves SQLite/PG type aliases (int/integer, bool/boolean, etc.).

### `queryRowsInternal`

```go
func queryRowsInternal(q querier, query string, args ...interface{}) ([]Row, error)
```
### `scanRows`

```go
func scanRows(rows *sql.Rows) ([]Row, error)
```
### `schemaHash`

```go
func schemaHash(models []parser.Model) string
```
### `typesEqual`

```go
func typesEqual(a, b string) bool
```

typesEqual returns true when two SQL type strings are equivalent after
normalization (lowercase, whitespace collapsed, common dialect aliases
resolved, parameterized lengths stripped). Conservative: when in doubt
it returns true to avoid false-positive drift warnings.

### `uniqueIndexDDLs`

```go
func uniqueIndexDDLs(model parser.Model) []string
```

uniqueIndexDDLs emits `CREATE UNIQUE INDEX IF NOT EXISTS` statements for
each composite UNIQUE group declared on the model. Field names are
resolved to their DB column names via fieldToColumnName (so references
map to their `<name>_id` columns). Groups that reference unknown fields
are skipped, the analyzer surfaces the error.

### `(DB) BeginTxHandle`

```go
func (db *DB) BeginTxHandle() (*TxHandle, error)
```

BeginTxHandle starts a new transaction wrapped in a TxHandle

### `(DB) Close`

```go
func (db *DB) Close() error
```

Close closes the underlying *sql.DB connection pool.

### `(DB) Conn`

```go
func (db *DB) Conn() *sql.DB
```

Conn returns the underlying *sql.DB for callers that need direct access
(e.g. to start raw transactions or run driver-specific queries).

### `(DB) DetectColumnDrift`

```go
func (db *DB) DetectColumnDrift(models []parser.Model, manifests ...map[string]*parser.CustomFieldManifest) ([]ColumnDrift, error)
```

DetectColumnDrift introspects each declared model's table and returns
every divergence between the live schema and the model declaration:
orphan columns, type mismatches, NOT NULL mismatches, single-column
UNIQUE mismatches, and DEFAULT presence/absence mismatches. Tables
missing from the database are skipped (planExistingTable adds them as
part of normal migration); orphan tables are covered by
DetectDataLossRisk. Reserved `id` / `custom` columns and column-mode
custom-field manifest entries are exempted.

### `(DB) DetectDataLossRisk`

```go
func (db *DB) DetectDataLossRisk(models []parser.Model) ([]DataLossRisk, error)
```

DetectDataLossRisk inspects the database for tables that do not correspond
to any model in the provided slice and contain data. It returns a slice of
risks and any error encountered during introspection.

### `(DB) Dialect`

```go
func (db *DB) Dialect() Dialect
```

Dialect returns the dialect used by this database connection.

### `(DB) ExecWithParams`

```go
func (db *DB) ExecWithParams(sqlStr string, params map[string]string) error
```

ExecWithParams executes a SQL statement with named parameters from a map.
Named params like :name, :email are replaced with dialect-specific placeholders.

### `(DB) Migrate`

```go
func (db *DB) Migrate(models []parser.Model, manifests ...map[string]*parser.CustomFieldManifest) ([]string, error)
```

Migrate compares models with the current database state and applies changes.
Pass a CustomManifests map as the optional second argument to include
column-mode custom field columns.

### `(DB) MigrateInternal`

```go
func (db *DB) MigrateInternal() error
```

MigrateInternal creates internal kilnx tables for sessions and jobs

### `(DB) MigrationHistory`

```go
func (db *DB) MigrationHistory() ([]MigrationRecord, error)
```

MigrationHistory returns all recorded migrations

### `(DB) PlanMigration`

```go
func (db *DB) PlanMigration(models []parser.Model, manifests ...map[string]*parser.CustomFieldManifest) ([]string, error)
```

PlanMigration compares models with the current database state and returns
the SQL statements that would be executed, without applying them.
Pass a CustomManifests map as the optional second argument to include
column-mode custom field columns in the plan.

### `(DB) QueryRows`

```go
func (db *DB) QueryRows(sqlStr string) ([]Row, error)
```

QueryRows executes a SELECT query and returns all rows as maps

### `(DB) QueryRowsWithParams`

```go
func (db *DB) QueryRowsWithParams(sqlStr string, params map[string]string) ([]Row, error)
```

QueryRowsWithParams executes a SELECT with named params

### `(DB) fieldToColumnDef`

```go
func (db *DB) fieldToColumnDef(f parser.Field) string
```
### `(DB) generateCreateTable`

```go
func (db *DB) generateCreateTable(model parser.Model, cm map[string]*parser.CustomFieldManifest) string
```
### `(DB) getColumns`

```go
func (db *DB) getColumns(table string) (map[string]bool, error)
```
### `(DB) getColumnsInfo`

```go
func (db *DB) getColumnsInfo(table string) (map[string]columnInfo, error)
```
### `(DB) getUniqueColumns`

```go
func (db *DB) getUniqueColumns(table string) (map[string]bool, error)
```
### `(DB) planExistingTable`

```go
func (db *DB) planExistingTable(model parser.Model, cm map[string]*parser.CustomFieldManifest) ([]string, error)
```
### `(DB) recordMigration`

```go
func (db *DB) recordMigration(hash, stmts string)
```
### `(DB) tableExists`

```go
func (db *DB) tableExists(name string) (bool, error)
```
### `(PostgresDialect) AutoIncrementPK`

```go
func (PostgresDialect) AutoIncrementPK() string
```

AutoIncrementPK returns the PostgreSQL BIGSERIAL primary key definition.

### `(PostgresDialect) AutoUpdateTriggerDDL`

```go
func (PostgresDialect) AutoUpdateTriggerDDL(table, field string) []string
```

AutoUpdateTriggerDDL returns a CREATE FUNCTION + CREATE TRIGGER pair that sets
the given field to NOW() before each UPDATE on the given table.

### `(PostgresDialect) BoolFalse`

```go
func (PostgresDialect) BoolFalse() string
```

BoolFalse returns the SQL literal "FALSE".

### `(PostgresDialect) BoolTrue`

```go
func (PostgresDialect) BoolTrue() string
```

BoolTrue returns the SQL literal "TRUE".

### `(PostgresDialect) ColumnsInfoSQL`

```go
func (PostgresDialect) ColumnsInfoSQL(table string) string
```

ColumnsInfoSQL returns name, normalized data_type, notnull flag, and a
1/0 has_default flag for each column of the given public-schema table.
data_type follows information_schema conventions (e.g. "integer",
"double precision", "timestamp without time zone"); callers normalize.

### `(PostgresDialect) ColumnsSQL`

```go
func (PostgresDialect) ColumnsSQL(table string) string
```

ColumnsSQL returns a query for the column names of the given public-schema
table, ordered by ordinal position. The table name is interpolated literally,
so callers must validate it.

### `(PostgresDialect) DSN`

```go
func (PostgresDialect) DSN(url string) string
```

DSN passes the URL through unchanged; pgx accepts postgres:// URLs directly.

### `(PostgresDialect) DriverName`

```go
func (PostgresDialect) DriverName() string
```

DriverName returns "pgx".

### `(PostgresDialect) FieldToDefault`

```go
func (d PostgresDialect) FieldToDefault(f parser.Field) string
```

FieldToDefault returns a " DEFAULT ..." clause for the field, or "" if none.
Auto fields use NOW(), CURRENT_DATE, gen_random_uuid(), or FALSE depending on type.

### `(PostgresDialect) FieldToSQLType`

```go
func (d PostgresDialect) FieldToSQLType(f parser.Field) string
```

FieldToSQLType maps a parser.Field type to a PostgreSQL column type.

### `(PostgresDialect) InitStatements`

```go
func (PostgresDialect) InitStatements() []string
```

InitStatements returns nil; PostgreSQL needs no per-connection setup.

### `(PostgresDialect) InternalTableDDL`

```go
func (PostgresDialect) InternalTableDDL() []string
```

InternalTableDDL returns the CREATE TABLE IF NOT EXISTS statements for
kilnx-managed tables (sessions, password resets, migrations, jobs, flags,
rate limits) using PostgreSQL types.

### `(PostgresDialect) ListTablesSQL`

```go
func (PostgresDialect) ListTablesSQL() string
```

ListTablesSQL returns a query for user tables in the public schema,
excluding kilnx-internal tables.

### `(PostgresDialect) Placeholder`

```go
func (PostgresDialect) Placeholder(index int) string
```

Placeholder returns "$N" for the given 1-based index.

### `(PostgresDialect) TableExistsSQL`

```go
func (PostgresDialect) TableExistsSQL() string
```

TableExistsSQL returns a query that counts matching rows in
information_schema.tables for the public schema.

### `(PostgresDialect) UniqueColumnsSQL`

```go
func (PostgresDialect) UniqueColumnsSQL(table string) string
```

UniqueColumnsSQL returns the names of columns covered by a single-column
UNIQUE constraint on the given public-schema table. Composite UNIQUE
constraints are excluded via the HAVING COUNT(*) = 1 group filter.

### `(SqliteDialect) AutoIncrementPK`

```go
func (SqliteDialect) AutoIncrementPK() string
```

AutoIncrementPK returns the SQLite INTEGER PRIMARY KEY AUTOINCREMENT
definition.

### `(SqliteDialect) AutoUpdateTriggerDDL`

```go
func (SqliteDialect) AutoUpdateTriggerDDL(table, field string) []string
```

AutoUpdateTriggerDDL returns a CREATE TRIGGER IF NOT EXISTS statement that
updates the given field to CURRENT_TIMESTAMP after each UPDATE on the table.

### `(SqliteDialect) BoolFalse`

```go
func (SqliteDialect) BoolFalse() string
```

BoolFalse returns the SQL literal "0".

### `(SqliteDialect) BoolTrue`

```go
func (SqliteDialect) BoolTrue() string
```

BoolTrue returns the SQL literal "1".

### `(SqliteDialect) ColumnsInfoSQL`

```go
func (SqliteDialect) ColumnsInfoSQL(table string) string
```

ColumnsInfoSQL returns name, type, notnull flag, and a 1/0 has_default
flag (derived from dflt_value IS NOT NULL) via pragma_table_info.

### `(SqliteDialect) ColumnsSQL`

```go
func (SqliteDialect) ColumnsSQL(table string) string
```

ColumnsSQL returns a query for column names of the given table via
pragma_table_info. The table name is interpolated literally; callers must
validate it.

### `(SqliteDialect) DSN`

```go
func (SqliteDialect) DSN(url string) string
```

DSN strips the "sqlite://" or "file:" prefix from url to get the file path.

### `(SqliteDialect) DriverName`

```go
func (SqliteDialect) DriverName() string
```

DriverName returns "sqlite".

### `(SqliteDialect) FieldToDefault`

```go
func (d SqliteDialect) FieldToDefault(f parser.Field) string
```

FieldToDefault returns a " DEFAULT ..." clause for the field, or "" if none.
Auto fields use CURRENT_TIMESTAMP, date('now'), a synthesized UUID expression,
or 0, depending on type.

### `(SqliteDialect) FieldToSQLType`

```go
func (d SqliteDialect) FieldToSQLType(f parser.Field) string
```

FieldToSQLType maps a parser.Field type to a SQLite column type.

### `(SqliteDialect) InitStatements`

```go
func (SqliteDialect) InitStatements() []string
```

InitStatements returns PRAGMA statements that enable WAL journaling and
foreign keys; both are run on every new connection.

### `(SqliteDialect) InternalTableDDL`

```go
func (SqliteDialect) InternalTableDDL() []string
```

InternalTableDDL returns the CREATE TABLE IF NOT EXISTS statements for
kilnx-managed tables (sessions, password resets, migrations, jobs, flags,
rate limits) using SQLite types.

### `(SqliteDialect) ListTablesSQL`

```go
func (SqliteDialect) ListTablesSQL() string
```

ListTablesSQL returns a query for user tables, excluding kilnx-internal
tables and SQLite's own metadata tables.

### `(SqliteDialect) Placeholder`

```go
func (SqliteDialect) Placeholder(_ int) string
```

Placeholder always returns "?" for SQLite.

### `(SqliteDialect) TableExistsSQL`

```go
func (SqliteDialect) TableExistsSQL() string
```

TableExistsSQL returns a query that counts entries in sqlite_master
matching the given table name.

### `(SqliteDialect) UniqueColumnsSQL`

```go
func (SqliteDialect) UniqueColumnsSQL(table string) string
```

UniqueColumnsSQL returns the names of columns that have a single-column
unique index on the given table. Composite unique indexes are skipped.
Includes both `u` (user CREATE UNIQUE INDEX) and `c` (table-level UNIQUE
constraint) origins; skips `pk` (PRIMARY KEY) since id is excluded by
callers.

### `(TxHandle) Commit`

```go
func (th *TxHandle) Commit() error
```

Commit commits the transaction

### `(TxHandle) ExecWithParams`

```go
func (th *TxHandle) ExecWithParams(sqlStr string, params map[string]string) error
```

ExecWithParams executes a mutation within the transaction

### `(TxHandle) QueryRowsWithParams`

```go
func (th *TxHandle) QueryRowsWithParams(sqlStr string, params map[string]string) ([]Row, error)
```

QueryRowsWithParams executes a SELECT within the transaction

### `(TxHandle) Rollback`

```go
func (th *TxHandle) Rollback() error
```

Rollback rolls back the transaction (no-op if already committed)


## Notes

<!-- MANUAL-NOTES START -->
# `internal/database`

Persistence layer: connection management, schema migration, dialect-aware query execution. Backs every SQL operation the runtime performs.

## Purpose

Kilnx apps target either SQLite (default, for dev and small deployments) or PostgreSQL (for production scale). The runtime should not care which one is in use. This package abstracts that choice behind a `Dialect` interface and a `database.DB` wrapper, and owns the migration that maps the parsed model AST to live schema.

## Three-way split

The package is organized around three concerns that match the three groups of files:

1. **Driver**: opens a `*sql.DB` from a `DATABASE_URL`. Files: [`driver.go`](../../../internal/database/driver.go), [`driver_postgres.go`](../../../internal/database/driver_postgres.go). Each file is a single blank-import line that pulls in the underlying driver (`modernc.org/sqlite` for SQLite, `github.com/jackc/pgx/v5/stdlib` for Postgres). Splitting them lets build tags, in principle, exclude one or the other; today both are always linked.

2. **Dialect**: emits SQL that varies between databases. Interface in [`dialect.go`](../../../internal/database/dialect.go), implementations in [`dialect_sqlite.go`](../../../internal/database/dialect_sqlite.go) and [`dialect_postgres.go`](../../../internal/database/dialect_postgres.go).

3. **Migration**: derives the target schema from `parser.Model` values, diffs it against the live schema, and applies the necessary `CREATE`/`ALTER` statements. Lives in [`database.go`](../../../internal/database/database.go).

Query execution helpers live in [`query.go`](../../../internal/database/query.go). Public types in [`interfaces.go`](../../../internal/database/interfaces.go) define what the runtime depends on so tests can substitute fakes.

## File map

- [`database.go`](../../../internal/database/database.go): `DB` struct, `Open`, `DetectDialect`, `BeginTxHandle`, `TxHandle`, `MigrateInternal`, `PlanMigration`, `Migrate`, `MigrationHistory`, `DetectDataLossRisk`, `IsValidIdentifier`, `schemaHash`, `customKindToSQL`, `fieldDefsTableDDL`, index/unique-index DDL builders.
- [`dialect.go`](../../../internal/database/dialect.go): the `Dialect` interface (driver name, DSN, init statements, placeholder syntax, table/column introspection SQL, type mappings, default-value clauses, bool literals, internal table DDL, auto-update trigger DDL).
- [`dialect_sqlite.go`](../../../internal/database/dialect_sqlite.go): SQLite implementation. Uses `?` placeholders, `pragma_table_info` for column introspection, `sqlite_master` for table listing, an `AFTER UPDATE` trigger for `auto_update`, WAL + foreign-keys pragmas on connect.
- [`dialect_postgres.go`](../../../internal/database/dialect_postgres.go): Postgres implementation. Uses `$N` placeholders, `information_schema.tables` and `information_schema.columns` for introspection, a `BEFORE UPDATE` trigger backed by a `plpgsql` function for `auto_update`, no init statements (FKs and WAL are defaults).
- [`driver.go`](../../../internal/database/driver.go), [`driver_postgres.go`](../../../internal/database/driver_postgres.go): blank-import the SQL drivers.
- [`query.go`](../../../internal/database/query.go): `Row` type (`map[string]string`), `QueryRows`, `QueryRowsWithParams`, `ExecWithParams`, transactional variants, `bindParams` (named `:name` params to dialect placeholders), `RewriteCustomFieldShorthand` (`custom.field` to `json_extract` or `->>`), slow-query callback hook.
- [`interfaces.go`](../../../internal/database/interfaces.go): `Executor` interface used by the runtime. `*DB` implements it; tests fake it.
- [`doc.go`](../../../internal/database/doc.go): package doc.

## Public surface

```go
func Open(url string) (*DB, error)
func DetectDialect(url string) Dialect

(db *DB) Close() error
(db *DB) Conn() *sql.DB
(db *DB) Dialect() Dialect

(db *DB) QueryRows(sql string) ([]Row, error)
(db *DB) QueryRowsWithParams(sql string, params map[string]string) ([]Row, error)
(db *DB) ExecWithParams(sql string, params map[string]string) error
(db *DB) BeginTxHandle() (*TxHandle, error)

(db *DB) MigrateInternal() error
(db *DB) Migrate(models []parser.Model, manifests ...map[string]*parser.CustomFieldManifest) ([]string, error)
(db *DB) PlanMigration(models []parser.Model, manifests ...map[string]*parser.CustomFieldManifest) ([]string, error)
(db *DB) MigrationHistory() ([]MigrationRecord, error)
(db *DB) DetectDataLossRisk(models []parser.Model) ([]DataLossRisk, error)

OnSlowQuery func(sql string, d time.Duration)   // optional callback set by the runtime logger
```

`TxHandle` exposes `ExecWithParams`, `QueryRowsWithParams`, `Commit`, `Rollback`. Commit and rollback are idempotent.

## Driver selection

`Open(url)` calls `DetectDialect(url)`:

- `postgres://...` or `postgresql://...` (case-insensitive): `PostgresDialect{}` with driver name `"pgx"`.
- Anything else: `SqliteDialect{}` with driver name `"sqlite"`. The `sqlite://` and `file:` prefixes are stripped from the DSN.

After `sql.Open`, every statement returned by `dialect.InitStatements()` is executed against the new connection. SQLite uses this to enable WAL journaling and foreign keys.

## Dialect abstraction

The interface is intentionally thin. Each method has a single, narrow purpose, so a third dialect (MySQL, e.g.) would be a single new file. The implementations differ in:

| Concern | SQLite | Postgres |
|---|---|---|
| Placeholder | `?` | `$1`, `$2`, ... |
| Auto PK | `INTEGER PRIMARY KEY AUTOINCREMENT` | `BIGSERIAL PRIMARY KEY` |
| Bool storage | `INTEGER` (0/1) | `BOOLEAN` |
| JSON storage | `TEXT` | `JSONB` |
| Timestamps | `DATETIME`, `CURRENT_TIMESTAMP` | `TIMESTAMP`, `NOW()` |
| Auto UUID default | hex-randomblob expression | `gen_random_uuid()` |
| Auto-update trigger | `AFTER UPDATE` SQL trigger | `BEFORE UPDATE` plpgsql trigger |
| Table introspection | `sqlite_master` | `information_schema.tables` |
| Column introspection | `pragma_table_info` | `information_schema.columns` |

`ColumnsSQL(table)` interpolates the table name as a literal (because PRAGMA does not accept parameters). The migration code calls `IsValidIdentifier` on the table name first; callers outside that path must do the same.

## Migration

`Migrate(models)` calls `PlanMigration(models)` and applies each statement via `db.conn.Exec`. The plan, per model:

1. Validate the model name as a SQL identifier. Refuse anything that does not match `^[a-zA-Z_][a-zA-Z0-9_]*$`.
2. Probe `tableExists`. If absent, emit a single `CREATE TABLE` (`generateCreateTable`).
3. If present, list existing columns (`getColumns`) and emit `ALTER TABLE ... ADD COLUMN` for each declared field that is missing. Computed fields are skipped (they are evaluated in the runtime, not stored).
4. If the model uses custom fields (`custom <file>` or `dynamic_fields`), ensure the `custom` column exists (`TEXT` for SQLite, `JSONB` for Postgres) and add real columns for any column-mode fields declared in the manifest.
5. If the model has `dynamic_fields`, emit `CREATE TABLE IF NOT EXISTS _<model>_field_defs` (`fieldDefsTableDDL`).
6. For each `auto_update` field, emit `dialect.AutoUpdateTriggerDDL`.
7. Emit `CREATE UNIQUE INDEX IF NOT EXISTS uq_<model>_<cols>` for each composite UNIQUE group (`uniqueIndexDDLs`).
8. Emit `CREATE INDEX IF NOT EXISTS ix_<model>_<cols>` for each non-unique index group (`indexDDLs`).

Index names have distinct prefixes (`uq_` vs `ix_`) so they are easy to grep. Groups that reference unknown fields are silently skipped, the analyzer surfaces those errors upstream.

After successful application, `recordMigration` inserts a row into `_kilnx_migrations` with a `schemaHash` (SHA-256 of a normalized model description, truncated to 8 bytes hex) and the joined SQL. `MigrationHistory()` reads it back ordered by id.

## Kilnx-managed metadata tables

`MigrateInternal` creates the per-runtime tables that are not derived from user models. Both dialects emit equivalent DDL through `dialect.InternalTableDDL()`:

- `_kilnx_sessions`: token, user_id, identity, role, data (JSON blob), expires_at.
- `_kilnx_password_resets`: token, email, expires_at.
- `_kilnx_migrations`: id, schema_hash, statements, applied_at.
- `_kilnx_jobs`: id, name, params, state, attempts, max_attempts, scheduled_at, started_at, completed_at, last_error.
- `_kilnx_flags`: name, enabled.
- `_kilnx_rate_limits`: scope_key, limit_period, window_start, request_count (composite PK).

All names start with `_kilnx_` so the user-table introspection queries can `LIKE '\_kilnx\_%' ESCAPE '\\'` exclude them. The `_<model>_field_defs` tables follow the same exclusion pattern.

## Named parameter binding

`ExecWithParams` and `QueryRowsWithParams` accept SQL with `:name` placeholders and a `map[string]string`. `bindParams` rewrites each match to `dialect.Placeholder(idx)` and appends the value to a positional args slice. Names not found in the map are left as-is, this lets `:memory:` and similar SQL-keyword colon usages survive untouched.

`Row` is a `map[string]string`. All scanned values are stringified with `fmt.Sprintf("%v", val)`, NULLs become `""`. The runtime is string-first end-to-end (because the `.kilnx` DSL is), so this matches the templating model.

## Custom field shorthand

`RewriteCustomFieldShorthand(sql, isPostgres)` rewrites `custom.fieldName` references to dialect-appropriate JSON extraction:

- SQLite: `json_extract("custom", '$.fieldName')`.
- Postgres: `"custom"->>'fieldName'`.

Only call it for models that have custom fields. The runtime checks `modelHasCustomFields` before invoking it.

## Data-loss detection

`DetectDataLossRisk(models)` lists every user table and reports any that are not represented by a model in the new schema and contain rows. For each risk it suggests an `ALTER TABLE ... RENAME TO ...` if a similarly-named model exists (singular/plural variants). The CLI shows this before applying a destructive migration.

## Slow-query callback

`DB.OnSlowQuery` is a `func(sql string, d time.Duration)`. The runtime wires it to `Logger.LogSlowQuery` from [`internal/runtime/server.go`](../../../internal/runtime/server.go) when `db` is a real `*database.DB`. Tests using a fake `Executor` skip the wiring.

## Gotchas

- **No down migrations**. `Migrate` is forward-only. Removing a field from a model leaves the column in place. Renaming is not detected; you get an add-and-orphan. `DetectDataLossRisk` exists to surface that case.
- **Identifier validation is fail-closed but narrow**. `isValidIdentifier` only accepts `[a-zA-Z_][a-zA-Z0-9_]*`. Quoted identifiers, schema-qualified names, and Unicode are all rejected. The migration relies on this to interpolate names directly into DDL strings.
- **`Row` values are always strings**. There is no typed scan path. If you need typed access, do it at the runtime layer.
- **`ColumnsSQL` interpolates the table name**. The validation contract belongs to the caller. Anything outside `PlanMigration`/`getColumns` that asks for column lists must validate first.
- **Migration history is best-effort**. `recordMigration` ignores the insert error. A migration that succeeds but fails to record itself still leaves the schema in the new state.
- **Postgres `gen_random_uuid()` requires `pgcrypto`**. On a fresh database without the extension, `auto` UUID fields will fail at insert time. Document this for ops, the package does not enable extensions.
- **SQLite UUID default is verbose**. The hex-randomblob expression in `dialect_sqlite.go` is correct UUID v4 shape, just hard to read. Do not "simplify" it without re-checking the variant nibble (`8`/`9`/`a`/`b` at position 13).
- **Schema hash is informational, not authoritative**. The migration applies actual diffs computed from live introspection. Two schemas with the same hash but different live state still get migrated correctly, the hash exists for the audit trail.
- **Internal table names are exclusion-fragile**. The `LIKE '\_kilnx\_%'` patterns assume nobody else creates tables starting with `_kilnx_`. The user-facing analyzer should reject model names with that prefix.

## When to touch this package

- Adding a new field type: extend `parser.FieldType`, then both `FieldToSQLType` and `FieldToDefault` in each dialect.
- Adding a new dialect: implement `Dialect`, register a detection branch in `DetectDialect`, add a blank-import file for the driver.
- Changing migration semantics: edit `PlanMigration` and `database_test.go`. The plan/apply split exists so callers can preview, preserve it.
- Adding kilnx-managed tables: extend `InternalTableDDL` in both dialects.
<!-- MANUAL-NOTES END -->
