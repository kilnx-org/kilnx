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
	dialect     Dialect
	OnSlowQuery func(sql string, d time.Duration) // optional callback for slow query logging
}

// Dialect returns the dialect used by this database connection.
func (db *DB) Dialect() Dialect { return db.dialect }

// TxHandle wraps a sql.Tx with named parameter support
type TxHandle struct {
	tx        *sql.Tx
	dialect   Dialect
	committed bool
}

// BeginTxHandle starts a new transaction wrapped in a TxHandle
func (db *DB) BeginTxHandle() (*TxHandle, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return nil, err
	}
	return &TxHandle{tx: tx, dialect: db.dialect}, nil
}

// ExecWithParams executes a mutation within the transaction
func (th *TxHandle) ExecWithParams(sqlStr string, params map[string]string) error {
	return ExecWithParamsTx(th.tx, th.dialect, sqlStr, params)
}

// QueryRowsWithParams executes a SELECT within the transaction
func (th *TxHandle) QueryRowsWithParams(sqlStr string, params map[string]string) ([]Row, error) {
	return QueryRowsWithParamsTx(th.tx, th.dialect, sqlStr, params)
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

// DetectDialect returns the appropriate Dialect for a database URL.
// URLs starting with "postgres://" or "postgresql://" use PostgreSQL.
// Everything else defaults to SQLite.
func DetectDialect(url string) Dialect {
	lower := strings.ToLower(url)
	if strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "postgresql://") {
		return PostgresDialect{}
	}
	return SqliteDialect{}
}

// Open connects to the database identified by url.
// The driver is auto-detected from the URL scheme:
//   - postgres://... or postgresql://... -> PostgreSQL (pgx)
//   - sqlite://... or file:... or bare path -> SQLite
func Open(url string) (*DB, error) {
	dialect := DetectDialect(url)
	dsn := dialect.DSN(url)

	conn, err := sql.Open(dialect.DriverName(), dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database %s: %w", url, err)
	}

	for _, stmt := range dialect.InitStatements() {
		if _, err := conn.Exec(stmt); err != nil {
			conn.Close()
			return nil, fmt.Errorf("init statement %q: %w", stmt, err)
		}
	}

	return &DB{conn: conn, dialect: dialect}, nil
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
			stmts = append(stmts, db.generateCreateTable(model))
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
	query, args := bindParams(db.dialect, "INSERT INTO _kilnx_migrations (schema_hash, statements) VALUES (:hash, :stmts)",
		map[string]string{"hash": hash, "stmts": stmts})
	db.conn.Exec(query, args...)
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
	query := db.dialect.TableExistsSQL()
	err := db.conn.QueryRow(query, name).Scan(&count)
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

		sqlType := db.dialect.FieldToSQLType(field)
		defaultClause := db.dialect.FieldToDefault(field)

		stmt := fmt.Sprintf("ALTER TABLE \"%s\" ADD COLUMN \"%s\" %s%s",
			model.Name, colName, sqlType, defaultClause)
		stmts = append(stmts, stmt)
	}

	if model.CustomFieldsFile != "" {
		if _, ok := existing["custom"]; !ok {
			colType := "TEXT"
			if db.dialect.DriverName() == "pgx" {
				colType = "JSONB"
			}
			stmts = append(stmts, fmt.Sprintf("ALTER TABLE \"%s\" ADD COLUMN \"custom\" %s", model.Name, colType))
		}
	}

	return stmts, nil
}

func (db *DB) getColumns(table string) (map[string]bool, error) {
	if !isValidIdentifier(table) {
		return nil, fmt.Errorf("invalid table name: %q", table)
	}

	query := db.dialect.ColumnsSQL(table)
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return columns, nil
}

func (db *DB) generateCreateTable(model parser.Model) string {
	var cols []string

	cols = append(cols, db.dialect.AutoIncrementPK())

	for _, field := range model.Fields {
		col := db.fieldToColumnDef(field)
		cols = append(cols, col)
	}

	if model.CustomFieldsFile != "" {
		if db.dialect.DriverName() == "pgx" {
			cols = append(cols, `"custom" JSONB`)
		} else {
			cols = append(cols, `"custom" TEXT`)
		}
	}

	return fmt.Sprintf("CREATE TABLE \"%s\" (\n  %s\n)", model.Name, strings.Join(cols, ",\n  "))
}

func (db *DB) fieldToColumnDef(f parser.Field) string {
	name := fieldToColumnName(f)
	sqlType := db.dialect.FieldToSQLType(f)

	var parts []string
	parts = append(parts, fmt.Sprintf("\"%s\"", name), sqlType)

	if f.Required {
		parts = append(parts, "NOT NULL")
	}

	if f.Unique {
		parts = append(parts, "UNIQUE")
	}

	def := db.dialect.FieldToDefault(f)
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

// MigrateInternal creates internal kilnx tables for sessions and jobs
func (db *DB) MigrateInternal() error {
	for _, stmt := range db.dialect.InternalTableDDL() {
		if _, err := db.conn.Exec(stmt); err != nil {
			return fmt.Errorf("creating internal table: %w", err)
		}
	}
	return nil
}

func isValidIdentifier(name string) bool {
	return identifierRe.MatchString(name)
}
