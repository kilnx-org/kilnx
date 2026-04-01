package database

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/kilnx-org/kilnx/internal/parser"
)

var identifierRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

type DB struct {
	conn        *sql.DB
	path        string
	OnSlowQuery func(sql string, d time.Duration) // optional callback for slow query logging
}

// TxHandle wraps a sql.Tx with named parameter support
type TxHandle struct {
	tx        *sql.Tx
	committed bool
}

// BeginTxHandle starts a new transaction wrapped in a TxHandle
func (db *DB) BeginTxHandle() (*TxHandle, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return nil, err
	}
	return &TxHandle{tx: tx}, nil
}

// ExecWithParams executes a mutation within the transaction
func (th *TxHandle) ExecWithParams(sqlStr string, params map[string]string) error {
	return ExecWithParamsTx(th.tx, sqlStr, params)
}

// QueryRowsWithParams executes a SELECT within the transaction
func (th *TxHandle) QueryRowsWithParams(sqlStr string, params map[string]string) ([]Row, error) {
	return QueryRowsWithParamsTx(th.tx, sqlStr, params)
}

// Commit commits the transaction
func (th *TxHandle) Commit() error {
	if th.committed {
		return nil
	}
	th.committed = true
	return th.tx.Commit()
}

// Rollback rolls back the transaction (no-op if already committed)
func (th *TxHandle) Rollback() error {
	if th.committed {
		return nil
	}
	return th.tx.Rollback()
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database %s: %w", path, err)
	}

	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}

	if _, err := conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}

	return &DB{conn: conn, path: path}, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) Conn() *sql.DB {
	return db.conn
}

// PlanMigration compares models with the current database state and returns
// the SQL statements that would be executed, without applying them.
func (db *DB) PlanMigration(models []parser.Model) ([]string, error) {
	var stmts []string

	for _, model := range models {
		if !isValidIdentifier(model.Name) {
			return stmts, fmt.Errorf("invalid model name: %q", model.Name)
		}

		exists, err := db.tableExists(model.Name)
		if err != nil {
			return stmts, err
		}

		if !exists {
			stmts = append(stmts, generateCreateTable(model))
		} else {
			alterStmts, err := db.planExistingTable(model)
			if err != nil {
				return stmts, err
			}
			stmts = append(stmts, alterStmts...)
		}
	}

	return stmts, nil
}

// Migrate compares models with the current database state and applies changes.
func (db *DB) Migrate(models []parser.Model) ([]string, error) {
	stmts, err := db.PlanMigration(models)
	if err != nil {
		return nil, err
	}

	var executed []string
	for _, stmt := range stmts {
		if _, err := db.conn.Exec(stmt); err != nil {
			return executed, fmt.Errorf("executing migration: %w\nSQL: %s", err, stmt)
		}
		executed = append(executed, stmt)
	}

	// Record migration if any statements were executed
	if len(executed) > 0 {
		hash := schemaHash(models)
		allSQL := strings.Join(executed, ";\n")
		db.recordMigration(hash, allSQL)
	}

	return executed, nil
}

// MigrationRecord represents a recorded migration entry
type MigrationRecord struct {
	ID         int
	SchemaHash string
	Statements string
	AppliedAt  string
}

// MigrationHistory returns all recorded migrations
func (db *DB) MigrationHistory() ([]MigrationRecord, error) {
	// Check if table exists first
	exists, err := db.tableExists("_kilnx_migrations")
	if err != nil || !exists {
		return nil, err
	}

	rows, err := db.conn.Query("SELECT id, schema_hash, statements, applied_at FROM _kilnx_migrations ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []MigrationRecord
	for rows.Next() {
		var r MigrationRecord
		if err := rows.Scan(&r.ID, &r.SchemaHash, &r.Statements, &r.AppliedAt); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (db *DB) recordMigration(hash, stmts string) {
	db.conn.Exec(
		"INSERT INTO _kilnx_migrations (schema_hash, statements) VALUES (?, ?)",
		hash, stmts,
	)
}

func schemaHash(models []parser.Model) string {
	var b strings.Builder
	for _, m := range models {
		fmt.Fprintf(&b, "model:%s\n", m.Name)
		for _, f := range m.Fields {
			fmt.Fprintf(&b, "  %s:%s", f.Name, f.Type)
			if f.Required {
				b.WriteString(":req")
			}
			if f.Unique {
				b.WriteString(":uniq")
			}
			if f.Default != "" {
				fmt.Fprintf(&b, ":def=%s", f.Default)
			}
			b.WriteString("\n")
		}
	}
	h := sha256.Sum256([]byte(b.String()))
	return fmt.Sprintf("%x", h[:8])
}

func (db *DB) tableExists(name string) (bool, error) {
	var count int
	err := db.conn.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name,
	).Scan(&count)
	return count > 0, err
}

// planExistingTable returns ALTER TABLE statements needed without executing them
func (db *DB) planExistingTable(model parser.Model) ([]string, error) {
	var stmts []string

	existing, err := db.getColumns(model.Name)
	if err != nil {
		return nil, err
	}

	for _, field := range model.Fields {
		colName := fieldToColumnName(field)
		if !isValidIdentifier(colName) {
			return stmts, fmt.Errorf("invalid column name: %q", colName)
		}
		if _, ok := existing[colName]; ok {
			continue
		}

		sqlType := fieldToSQLType(field)
		defaultClause := fieldToDefault(field)

		stmt := fmt.Sprintf("ALTER TABLE \"%s\" ADD COLUMN \"%s\" %s%s",
			model.Name, colName, sqlType, defaultClause)
		stmts = append(stmts, stmt)
	}

	return stmts, nil
}

func (db *DB) getColumns(table string) (map[string]bool, error) {
	if !isValidIdentifier(table) {
		return nil, fmt.Errorf("invalid table name: %q", table)
	}

	rows, err := db.conn.Query(fmt.Sprintf("PRAGMA table_info(\"%s\")", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, typeName string
		var notNull, pk int
		var dflt *string
		if err := rows.Scan(&cid, &name, &typeName, &notNull, &dflt, &pk); err != nil {
			return nil, err
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return columns, nil
}

func generateCreateTable(model parser.Model) string {
	var cols []string

	cols = append(cols, "id INTEGER PRIMARY KEY AUTOINCREMENT")

	for _, field := range model.Fields {
		col := fieldToColumnDef(field)
		cols = append(cols, col)
	}

	return fmt.Sprintf("CREATE TABLE \"%s\" (\n  %s\n)", model.Name, strings.Join(cols, ",\n  "))
}

func fieldToColumnDef(f parser.Field) string {
	name := fieldToColumnName(f)
	sqlType := fieldToSQLType(f)

	var parts []string
	parts = append(parts, fmt.Sprintf("\"%s\"", name), sqlType)

	if f.Required {
		parts = append(parts, "NOT NULL")
	}

	if f.Unique {
		parts = append(parts, "UNIQUE")
	}

	def := fieldToDefault(f)
	if def != "" {
		parts = append(parts, strings.TrimSpace(def))
	}

	if f.Type == parser.FieldOption && len(f.Options) > 0 {
		quoted := make([]string, len(f.Options))
		for i, opt := range f.Options {
			quoted[i] = fmt.Sprintf("'%s'", opt)
		}
		parts = append(parts, fmt.Sprintf("CHECK(\"%s\" IN (%s))", name, strings.Join(quoted, ", ")))
	}

	if f.Type == parser.FieldReference {
		parts = append(parts, fmt.Sprintf("REFERENCES \"%s\"(id)", f.Reference))
	}

	return strings.Join(parts, " ")
}

func fieldToColumnName(f parser.Field) string {
	if f.Type == parser.FieldReference {
		return f.Name + "_id"
	}
	return f.Name
}

func fieldToSQLType(f parser.Field) string {
	switch f.Type {
	case parser.FieldText, parser.FieldEmail, parser.FieldRichtext,
		parser.FieldPassword, parser.FieldPhone, parser.FieldImage:
		return "TEXT"
	case parser.FieldBool:
		return "INTEGER"
	case parser.FieldTimestamp:
		return "DATETIME"
	case parser.FieldInt:
		return "INTEGER"
	case parser.FieldFloat:
		return "REAL"
	case parser.FieldOption:
		return "TEXT"
	case parser.FieldReference:
		return "INTEGER"
	default:
		return "TEXT"
	}
}

func fieldToDefault(f parser.Field) string {
	if f.Auto && f.Type == parser.FieldTimestamp {
		return " DEFAULT CURRENT_TIMESTAMP"
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
			return fmt.Sprintf(" DEFAULT %s", f.Default)
		default:
			escaped := strings.ReplaceAll(f.Default, "'", "''")
			return fmt.Sprintf(" DEFAULT '%s'", escaped)
		}
	}
	return ""
}

// MigrateInternal creates internal kilnx tables for sessions and jobs
func (db *DB) MigrateInternal() error {
	stmts := []string{
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
	for _, stmt := range stmts {
		if _, err := db.conn.Exec(stmt); err != nil {
			return fmt.Errorf("creating internal table: %w", err)
		}
	}
	return nil
}

func isValidIdentifier(name string) bool {
	return identifierRe.MatchString(name)
}
