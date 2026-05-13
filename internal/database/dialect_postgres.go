package database

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// PostgresDialect implements Dialect for PostgreSQL via pgx.
type PostgresDialect struct{}

// DriverName returns "pgx".
func (PostgresDialect) DriverName() string { return "pgx" }

// DSN passes the URL through unchanged; pgx accepts postgres:// URLs directly.
func (PostgresDialect) DSN(url string) string {
	// pgx accepts standard postgres:// URLs directly
	return url
}

// InitStatements returns nil; PostgreSQL needs no per-connection setup.
func (PostgresDialect) InitStatements() []string {
	// PostgreSQL has foreign keys enabled by default and WAL is the default
	// journal mode. No init statements needed.
	return nil
}

// Placeholder returns "$N" for the given 1-based index.
func (PostgresDialect) Placeholder(index int) string {
	return fmt.Sprintf("$%d", index)
}

// TableExistsSQL returns a query that counts matching rows in
// information_schema.tables for the public schema.
func (PostgresDialect) TableExistsSQL() string {
	return `SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = $1`
}

// ListTablesSQL returns a query for user tables in the public schema,
// excluding kilnx-internal tables.
func (PostgresDialect) ListTablesSQL() string {
	return `SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public'
		  AND table_name NOT LIKE '\_kilnx\_%' ESCAPE '\'
		  AND table_name NOT LIKE '\_%\_field\_defs' ESCAPE '\'`
}

// ColumnsSQL returns a query for the column names of the given public-schema
// table, ordered by ordinal position. The table name is interpolated literally,
// so callers must validate it.
func (PostgresDialect) ColumnsSQL(table string) string {
	return fmt.Sprintf(
		`SELECT column_name FROM information_schema.columns
		 WHERE table_schema = 'public' AND table_name = '%s'
		 ORDER BY ordinal_position`, table)
}

// ColumnsInfoSQL returns name, normalized data_type, notnull flag, and a
// 1/0 has_default flag for each column of the given public-schema table.
// data_type follows information_schema conventions (e.g. "integer",
// "double precision", "timestamp without time zone"); callers normalize.
func (PostgresDialect) ColumnsInfoSQL(table string) string {
	return fmt.Sprintf(
		`SELECT column_name,
		        data_type,
		        CASE WHEN is_nullable = 'NO' THEN 1 ELSE 0 END,
		        CASE WHEN column_default IS NULL THEN 0 ELSE 1 END
		 FROM information_schema.columns
		 WHERE table_schema = 'public' AND table_name = '%s'
		 ORDER BY ordinal_position`, table)
}

// UniqueColumnsSQL returns the names of columns covered by a single-column
// UNIQUE constraint on the given public-schema table. Composite UNIQUE
// constraints are excluded via the HAVING COUNT(*) = 1 group filter.
func (PostgresDialect) UniqueColumnsSQL(table string) string {
	return fmt.Sprintf(`
		SELECT MIN(kcu.column_name)
		FROM information_schema.table_constraints AS tc
		JOIN information_schema.key_column_usage AS kcu
		  ON tc.constraint_name = kcu.constraint_name
		 AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'UNIQUE'
		  AND tc.table_schema = 'public'
		  AND tc.table_name = '%s'
		GROUP BY tc.constraint_name
		HAVING COUNT(*) = 1
	`, table)
}

// AutoIncrementPK returns the PostgreSQL BIGSERIAL primary key definition.
func (PostgresDialect) AutoIncrementPK() string {
	return "id BIGSERIAL PRIMARY KEY"
}

// FieldToSQLType maps a parser.Field type to a PostgreSQL column type.
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

// FieldToDefault returns a " DEFAULT ..." clause for the field, or "" if none.
// Auto fields use NOW(), CURRENT_DATE, gen_random_uuid(), or FALSE depending on type.
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

// BoolTrue returns the SQL literal "TRUE".
func (PostgresDialect) BoolTrue() string { return "TRUE" }

// BoolFalse returns the SQL literal "FALSE".
func (PostgresDialect) BoolFalse() string { return "FALSE" }

// AutoUpdateTriggerDDL returns a CREATE FUNCTION + CREATE TRIGGER pair that sets
// the given field to NOW() before each UPDATE on the given table.
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

// InternalTableDDL returns the CREATE TABLE IF NOT EXISTS statements for
// kilnx-managed tables (sessions, password resets, migrations, jobs, flags,
// rate limits) using PostgreSQL types.
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
		`CREATE TABLE IF NOT EXISTS _kilnx_flags (
			name TEXT PRIMARY KEY,
			enabled BOOLEAN NOT NULL DEFAULT FALSE
		)`,
		`CREATE TABLE IF NOT EXISTS _kilnx_rate_limits (
			scope_key TEXT NOT NULL,
			limit_period TEXT NOT NULL,
			window_start BIGINT NOT NULL,
			request_count INTEGER NOT NULL DEFAULT 1,
			PRIMARY KEY (scope_key, limit_period, window_start)
		)`,
	}
}
