package database

import "github.com/kilnx-org/kilnx/internal/parser"

// Dialect abstracts database-specific SQL generation and introspection.
// Each supported database (SQLite, PostgreSQL) implements this interface.
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

	// ColumnsSQL returns a query that yields (column_name TEXT) rows for a table.
	// Receives the table name as a literal (not a parameter) for PRAGMA compat.
	ColumnsSQL(table string) string

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
}
