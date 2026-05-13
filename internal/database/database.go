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

// DB is a thin wrapper around *sql.DB that selects between SQLite and Postgres
// at runtime via a Dialect. It also exposes named-parameter execution helpers
// and an optional slow-query callback.
type DB struct {
	conn        *sql.DB
	dialect     Dialect
	OnSlowQuery func(sql string, d time.Duration) // optional callback for slow query logging
}

// Dialect returns the dialect used by this database connection.
func (db *DB) Dialect() Dialect { return db.dialect }

// TxHandle wraps a *sql.Tx with named-parameter execution and idempotent
// commit/rollback. It is created by [DB.BeginTxHandle].
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

// Close closes the underlying *sql.DB connection pool.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Conn returns the underlying *sql.DB for callers that need direct access
// (e.g. to start raw transactions or run driver-specific queries).
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// PlanMigration compares models with the current database state and returns
// the SQL statements that would be executed, without applying them.
// Pass a CustomManifests map as the optional second argument to include
// column-mode custom field columns in the plan.
func (db *DB) PlanMigration(models []parser.Model, manifests ...map[string]*parser.CustomFieldManifest) ([]string, error) {
	var cm map[string]*parser.CustomFieldManifest
	if len(manifests) > 0 {
		cm = manifests[0]
	}
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
			stmts = append(stmts, db.generateCreateTable(model, cm))
		} else {
			alterStmts, err := db.planExistingTable(model, cm)
			if err != nil {
				return stmts, err
			}
			stmts = append(stmts, alterStmts...)
		}

		if model.DynamicFields {
			isPostgres := db.dialect.DriverName() == "pgx"
			stmts = append(stmts, fieldDefsTableDDL(model.Name, isPostgres))
		}

		for _, field := range model.Fields {
			if field.AutoUpdate {
				stmts = append(stmts, db.dialect.AutoUpdateTriggerDDL(model.Name, field.Name)...)
			}
		}

		stmts = append(stmts, uniqueIndexDDLs(model)...)
		stmts = append(stmts, indexDDLs(model)...)
	}

	return stmts, nil
}

// indexDDLs emits `CREATE INDEX IF NOT EXISTS` statements for each
// non-unique `index (...)` group declared on the model. Index names
// are prefixed with `ix_` to stay distinguishable from composite
// UNIQUE indexes (`uq_`). Groups referencing unknown fields are
// skipped; the analyzer surfaces the error.
func indexDDLs(model parser.Model) []string {
	if len(model.Indexes) == 0 {
		return nil
	}
	byName := make(map[string]parser.Field, len(model.Fields))
	for _, f := range model.Fields {
		byName[f.Name] = f
	}
	var stmts []string
	for _, group := range model.Indexes {
		cols := make([]string, 0, len(group))
		ok := true
		for _, name := range group {
			f, found := byName[name]
			if !found {
				ok = false
				break
			}
			col := fieldToColumnName(f)
			if !isValidIdentifier(col) {
				ok = false
				break
			}
			cols = append(cols, col)
		}
		if !ok || len(cols) == 0 {
			continue
		}
		indexName := "ix_" + model.Name + "_" + strings.Join(cols, "_")
		if !isValidIdentifier(indexName) {
			continue
		}
		quoted := make([]string, len(cols))
		for i, c := range cols {
			quoted[i] = fmt.Sprintf("\"%s\"", c)
		}
		stmts = append(stmts, fmt.Sprintf(
			"CREATE INDEX IF NOT EXISTS \"%s\" ON \"%s\" (%s)",
			indexName, model.Name, strings.Join(quoted, ", "),
		))
	}
	return stmts
}

// uniqueIndexDDLs emits `CREATE UNIQUE INDEX IF NOT EXISTS` statements for
// each composite UNIQUE group declared on the model. Field names are
// resolved to their DB column names via fieldToColumnName (so references
// map to their `<name>_id` columns). Groups that reference unknown fields
// are skipped, the analyzer surfaces the error.
func uniqueIndexDDLs(model parser.Model) []string {
	if len(model.UniqueConstraints) == 0 {
		return nil
	}
	byName := make(map[string]parser.Field, len(model.Fields))
	for _, f := range model.Fields {
		byName[f.Name] = f
	}
	var stmts []string
	for _, group := range model.UniqueConstraints {
		cols := make([]string, 0, len(group))
		ok := true
		for _, name := range group {
			f, found := byName[name]
			if !found {
				ok = false
				break
			}
			col := fieldToColumnName(f)
			if !isValidIdentifier(col) {
				ok = false
				break
			}
			cols = append(cols, col)
		}
		if !ok || len(cols) < 2 {
			continue
		}
		indexName := "uq_" + model.Name + "_" + strings.Join(cols, "_")
		if !isValidIdentifier(indexName) {
			continue
		}
		quoted := make([]string, len(cols))
		for i, c := range cols {
			quoted[i] = fmt.Sprintf("\"%s\"", c)
		}
		stmts = append(stmts, fmt.Sprintf(
			"CREATE UNIQUE INDEX IF NOT EXISTS \"%s\" ON \"%s\" (%s)",
			indexName, model.Name, strings.Join(quoted, ", "),
		))
	}
	return stmts
}

// Migrate compares models with the current database state and applies changes.
// Pass a CustomManifests map as the optional second argument to include
// column-mode custom field columns.
func (db *DB) Migrate(models []parser.Model, manifests ...map[string]*parser.CustomFieldManifest) ([]string, error) {
	stmts, err := db.PlanMigration(models, manifests...)
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
		if m.DynamicFields {
			b.WriteString("  dynamic_fields:true\n")
		}
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
		for _, group := range m.UniqueConstraints {
			fmt.Fprintf(&b, "  unique:%s\n", strings.Join(group, ","))
		}
		for _, group := range m.Indexes {
			fmt.Fprintf(&b, "  index:%s\n", strings.Join(group, ","))
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

// DataLossRisk describes a table in the database that is no longer represented
// by any model in the current schema. If it contains rows, the migration may
// leave data behind.
type DataLossRisk struct {
	TableName  string
	RowCount   int
	Suggestion string // human-readable hint, e.g. an ALTER TABLE RENAME statement
}

// DetectDataLossRisk inspects the database for tables that do not correspond
// to any model in the provided slice and contain data. It returns a slice of
// risks and any error encountered during introspection.
func (db *DB) DetectDataLossRisk(models []parser.Model) ([]DataLossRisk, error) {
	modelNames := make(map[string]bool, len(models))
	for _, m := range models {
		modelNames[m.Name] = true
	}

	rows, err := db.conn.Query(db.dialect.ListTablesSQL())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var risks []DataLossRisk
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}
		if modelNames[tableName] {
			continue
		}

		var count int
		escaped := strings.ReplaceAll(tableName, `"`, `""`)
		countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, escaped)
		if err := db.conn.QueryRow(countQuery).Scan(&count); err != nil {
			return nil, fmt.Errorf("counting rows in %q: %w", tableName, err)
		}
		if count == 0 {
			continue
		}

		var suggestion string
		for _, m := range models {
			if strings.EqualFold(m.Name, tableName) {
				continue
			}
			if strings.EqualFold(m.Name+"s", tableName) ||
				strings.EqualFold(tableName+"s", m.Name) ||
				strings.EqualFold(m.Name, tableName+"es") ||
				strings.EqualFold(tableName, m.Name+"es") {
				suggestion = fmt.Sprintf(`run "ALTER TABLE %s RENAME TO %s;" before migrating to keep the data`, tableName, m.Name)
				break
			}
		}

		risks = append(risks, DataLossRisk{
			TableName:  tableName,
			RowCount:   count,
			Suggestion: suggestion,
		})
	}
	return risks, rows.Err()
}

// ColumnDriftKind classifies how a database column diverges from its model
// declaration. Drift detection is read-only; PlanMigration never emits ALTER
// COLUMN or DROP COLUMN automatically, so this is purely advisory.
type ColumnDriftKind string

const (
	// DriftOrphan: column exists in DB but no longer declared in the model.
	DriftOrphan ColumnDriftKind = "orphan"
	// DriftType: column type in DB differs from the type the model would emit.
	DriftType ColumnDriftKind = "type"
	// DriftNotNull: model and DB disagree on NOT NULL.
	DriftNotNull ColumnDriftKind = "notnull"
	// DriftUnique: model and DB disagree on single-column UNIQUE.
	DriftUnique ColumnDriftKind = "unique"
	// DriftDefault: model and DB disagree on the presence of a DEFAULT.
	// (Value comparison is intentionally skipped: dialect normalization is
	// too noisy to compare default expressions reliably across SQLite/PG.)
	DriftDefault ColumnDriftKind = "default"
)

// ColumnDrift describes one divergence between a declared model field and
// the live database schema. PlanMigration is intentionally additive (model
// -> DB) and never emits DROP COLUMN or ALTER COLUMN, so these warnings are
// the only signal that the schemas have diverged. `kilnx migrate` reports
// them as warnings; the migration itself is not blocked.
type ColumnDrift struct {
	Kind      ColumnDriftKind
	TableName string
	Column    string
	Detail    string // human-readable, e.g. "model=TEXT db=INTEGER"
}

// columnInfo captures the per-column data returned by Dialect.ColumnsInfoSQL.
type columnInfo struct {
	Type       string
	NotNull    bool
	HasDefault bool
}

// DetectColumnDrift introspects each declared model's table and returns
// every divergence between the live schema and the model declaration:
// orphan columns, type mismatches, NOT NULL mismatches, single-column
// UNIQUE mismatches, and DEFAULT presence/absence mismatches. Tables
// missing from the database are skipped (planExistingTable adds them as
// part of normal migration); orphan tables are covered by
// DetectDataLossRisk. Reserved `id` / `custom` columns and column-mode
// custom-field manifest entries are exempted.
func (db *DB) DetectColumnDrift(models []parser.Model, manifests ...map[string]*parser.CustomFieldManifest) ([]ColumnDrift, error) {
	var cm map[string]*parser.CustomFieldManifest
	if len(manifests) > 0 {
		cm = manifests[0]
	}
	var drifts []ColumnDrift
	for _, model := range models {
		if !isValidIdentifier(model.Name) {
			continue
		}
		exists, err := db.tableExists(model.Name)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		dbCols, err := db.getColumnsInfo(model.Name)
		if err != nil {
			return nil, err
		}
		dbUniques, err := db.getUniqueColumns(model.Name)
		if err != nil {
			return nil, err
		}

		// Build the expected column set: id + non-computed model fields +
		// reserved custom slot + column-mode manifest fields.
		expected := map[string]bool{"id": true}
		fieldByCol := make(map[string]parser.Field, len(model.Fields))
		for _, field := range model.Fields {
			if field.Type == parser.FieldComputed {
				continue
			}
			col := fieldToColumnName(field)
			expected[col] = true
			fieldByCol[col] = field
		}
		if model.CustomFieldsFile != "" || model.DynamicFields {
			expected["custom"] = true
		}
		if manifest, ok := cm[model.Name]; ok {
			for _, f := range manifest.Fields {
				if f.Mode != parser.CustomFieldModeColumn {
					continue
				}
				expected[f.Name] = true
			}
		}

		// Orphan columns (DB-only).
		for col := range dbCols {
			if !expected[col] {
				drifts = append(drifts, ColumnDrift{
					Kind:      DriftOrphan,
					TableName: model.Name,
					Column:    col,
				})
			}
		}

		// Per-field drift (type / notnull / unique / default presence).
		// Skip if the DB column is missing: that case is handled by the
		// additive migrator (planExistingTable will ADD COLUMN it).
		for col, field := range fieldByCol {
			info, ok := dbCols[col]
			if !ok {
				continue
			}
			expectedType := db.dialect.FieldToSQLType(field)
			if !typesEqual(expectedType, info.Type) {
				drifts = append(drifts, ColumnDrift{
					Kind:      DriftType,
					TableName: model.Name,
					Column:    col,
					Detail:    fmt.Sprintf("model=%s db=%s", expectedType, info.Type),
				})
			}
			if field.Required != info.NotNull {
				drifts = append(drifts, ColumnDrift{
					Kind:      DriftNotNull,
					TableName: model.Name,
					Column:    col,
					Detail:    fmt.Sprintf("model required=%t db notnull=%t", field.Required, info.NotNull),
				})
			}
			_, hasUniq := dbUniques[col]
			// Field-level UNIQUE only; primary key on id is excluded by
			// fieldByCol (id is never a field).
			if field.Unique != hasUniq {
				drifts = append(drifts, ColumnDrift{
					Kind:      DriftUnique,
					TableName: model.Name,
					Column:    col,
					Detail:    fmt.Sprintf("model unique=%t db unique=%t", field.Unique, hasUniq),
				})
			}
			modelHasDefault := field.Default != "" || field.Auto
			if modelHasDefault != info.HasDefault {
				drifts = append(drifts, ColumnDrift{
					Kind:      DriftDefault,
					TableName: model.Name,
					Column:    col,
					Detail:    fmt.Sprintf("model has_default=%t db has_default=%t", modelHasDefault, info.HasDefault),
				})
			}
		}
	}
	return drifts, nil
}

// typesEqual returns true when two SQL type strings are equivalent after
// normalization (lowercase, whitespace collapsed, common dialect aliases
// resolved, parameterized lengths stripped). Conservative: when in doubt
// it returns true to avoid false-positive drift warnings.
func typesEqual(a, b string) bool {
	return normalizeSQLType(a) == normalizeSQLType(b)
}

// normalizeSQLType maps a declared or introspected SQL type to a canonical
// form for cross-comparison. Strips parenthesized length/precision and
// resolves SQLite/PG type aliases (int/integer, bool/boolean, etc.).
func normalizeSQLType(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	if i := strings.IndexByte(t, '('); i >= 0 {
		t = strings.TrimSpace(t[:i])
	}
	t = strings.Join(strings.Fields(t), " ")
	switch t {
	case "int", "int4", "integer":
		return "integer"
	case "int8", "bigint", "bigserial":
		return "bigint"
	case "real", "float8", "double precision":
		return "double precision"
	case "bool", "boolean":
		return "boolean"
	case "timestamp", "timestamp without time zone", "datetime":
		return "timestamp"
	case "timestamptz", "timestamp with time zone":
		return "timestamptz"
	case "varchar", "character varying", "text":
		return "text"
	case "numeric", "decimal":
		return "numeric"
	}
	return t
}

func (db *DB) getColumnsInfo(table string) (map[string]columnInfo, error) {
	if !isValidIdentifier(table) {
		return nil, fmt.Errorf("invalid table name: %q", table)
	}
	rows, err := db.conn.Query(db.dialect.ColumnsInfoSQL(table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]columnInfo)
	for rows.Next() {
		var (
			name       string
			ctype      string
			notnull    int
			hasDefault int
		)
		if err := rows.Scan(&name, &ctype, &notnull, &hasDefault); err != nil {
			return nil, err
		}
		out[name] = columnInfo{Type: ctype, NotNull: notnull != 0, HasDefault: hasDefault != 0}
	}
	return out, rows.Err()
}

func (db *DB) getUniqueColumns(table string) (map[string]bool, error) {
	if !isValidIdentifier(table) {
		return nil, fmt.Errorf("invalid table name: %q", table)
	}
	rows, err := db.conn.Query(db.dialect.UniqueColumnsSQL(table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out[name] = true
	}
	return out, rows.Err()
}

func (db *DB) planExistingTable(model parser.Model, cm map[string]*parser.CustomFieldManifest) ([]string, error) {
	var stmts []string

	existing, err := db.getColumns(model.Name)
	if err != nil {
		return nil, err
	}

	for _, field := range model.Fields {
		if field.Type == parser.FieldComputed {
			continue
		}
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

	if model.CustomFieldsFile != "" || model.DynamicFields {
		if _, ok := existing["custom"]; !ok {
			colType := "TEXT"
			if db.dialect.DriverName() == "pgx" {
				colType = "JSONB"
			}
			stmts = append(stmts, fmt.Sprintf("ALTER TABLE \"%s\" ADD COLUMN \"custom\" %s", model.Name, colType))
		}
		// column-mode custom fields become real columns
		if manifest, ok := cm[model.Name]; ok {
			for _, f := range manifest.Fields {
				if f.Mode != parser.CustomFieldModeColumn {
					continue
				}
				if !isValidIdentifier(f.Name) {
					continue
				}
				if _, ok := existing[f.Name]; ok {
					continue
				}
				sqlType := customKindToSQL(f.Kind, db.dialect.DriverName() == "pgx")
				stmts = append(stmts, fmt.Sprintf("ALTER TABLE \"%s\" ADD COLUMN \"%s\" %s", model.Name, f.Name, sqlType))
			}
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

func (db *DB) generateCreateTable(model parser.Model, cm map[string]*parser.CustomFieldManifest) string {
	var cols []string

	cols = append(cols, db.dialect.AutoIncrementPK())

	for _, field := range model.Fields {
		if field.Type == parser.FieldComputed {
			continue
		}
		col := db.fieldToColumnDef(field)
		cols = append(cols, col)
	}

	if model.CustomFieldsFile != "" || model.DynamicFields {
		if db.dialect.DriverName() == "pgx" {
			cols = append(cols, `"custom" JSONB`)
		} else {
			cols = append(cols, `"custom" TEXT`)
		}
		// column-mode custom fields become real columns
		if manifest, ok := cm[model.Name]; ok {
			isPostgres := db.dialect.DriverName() == "pgx"
			// build set of names already used by model fields + reserved names
			usedNames := map[string]bool{"id": true, "custom": true}
			for _, field := range model.Fields {
				usedNames[fieldToColumnName(field)] = true
			}
			for _, f := range manifest.Fields {
				if f.Mode != parser.CustomFieldModeColumn || !isValidIdentifier(f.Name) {
					continue
				}
				if usedNames[f.Name] {
					continue
				}
				cols = append(cols, fmt.Sprintf(`"%s" %s`, f.Name, customKindToSQL(f.Kind, isPostgres)))
			}
		}
	}

	return fmt.Sprintf("CREATE TABLE \"%s\" (\n  %s\n)", model.Name, strings.Join(cols, ",\n  "))
}

// customKindToSQL maps a manifest field kind to a SQL column type.
func customKindToSQL(kind parser.CustomFieldKind, isPostgres bool) string {
	switch kind {
	case parser.CustomFieldKindNumber:
		return "REAL"
	case parser.CustomFieldKindBool:
		if isPostgres {
			return "BOOLEAN"
		}
		return "INTEGER"
	case parser.CustomFieldKindReference:
		if isPostgres {
			return "BIGINT"
		}
		return "INTEGER"
	default:
		return "TEXT"
	}
}

// fieldDefsTableDDL returns the CREATE TABLE IF NOT EXISTS for _<model>_field_defs.
func fieldDefsTableDDL(modelName string, isPostgres bool) string {
	pkDef := "id INTEGER PRIMARY KEY AUTOINCREMENT"
	boolType := "INTEGER"
	boolDefault := "DEFAULT 0"
	if isPostgres {
		pkDef = "id BIGSERIAL PRIMARY KEY"
		boolType = "BOOLEAN"
		boolDefault = "DEFAULT FALSE"
	}
	return fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS \"_%s_field_defs\" (\n"+
			"  %s,\n"+
			"  \"name\" TEXT NOT NULL,\n"+
			"  \"kind\" TEXT NOT NULL,\n"+
			"  \"label\" TEXT NOT NULL,\n"+
			"  \"required\" %s NOT NULL %s,\n"+
			"  \"options\" TEXT,\n"+
			"  \"reference_model\" TEXT,\n"+
			"  \"tenant_id\" TEXT,\n"+
			"  \"sort_order\" INTEGER DEFAULT 0\n"+
			")",
		modelName, pkDef, boolType, boolDefault)
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

// IsValidIdentifier reports whether name is a safe SQL identifier.
func IsValidIdentifier(name string) bool {
	return isValidIdentifier(name)
}
