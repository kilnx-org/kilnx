package database

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// PostgresDialect implements Dialect for PostgreSQL via pgx.
type PostgresDialect struct{}

func (PostgresDialect) DriverName() string { return "pgx" }

func (PostgresDialect) DSN(url string) string {
	// pgx accepts standard postgres:// URLs directly
	return url
}

func (PostgresDialect) InitStatements() []string {
	// PostgreSQL has foreign keys enabled by default and WAL is the default
	// journal mode. No init statements needed.
	return nil
}

func (PostgresDialect) Placeholder(index int) string {
	return fmt.Sprintf("$%d", index)
}

func (PostgresDialect) TableExistsSQL() string {
	return `SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = $1`
}

func (PostgresDialect) ListTablesSQL() string {
	return `SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public'
		  AND table_name NOT LIKE '\_kilnx\_%' ESCAPE '\'
		  AND table_name NOT LIKE '\_%\_field\_defs' ESCAPE '\'`
}

func (PostgresDialect) ColumnsSQL(table string) string {
	return fmt.Sprintf(
		`SELECT column_name FROM information_schema.columns
		 WHERE table_schema = 'public' AND table_name = '%s'
		 ORDER BY ordinal_position`, table)
}

func (PostgresDialect) AutoIncrementPK() string {
	return "id BIGSERIAL PRIMARY KEY"
}

func (d PostgresDialect) FieldToSQLType(f parser.Field) string {
	switch f.Type {
	case parser.FieldText, parser.FieldEmail, parser.FieldRichtext,
		parser.FieldPassword, parser.FieldPhone, parser.FieldImage,
		parser.FieldURL, parser.FieldFile,
		parser.FieldTags, parser.FieldOption:
		return "TEXT"
	case parser.FieldBool:
		return "BOOLEAN"
	case parser.FieldTimestamp:
		return "TIMESTAMP"
	case parser.FieldDate:
		return "DATE"
	case parser.FieldInt:
		return "INTEGER"
	case parser.FieldFloat:
		return "DOUBLE PRECISION"
	case parser.FieldDecimal:
		return "NUMERIC"
	case parser.FieldJSON:
		return "JSONB"
	case parser.FieldUUID:
		return "UUID"
	case parser.FieldReference, parser.FieldBigInt:
		return "BIGINT"
	default:
		return "TEXT"
	}
}

func (d PostgresDialect) FieldToDefault(f parser.Field) string {
	if f.Auto && f.Type == parser.FieldTimestamp {
		return " DEFAULT NOW()"
	}
	if f.Auto && f.Type == parser.FieldDate {
		return " DEFAULT CURRENT_DATE"
	}
	if f.Auto && f.Type == parser.FieldUUID {
		return " DEFAULT gen_random_uuid()"
	}
	if f.Auto && f.Type == parser.FieldBool {
		return " DEFAULT FALSE"
	}
	if f.Default != "" {
		switch f.Type {
		case parser.FieldBool:
			if f.Default == "true" {
				return " DEFAULT TRUE"
			}
			return " DEFAULT FALSE"
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

func (PostgresDialect) BoolTrue() string  { return "TRUE" }
func (PostgresDialect) BoolFalse() string { return "FALSE" }

func (PostgresDialect) AutoUpdateTriggerDDL(table, field string) []string {
	fnName := fmt.Sprintf("_kilnx_upd_%s_%s_fn", table, field)
	trigName := fmt.Sprintf("_kilnx_upd_%s_%s", table, field)
	return []string{
		fmt.Sprintf(
			`CREATE OR REPLACE FUNCTION "%s"() RETURNS TRIGGER AS $$ BEGIN NEW."%s" = NOW(); RETURN NEW; END; $$ LANGUAGE plpgsql`,
			fnName, field,
		),
		fmt.Sprintf(
			`CREATE OR REPLACE TRIGGER "%s" BEFORE UPDATE ON "%s" FOR EACH ROW EXECUTE FUNCTION "%s"()`,
			trigName, table, fnName,
		),
	}
}

func (PostgresDialect) InternalTableDDL() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS _kilnx_sessions (
			token TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			identity TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT '',
			data TEXT NOT NULL DEFAULT '{}',
			expires_at TIMESTAMP NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS _kilnx_password_resets (
			token TEXT PRIMARY KEY,
			email TEXT NOT NULL,
			expires_at TIMESTAMP NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS _kilnx_migrations (
			id BIGSERIAL PRIMARY KEY,
			schema_hash TEXT NOT NULL,
			statements TEXT NOT NULL,
			applied_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS _kilnx_jobs (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			params TEXT NOT NULL DEFAULT '{}',
			state TEXT NOT NULL DEFAULT 'available',
			attempts INTEGER NOT NULL DEFAULT 0,
			max_attempts INTEGER NOT NULL DEFAULT 1,
			scheduled_at TIMESTAMP NOT NULL DEFAULT NOW(),
			started_at TIMESTAMP,
			completed_at TIMESTAMP,
			last_error TEXT
		)`,
	}
}
