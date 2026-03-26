package database

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/kilnx-org/kilnx/internal/parser"
)

var identifierRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

type DB struct {
	conn *sql.DB
	path string
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

// Migrate compares models with the current database state and applies changes.
func (db *DB) Migrate(models []parser.Model) ([]string, error) {
	var executed []string

	for _, model := range models {
		if !isValidIdentifier(model.Name) {
			return executed, fmt.Errorf("invalid model name: %q", model.Name)
		}

		exists, err := db.tableExists(model.Name)
		if err != nil {
			return executed, err
		}

		if !exists {
			stmt := generateCreateTable(model)
			if _, err := db.conn.Exec(stmt); err != nil {
				return executed, fmt.Errorf("creating table '%s': %w\nSQL: %s", model.Name, err, stmt)
			}
			executed = append(executed, stmt)
		} else {
			stmts, err := db.migrateExistingTable(model)
			if err != nil {
				return executed, err
			}
			executed = append(executed, stmts...)
		}
	}

	return executed, nil
}

func (db *DB) tableExists(name string) (bool, error) {
	var count int
	err := db.conn.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name,
	).Scan(&count)
	return count > 0, err
}

func (db *DB) migrateExistingTable(model parser.Model) ([]string, error) {
	var executed []string

	existing, err := db.getColumns(model.Name)
	if err != nil {
		return nil, err
	}

	for _, field := range model.Fields {
		colName := fieldToColumnName(field)
		if !isValidIdentifier(colName) {
			return executed, fmt.Errorf("invalid column name: %q", colName)
		}
		if _, ok := existing[colName]; ok {
			continue
		}

		sqlType := fieldToSQLType(field)
		defaultClause := fieldToDefault(field)

		stmt := fmt.Sprintf("ALTER TABLE \"%s\" ADD COLUMN \"%s\" %s%s",
			model.Name, colName, sqlType, defaultClause)

		if _, err := db.conn.Exec(stmt); err != nil {
			return executed, fmt.Errorf("adding column '%s' to '%s': %w\nSQL: %s",
				colName, model.Name, err, stmt)
		}
		executed = append(executed, stmt)
	}

	return executed, nil
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
