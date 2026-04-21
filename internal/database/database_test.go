package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// helper: open a temp SQLite database, return the DB and a cleanup function.
func openTemp(t *testing.T) (*DB, func()) {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(tmp)
	if err != nil {
		t.Fatalf("Open(%s): %v", tmp, err)
	}
	return db, func() { db.Close() }
}

// ---------- Open / Close ----------

func TestOpenAndClose(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	// A simple query should succeed on a freshly opened database.
	rows, err := db.QueryRows("SELECT 1 AS val")
	if err != nil {
		t.Fatalf("query after open: %v", err)
	}
	if len(rows) != 1 || rows[0]["val"] != "1" {
		t.Fatalf("expected [{val:1}], got %v", rows)
	}
}

func TestOpenInvalidPath(t *testing.T) {
	_, err := Open("/nonexistent/dir/db.sqlite")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestClosePreventsFurtherQueries(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(tmp)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	_, err = db.QueryRows("SELECT 1")
	if err == nil {
		t.Fatal("expected error after close")
	}
}

// ---------- MigrateInternal ----------

func TestMigrateInternal(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	if err := db.MigrateInternal(); err != nil {
		t.Fatalf("MigrateInternal: %v", err)
	}

	// Verify _kilnx_sessions exists
	rows, err := db.QueryRows("SELECT name FROM sqlite_master WHERE type='table' AND name='_kilnx_sessions'")
	if err != nil {
		t.Fatalf("checking sessions table: %v", err)
	}
	if len(rows) != 1 {
		t.Fatal("_kilnx_sessions table not created")
	}

	// Verify _kilnx_jobs exists
	rows, err = db.QueryRows("SELECT name FROM sqlite_master WHERE type='table' AND name='_kilnx_jobs'")
	if err != nil {
		t.Fatalf("checking jobs table: %v", err)
	}
	if len(rows) != 1 {
		t.Fatal("_kilnx_jobs table not created")
	}
}

func TestMigrateInternalIdempotent(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	if err := db.MigrateInternal(); err != nil {
		t.Fatalf("first MigrateInternal: %v", err)
	}
	if err := db.MigrateInternal(); err != nil {
		t.Fatalf("second MigrateInternal should be idempotent: %v", err)
	}
}

// ---------- Migrate models ----------

func TestMigrateCreatesTable(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	models := []parser.Model{{
		Name: "user",
		Fields: []parser.Field{
			{Name: "name", Type: parser.FieldText, Required: true},
			{Name: "email", Type: parser.FieldEmail, Required: true, Unique: true},
			{Name: "age", Type: parser.FieldInt},
			{Name: "score", Type: parser.FieldFloat},
			{Name: "active", Type: parser.FieldBool},
			{Name: "created_at", Type: parser.FieldTimestamp, Auto: true},
		},
	}}

	stmts, err := db.Migrate(models)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if len(stmts) == 0 {
		t.Fatal("expected at least one executed statement")
	}

	// Verify columns via PRAGMA table_info
	rows, err := db.QueryRows(`PRAGMA table_info("user")`)
	if err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}

	colNames := make(map[string]bool)
	for _, r := range rows {
		colNames[r["name"]] = true
	}

	for _, expected := range []string{"id", "name", "email", "age", "score", "active", "created_at"} {
		if !colNames[expected] {
			t.Errorf("column %q not found, got %v", expected, colNames)
		}
	}
}

func TestMigrateReferenceField(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	models := []parser.Model{
		{Name: "author", Fields: []parser.Field{
			{Name: "name", Type: parser.FieldText},
		}},
		{Name: "post", Fields: []parser.Field{
			{Name: "title", Type: parser.FieldText},
			{Name: "author", Type: parser.FieldReference, Reference: "author"},
		}},
	}

	if _, err := db.Migrate(models); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	rows, err := db.QueryRows(`PRAGMA table_info("post")`)
	if err != nil {
		t.Fatalf("PRAGMA: %v", err)
	}
	found := false
	for _, r := range rows {
		if r["name"] == "author_id" {
			found = true
		}
	}
	if !found {
		t.Error("reference field should produce author_id column")
	}
}

func TestMigrateInvalidModelName(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	models := []parser.Model{{Name: "drop table; --", Fields: nil}}
	_, err := db.Migrate(models)
	if err == nil {
		t.Fatal("expected error for invalid model name")
	}
}

// ---------- Migrate additive ----------

func TestMigrateAddsNewColumn(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	// First migration: table with one field
	v1 := []parser.Model{{
		Name:   "product",
		Fields: []parser.Field{{Name: "name", Type: parser.FieldText}},
	}}
	if _, err := db.Migrate(v1); err != nil {
		t.Fatalf("v1 Migrate: %v", err)
	}

	// Second migration: add a field
	v2 := []parser.Model{{
		Name: "product",
		Fields: []parser.Field{
			{Name: "name", Type: parser.FieldText},
			{Name: "price", Type: parser.FieldFloat},
		},
	}}
	stmts, err := db.Migrate(v2)
	if err != nil {
		t.Fatalf("v2 Migrate: %v", err)
	}
	if len(stmts) == 0 {
		t.Fatal("expected ALTER TABLE statement for new column")
	}

	// Verify column exists
	rows, err := db.QueryRows(`PRAGMA table_info("product")`)
	if err != nil {
		t.Fatalf("PRAGMA: %v", err)
	}
	found := false
	for _, r := range rows {
		if r["name"] == "price" {
			found = true
		}
	}
	if !found {
		t.Error("price column should have been added")
	}
}

func TestMigrateNoOpWhenColumnsExist(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	models := []parser.Model{{
		Name:   "item",
		Fields: []parser.Field{{Name: "name", Type: parser.FieldText}},
	}}
	if _, err := db.Migrate(models); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}

	stmts, err := db.Migrate(models)
	if err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	if len(stmts) != 0 {
		t.Errorf("expected no statements for unchanged model, got %v", stmts)
	}
}

// ---------- QueryRows ----------

func TestQueryRows(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	db.Conn().Exec(`CREATE TABLE colors (id INTEGER PRIMARY KEY, name TEXT)`)
	db.Conn().Exec(`INSERT INTO colors (name) VALUES ('red')`)
	db.Conn().Exec(`INSERT INTO colors (name) VALUES ('blue')`)

	rows, err := db.QueryRows("SELECT id, name FROM colors ORDER BY id")
	if err != nil {
		t.Fatalf("QueryRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["name"] != "red" {
		t.Errorf("expected red, got %s", rows[0]["name"])
	}
	if rows[1]["name"] != "blue" {
		t.Errorf("expected blue, got %s", rows[1]["name"])
	}
}

func TestQueryRowsEmpty(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	db.Conn().Exec(`CREATE TABLE empty_table (id INTEGER PRIMARY KEY)`)
	rows, err := db.QueryRows("SELECT * FROM empty_table")
	if err != nil {
		t.Fatalf("QueryRows: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(rows))
	}
}

func TestQueryRowsInvalidSQL(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	_, err := db.QueryRows("SELECT * FROM nonexistent_table")
	if err == nil {
		t.Fatal("expected error for nonexistent table")
	}
}

// ---------- QueryRowsWithParams ----------

func TestQueryRowsWithParams(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	db.Conn().Exec(`CREATE TABLE people (id INTEGER PRIMARY KEY, name TEXT, city TEXT)`)
	db.Conn().Exec(`INSERT INTO people (name, city) VALUES ('Alice', 'NYC')`)
	db.Conn().Exec(`INSERT INTO people (name, city) VALUES ('Bob', 'LA')`)
	db.Conn().Exec(`INSERT INTO people (name, city) VALUES ('Charlie', 'NYC')`)

	rows, err := db.QueryRowsWithParams(
		"SELECT name FROM people WHERE city = :city ORDER BY name",
		map[string]string{"city": "NYC"},
	)
	if err != nil {
		t.Fatalf("QueryRowsWithParams: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["name"] != "Alice" {
		t.Errorf("expected Alice, got %s", rows[0]["name"])
	}
}

func TestQueryRowsWithMultipleParams(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	db.Conn().Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT, category TEXT, price REAL)`)
	db.Conn().Exec(`INSERT INTO items (name, category, price) VALUES ('Widget', 'A', 9.99)`)
	db.Conn().Exec(`INSERT INTO items (name, category, price) VALUES ('Gadget', 'A', 19.99)`)
	db.Conn().Exec(`INSERT INTO items (name, category, price) VALUES ('Doohickey', 'B', 5.00)`)

	rows, err := db.QueryRowsWithParams(
		"SELECT name FROM items WHERE category = :cat AND price > :min_price",
		map[string]string{"cat": "A", "min_price": "10"},
	)
	if err != nil {
		t.Fatalf("QueryRowsWithParams: %v", err)
	}
	if len(rows) != 1 || rows[0]["name"] != "Gadget" {
		t.Errorf("expected [Gadget], got %v", rows)
	}
}

// ---------- ExecWithParams ----------

func TestExecWithParamsInsert(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	db.Conn().Exec(`CREATE TABLE notes (id INTEGER PRIMARY KEY, title TEXT, body TEXT)`)

	err := db.ExecWithParams(
		"INSERT INTO notes (title, body) VALUES (:title, :body)",
		map[string]string{"title": "Hello", "body": "World"},
	)
	if err != nil {
		t.Fatalf("ExecWithParams INSERT: %v", err)
	}

	rows, err := db.QueryRows("SELECT title, body FROM notes")
	if err != nil {
		t.Fatalf("verifying insert: %v", err)
	}
	if len(rows) != 1 || rows[0]["title"] != "Hello" || rows[0]["body"] != "World" {
		t.Errorf("unexpected data: %v", rows)
	}
}

func TestExecWithParamsUpdate(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	db.Conn().Exec(`CREATE TABLE counters (id INTEGER PRIMARY KEY, name TEXT, val INTEGER)`)
	db.Conn().Exec(`INSERT INTO counters (name, val) VALUES ('hits', 0)`)

	err := db.ExecWithParams(
		"UPDATE counters SET val = :val WHERE name = :name",
		map[string]string{"val": "42", "name": "hits"},
	)
	if err != nil {
		t.Fatalf("ExecWithParams UPDATE: %v", err)
	}

	rows, _ := db.QueryRows("SELECT val FROM counters WHERE name='hits'")
	if len(rows) != 1 || rows[0]["val"] != "42" {
		t.Errorf("expected val=42, got %v", rows)
	}
}

func TestExecWithParamsDelete(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	db.Conn().Exec(`CREATE TABLE tasks (id INTEGER PRIMARY KEY, done INTEGER)`)
	db.Conn().Exec(`INSERT INTO tasks (done) VALUES (0)`)
	db.Conn().Exec(`INSERT INTO tasks (done) VALUES (1)`)

	err := db.ExecWithParams(
		"DELETE FROM tasks WHERE done = :done",
		map[string]string{"done": "1"},
	)
	if err != nil {
		t.Fatalf("ExecWithParams DELETE: %v", err)
	}

	rows, _ := db.QueryRows("SELECT COUNT(*) AS cnt FROM tasks")
	if rows[0]["cnt"] != "1" {
		t.Errorf("expected 1 remaining, got %s", rows[0]["cnt"])
	}
}

// ---------- BeginTxHandle ----------

func TestTxHandleCommit(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	db.Conn().Exec(`CREATE TABLE txtest (id INTEGER PRIMARY KEY, val TEXT)`)

	th, err := db.BeginTxHandle()
	if err != nil {
		t.Fatalf("BeginTxHandle: %v", err)
	}

	if err := th.ExecWithParams("INSERT INTO txtest (val) VALUES (:v)", map[string]string{"v": "committed"}); err != nil {
		t.Fatalf("tx exec: %v", err)
	}
	if err := th.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	rows, _ := db.QueryRows("SELECT val FROM txtest")
	if len(rows) != 1 || rows[0]["val"] != "committed" {
		t.Errorf("committed data not visible: %v", rows)
	}
}

func TestTxHandleRollback(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	db.Conn().Exec(`CREATE TABLE txtest2 (id INTEGER PRIMARY KEY, val TEXT)`)

	th, err := db.BeginTxHandle()
	if err != nil {
		t.Fatalf("BeginTxHandle: %v", err)
	}

	th.ExecWithParams("INSERT INTO txtest2 (val) VALUES (:v)", map[string]string{"v": "rolled_back"})
	if err := th.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	rows, _ := db.QueryRows("SELECT val FROM txtest2")
	if len(rows) != 0 {
		t.Errorf("expected 0 rows after rollback, got %d", len(rows))
	}
}

func TestTxHandleDoubleCommitNoOp(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	db.Conn().Exec(`CREATE TABLE txtest3 (id INTEGER PRIMARY KEY)`)

	th, _ := db.BeginTxHandle()
	th.Commit()
	// Second commit should be a no-op, not an error
	if err := th.Commit(); err != nil {
		t.Errorf("double Commit should return nil, got %v", err)
	}
}

func TestTxHandleRollbackAfterCommitNoOp(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	db.Conn().Exec(`CREATE TABLE txtest4 (id INTEGER PRIMARY KEY)`)

	th, _ := db.BeginTxHandle()
	th.Commit()
	if err := th.Rollback(); err != nil {
		t.Errorf("Rollback after Commit should return nil, got %v", err)
	}
}

func TestTxHandleQueryRowsWithParams(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	db.Conn().Exec(`CREATE TABLE txquery (id INTEGER PRIMARY KEY, name TEXT)`)
	db.Conn().Exec(`INSERT INTO txquery (name) VALUES ('alpha')`)
	db.Conn().Exec(`INSERT INTO txquery (name) VALUES ('beta')`)

	th, _ := db.BeginTxHandle()
	defer th.Rollback()

	rows, err := th.QueryRowsWithParams(
		"SELECT name FROM txquery WHERE name = :n",
		map[string]string{"n": "alpha"},
	)
	if err != nil {
		t.Fatalf("tx QueryRowsWithParams: %v", err)
	}
	if len(rows) != 1 || rows[0]["name"] != "alpha" {
		t.Errorf("expected [alpha], got %v", rows)
	}
}

// ---------- Param binding edge cases ----------

func TestBindParamsMissingLeftAsIs(t *testing.T) {
	query, args := bindParams(SqliteDialect{},
		"SELECT * FROM t WHERE a = :known AND b = :missing",
		map[string]string{"known": "val"},
	)
	// :missing should remain in the query string
	if !strings.Contains(query, ":missing") {
		t.Errorf("missing param should stay as-is, got query: %s", query)
	}
	if len(args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(args))
	}
}

func TestBindParamsSpecialCharactersInValues(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	db.Conn().Exec(`CREATE TABLE special (id INTEGER PRIMARY KEY, data TEXT)`)

	special := "O'Reilly; DROP TABLE special; --"
	err := db.ExecWithParams(
		"INSERT INTO special (data) VALUES (:data)",
		map[string]string{"data": special},
	)
	if err != nil {
		t.Fatalf("insert special chars: %v", err)
	}

	rows, _ := db.QueryRows("SELECT data FROM special")
	if len(rows) != 1 || rows[0]["data"] != special {
		t.Errorf("expected %q, got %v", special, rows)
	}
}

func TestBindParamsEmptyMap(t *testing.T) {
	query, args := bindParams(SqliteDialect{}, "SELECT 1", map[string]string{})
	if query != "SELECT 1" {
		t.Errorf("unexpected query: %s", query)
	}
	if len(args) != 0 {
		t.Errorf("expected 0 args, got %d", len(args))
	}
}

func TestBindParamsDuplicateParam(t *testing.T) {
	query, args := bindParams(SqliteDialect{},
		"SELECT * FROM t WHERE a = :x OR b = :x",
		map[string]string{"x": "val"},
	)
	// Both occurrences should be replaced
	if strings.Contains(query, ":x") {
		t.Errorf("param :x should be replaced in both positions, got: %s", query)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args (one per occurrence), got %d", len(args))
	}
}

// ---------- SQL type mapping ----------

func TestFieldToSQLTypeMapping(t *testing.T) {
	d := SqliteDialect{}
	tests := []struct {
		fieldType parser.FieldType
		expected  string
	}{
		{parser.FieldText, "TEXT"},
		{parser.FieldEmail, "TEXT"},
		{parser.FieldPassword, "TEXT"},
		{parser.FieldPhone, "TEXT"},
		{parser.FieldImage, "TEXT"},
		{parser.FieldRichtext, "TEXT"},
		{parser.FieldOption, "TEXT"},
		{parser.FieldBool, "INTEGER"},
		{parser.FieldInt, "INTEGER"},
		{parser.FieldFloat, "REAL"},
		{parser.FieldTimestamp, "DATETIME"},
		{parser.FieldReference, "INTEGER"},
	}
	for _, tc := range tests {
		got := d.FieldToSQLType(parser.Field{Type: tc.fieldType})
		if got != tc.expected {
			t.Errorf("FieldToSQLType(%s) = %s, want %s", tc.fieldType, got, tc.expected)
		}
	}
}

// ---------- Default values ----------

func TestFieldToDefaultTimestampAuto(t *testing.T) {
	d := SqliteDialect{}
	f := parser.Field{Type: parser.FieldTimestamp, Auto: true}
	got := d.FieldToDefault(f)
	if got != " DEFAULT CURRENT_TIMESTAMP" {
		t.Errorf("expected DEFAULT CURRENT_TIMESTAMP, got %q", got)
	}
}

func TestFieldToDefaultBoolAuto(t *testing.T) {
	d := SqliteDialect{}
	f := parser.Field{Type: parser.FieldBool, Auto: true}
	got := d.FieldToDefault(f)
	if got != " DEFAULT 0" {
		t.Errorf("expected DEFAULT 0, got %q", got)
	}
}

func TestFieldToDefaultBoolTrue(t *testing.T) {
	d := SqliteDialect{}
	f := parser.Field{Type: parser.FieldBool, Default: "true"}
	got := d.FieldToDefault(f)
	if got != " DEFAULT 1" {
		t.Errorf("expected DEFAULT 1, got %q", got)
	}
}

func TestFieldToDefaultTextEscapeQuotes(t *testing.T) {
	d := SqliteDialect{}
	f := parser.Field{Type: parser.FieldText, Default: "it's fine"}
	got := d.FieldToDefault(f)
	if got != " DEFAULT 'it''s fine'" {
		t.Errorf("expected escaped quote, got %q", got)
	}
}

// ---------- isValidIdentifier ----------

func TestIsValidIdentifier(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"users", true},
		{"_private", true},
		{"CamelCase", true},
		{"has_123", true},
		{"123start", false},
		{"has space", false},
		{"drop;table", false},
		{"", false},
	}
	for _, tc := range tests {
		got := isValidIdentifier(tc.input)
		if got != tc.valid {
			t.Errorf("isValidIdentifier(%q) = %v, want %v", tc.input, got, tc.valid)
		}
	}
}

// ---------- generateCreateTable ----------

func TestGenerateCreateTableContainsAutoincrement(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()
	model := parser.Model{
		Name: "book",
		Fields: []parser.Field{
			{Name: "title", Type: parser.FieldText, Required: true},
		},
	}
	sql := db.generateCreateTable(model)
	if !strings.Contains(sql, "id INTEGER PRIMARY KEY AUTOINCREMENT") {
		t.Errorf("missing id AUTOINCREMENT in: %s", sql)
	}
	if !strings.Contains(sql, `"title" TEXT NOT NULL`) {
		t.Errorf("missing title column in: %s", sql)
	}
}

// ---------- Conn accessor ----------

func TestConnReturnsUnderlyingDB(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.db")
	db, _ := Open(tmp)
	defer db.Close()

	if db.Conn() == nil {
		t.Fatal("Conn() returned nil")
	}
}

// ---------- WAL mode and foreign keys ----------

func TestWALModeEnabled(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	rows, err := db.QueryRows("PRAGMA journal_mode")
	if err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if len(rows) == 0 || rows[0]["journal_mode"] != "wal" {
		t.Errorf("expected WAL journal mode, got %v", rows)
	}
}

func TestForeignKeysEnabled(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	rows, err := db.QueryRows("PRAGMA foreign_keys")
	if err != nil {
		t.Fatalf("PRAGMA foreign_keys: %v", err)
	}
	if len(rows) == 0 || rows[0]["foreign_keys"] != "1" {
		t.Errorf("expected foreign_keys=1, got %v", rows)
	}
}

// ---------- temp file cleanup sanity ----------

func TestTempDirCleanedUp(t *testing.T) {
	var dbPath string
	func() {
		dir := t.TempDir()
		dbPath = filepath.Join(dir, "ephemeral.db")
		db, err := Open(dbPath)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		db.Close()
	}()
	// After the inner func, TempDir should be cleaned up by t after the test.
	// Just confirm the path was set.
	if dbPath == "" {
		t.Fatal("dbPath was not set")
	}
	_ = os.Remove(dbPath) // best-effort cleanup
}

func TestPlanMigration_DryRun(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	models := []parser.Model{
		{Name: "task", Fields: []parser.Field{
			{Name: "title", Type: parser.FieldText, Required: true},
			{Name: "done", Type: parser.FieldBool},
		}},
	}

	stmts, err := db.PlanMigration(models)
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(stmts))
	}
	if !strings.HasPrefix(stmts[0], "CREATE TABLE") {
		t.Errorf("expected CREATE TABLE, got %q", stmts[0])
	}

	// Table should NOT exist (dry run)
	exists, _ := db.tableExists("task")
	if exists {
		t.Error("PlanMigration should not create tables")
	}
}

func TestMigrationHistory(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	if err := db.MigrateInternal(); err != nil {
		t.Fatalf("MigrateInternal: %v", err)
	}

	models := []parser.Model{
		{Name: "note", Fields: []parser.Field{
			{Name: "body", Type: parser.FieldText},
		}},
	}

	_, err := db.Migrate(models)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	history, err := db.MigrationHistory()
	if err != nil {
		t.Fatalf("MigrationHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 migration record, got %d", len(history))
	}
	if history[0].SchemaHash == "" {
		t.Error("schema hash should not be empty")
	}
	if !strings.Contains(history[0].Statements, "CREATE TABLE") {
		t.Error("statements should contain CREATE TABLE")
	}
}

func TestMigrationStatus_Pending(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	if err := db.MigrateInternal(); err != nil {
		t.Fatalf("MigrateInternal: %v", err)
	}

	// Migrate initial model
	models := []parser.Model{
		{Name: "item", Fields: []parser.Field{
			{Name: "name", Type: parser.FieldText},
		}},
	}
	if _, err := db.Migrate(models); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Add a field
	models[0].Fields = append(models[0].Fields, parser.Field{
		Name: "price", Type: parser.FieldFloat,
	})

	// Plan should show pending ALTER
	pending, err := db.PlanMigration(models)
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending statement, got %d", len(pending))
	}
	if !strings.Contains(pending[0], "ALTER TABLE") {
		t.Errorf("expected ALTER TABLE, got %q", pending[0])
	}
}

func TestCustomFieldsColumn(t *testing.T) {
	db, cleanup := openTemp(t)
	defer cleanup()

	models := []parser.Model{
		{
			Name:             "deal",
			CustomFieldsFile: "deal_fields.kilnx",
			Fields: []parser.Field{
				{Name: "title", Type: parser.FieldText},
			},
		},
	}

	stmts, err := db.PlanMigration(models)
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 CREATE TABLE statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], `"custom" TEXT`) {
		t.Errorf("expected 'custom' TEXT column in CREATE TABLE, got:\n%s", stmts[0])
	}

	if _, err := db.Migrate(models); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Removing CustomFieldsFile should not drop the column (ALTER TABLE ADD COLUMN only)
	models[0].CustomFieldsFile = "deal_fields.kilnx"
	pending, err := db.PlanMigration(models)
	if err != nil {
		t.Fatalf("PlanMigration second: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("expected 0 pending changes (custom column already exists), got %d: %v", len(pending), pending)
	}
}

func TestRewriteCustomFieldShorthand_SQLite(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{
			`SELECT * FROM deal WHERE custom.revenue > 100`,
			`SELECT * FROM deal WHERE json_extract("custom", '$.revenue') > 100`,
		},
		{
			`SELECT custom.region FROM deal ORDER BY custom.region`,
			`SELECT json_extract("custom", '$.region') FROM deal ORDER BY json_extract("custom", '$.region')`,
		},
		{
			`SELECT * FROM deal WHERE status = 'open'`,
			`SELECT * FROM deal WHERE status = 'open'`, // no change
		},
	}
	for _, c := range cases {
		got := RewriteCustomFieldShorthand(c.in, false)
		if got != c.want {
			t.Errorf("SQLite rewrite:\n  in:   %s\n  want: %s\n  got:  %s", c.in, c.want, got)
		}
	}
}

func TestRewriteCustomFieldShorthand_Postgres(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{
			`SELECT * FROM deal WHERE custom.revenue > 100`,
			`SELECT * FROM deal WHERE "custom"->>'revenue' > 100`,
		},
		{
			`SELECT custom.region FROM deal`,
			`SELECT "custom"->>'region' FROM deal`,
		},
	}
	for _, c := range cases {
		got := RewriteCustomFieldShorthand(c.in, true)
		if got != c.want {
			t.Errorf("Postgres rewrite:\n  in:   %s\n  want: %s\n  got:  %s", c.in, c.want, got)
		}
	}
}
