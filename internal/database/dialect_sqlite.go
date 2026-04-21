package database

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// SqliteDialect implements Dialect for SQLite via modernc.org/sqlite.
type SqliteDialect struct{}

func (SqliteDialect) DriverName() string { return "sqlite" }

func (SqliteDialect) DSN(url string) string {
	dsn := strings.TrimPrefix(url, "sqlite://")
	dsn = strings.TrimPrefix(dsn, "file:")
	return dsn
}

func (SqliteDialect) InitStatements() []string {
	return []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
	}
}

func (SqliteDialect) Placeholder(_ int) string { return "?" }

func (SqliteDialect) TableExistsSQL() string {
	return "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?"
}

func (SqliteDialect) ColumnsSQL(table string) string {
	return fmt.Sprintf(`SELECT name FROM pragma_table_info("%s")`, table)
}

func (SqliteDialect) AutoIncrementPK() string {
	return "id INTEGER PRIMARY KEY AUTOINCREMENT"
}

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

func (SqliteDialect) BoolTrue() string  { return "1" }
func (SqliteDialect) BoolFalse() string { return "0" }

func (SqliteDialect) AutoUpdateTriggerDDL(table, field string) []string {
	return []string{
		fmt.Sprintf(
			`CREATE TRIGGER IF NOT EXISTS "_kilnx_upd_%s_%s" AFTER UPDATE ON "%s" BEGIN UPDATE "%s" SET "%s" = CURRENT_TIMESTAMP WHERE id = NEW.id; END`,
			table, field, table, table, field,
		),
	}
}

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
	}
}
