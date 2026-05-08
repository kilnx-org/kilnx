package database

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// SqliteDialect implements Dialect for SQLite via modernc.org/sqlite.
type SqliteDialect struct{}

// DriverName returns "sqlite".
func (SqliteDialect) DriverName() string { return "sqlite" }

// DSN strips the "sqlite://" or "file:" prefix from url to get the file path.
func (SqliteDialect) DSN(url string) string {
	dsn := strings.TrimPrefix(url, "sqlite://")
	dsn = strings.TrimPrefix(dsn, "file:")
	return dsn
}

// InitStatements returns PRAGMA statements that enable WAL journaling and
// foreign keys; both are run on every new connection.
func (SqliteDialect) InitStatements() []string {
	return []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
	}
}

// Placeholder always returns "?" for SQLite.
func (SqliteDialect) Placeholder(_ int) string { return "?" }

// TableExistsSQL returns a query that counts entries in sqlite_master
// matching the given table name.
func (SqliteDialect) TableExistsSQL() string {
	return "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?"
}

// ListTablesSQL returns a query for user tables, excluding kilnx-internal
// tables and SQLite's own metadata tables.
func (SqliteDialect) ListTablesSQL() string {
	return `SELECT name FROM sqlite_master
		WHERE type='table'
		  AND name NOT LIKE '\_kilnx\_%' ESCAPE '\'
		  AND name NOT LIKE '\_%\_field\_defs' ESCAPE '\'
		  AND name NOT LIKE 'sqlite\_%' ESCAPE '\'`
}

// ColumnsSQL returns a query for column names of the given table via
// pragma_table_info. The table name is interpolated literally; callers must
// validate it.
func (SqliteDialect) ColumnsSQL(table string) string {
	return fmt.Sprintf(`SELECT name FROM pragma_table_info("%s")`, table)
}

// AutoIncrementPK returns the SQLite INTEGER PRIMARY KEY AUTOINCREMENT
// definition.
func (SqliteDialect) AutoIncrementPK() string {
	return "id INTEGER PRIMARY KEY AUTOINCREMENT"
}

// FieldToSQLType maps a parser.Field type to a SQLite column type.
func (d SqliteDialect) FieldToSQLType(f parser.Field) string {
	switch f.Type {
	case parser.FieldText, parser.FieldEmail, parser.FieldRichtext,
		parser.FieldPassword, parser.FieldPhone, parser.FieldImage,
		parser.FieldURL, parser.FieldDecimal, parser.FieldFile,
		parser.FieldTags, parser.FieldJSON, parser.FieldUUID,
		parser.FieldOption, parser.FieldDate:
		return "TEXT"
	case parser.FieldBool:
		return "INTEGER"
	case parser.FieldTimestamp:
		return "DATETIME"
	case parser.FieldInt:
		return "INTEGER"
	case parser.FieldFloat:
		return "REAL"
	case parser.FieldReference, parser.FieldBigInt:
		return "INTEGER"
	default:
		return "TEXT"
	}
}

// FieldToDefault returns a " DEFAULT ..." clause for the field, or "" if none.
// Auto fields use CURRENT_TIMESTAMP, date('now'), a synthesized UUID expression,
// or 0, depending on type.
func (d SqliteDialect) FieldToDefault(f parser.Field) string {
	if f.Auto && f.Type == parser.FieldTimestamp {
		return " DEFAULT CURRENT_TIMESTAMP"
	}
	if f.Auto && f.Type == parser.FieldDate {
		return " DEFAULT (date('now'))"
	}
	if f.Auto && f.Type == parser.FieldUUID {
		return ` DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6))))`
	}
	if f.Auto && f.Type == parser.FieldBool {
		return " DEFAULT 0"
	}
	if f.Default != "" {
		switch f.Type {
		case parser.FieldBool:
			if f.Default == "true" {
				return " DEFAULT 1"
			}
			return " DEFAULT 0"
		case parser.FieldInt, parser.FieldFloat:
			if _, err := strconv.ParseFloat(f.Default, 64); err != nil {
				return ""
			}
			return fmt.Sprintf(" DEFAULT %s", f.Default)
		default:
			escaped := strings.ReplaceAll(f.Default, "'", "''")
			return fmt.Sprintf(" DEFAULT '%s'", escaped)
		}
	}
	return ""
}

// BoolTrue returns the SQL literal "1".
func (SqliteDialect) BoolTrue() string { return "1" }

// BoolFalse returns the SQL literal "0".
func (SqliteDialect) BoolFalse() string { return "0" }

// AutoUpdateTriggerDDL returns a CREATE TRIGGER IF NOT EXISTS statement that
// updates the given field to CURRENT_TIMESTAMP after each UPDATE on the table.
func (SqliteDialect) AutoUpdateTriggerDDL(table, field string) []string {
	return []string{
		fmt.Sprintf(
			`CREATE TRIGGER IF NOT EXISTS "_kilnx_upd_%s_%s" AFTER UPDATE ON "%s" BEGIN UPDATE "%s" SET "%s" = CURRENT_TIMESTAMP WHERE id = NEW.id; END`,
			table, field, table, table, field,
		),
	}
}

// InternalTableDDL returns the CREATE TABLE IF NOT EXISTS statements for
// kilnx-managed tables (sessions, password resets, migrations, jobs, flags,
// rate limits) using SQLite types.
func (SqliteDialect) InternalTableDDL() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS _kilnx_sessions (
			token TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			identity TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT '',
			data TEXT NOT NULL DEFAULT '{}',
			expires_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS _kilnx_password_resets (
			token TEXT PRIMARY KEY,
			email TEXT NOT NULL,
			expires_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS _kilnx_migrations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			schema_hash TEXT NOT NULL,
			statements TEXT NOT NULL,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS _kilnx_jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			params TEXT NOT NULL DEFAULT '{}',
			state TEXT NOT NULL DEFAULT 'available',
			attempts INTEGER NOT NULL DEFAULT 0,
			max_attempts INTEGER NOT NULL DEFAULT 1,
			scheduled_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			started_at DATETIME,
			completed_at DATETIME,
			last_error TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS _kilnx_flags (
			name TEXT PRIMARY KEY,
			enabled BOOLEAN NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS _kilnx_rate_limits (
			scope_key TEXT NOT NULL,
			limit_period TEXT NOT NULL,
			window_start INTEGER NOT NULL,
			request_count INTEGER NOT NULL DEFAULT 1,
			PRIMARY KEY (scope_key, limit_period, window_start)
		)`,
	}
}
