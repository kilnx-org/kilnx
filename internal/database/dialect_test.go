package database

import (
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/parser"
)

// ---------- DetectDialect ----------

func TestDetectDialectSQLiteDefault(t *testing.T) {
	tests := []string{"app.db", "sqlite://app.db", "file:app.db", "/data/myapp.db", ""}
	for _, url := range tests {
		d := DetectDialect(url)
		if _, ok := d.(SqliteDialect); !ok {
			t.Errorf("DetectDialect(%q) = %T, want SqliteDialect", url, d)
		}
	}
}

func TestDetectDialectPostgres(t *testing.T) {
	tests := []string{
		"postgres://user:pass@localhost/mydb",
		"postgresql://user:pass@localhost:5432/mydb?sslmode=disable",
		"POSTGRES://UPPER@host/db",
	}
	for _, url := range tests {
		d := DetectDialect(url)
		if _, ok := d.(PostgresDialect); !ok {
			t.Errorf("DetectDialect(%q) = %T, want PostgresDialect", url, d)
		}
	}
}

// ---------- SqliteDialect ----------

func TestSqliteDialectPlaceholder(t *testing.T) {
	d := SqliteDialect{}
	for i := 1; i <= 5; i++ {
		if got := d.Placeholder(i); got != "?" {
			t.Errorf("SqliteDialect.Placeholder(%d) = %q, want ?", i, got)
		}
	}
}

func TestSqliteDialectDriverName(t *testing.T) {
	d := SqliteDialect{}
	if d.DriverName() != "sqlite" {
		t.Error("expected driver name 'sqlite'")
	}
}

func TestSqliteDialectDSN(t *testing.T) {
	d := SqliteDialect{}
	if got := d.DSN("sqlite://myapp.db"); got != "myapp.db" {
		t.Errorf("DSN(sqlite://myapp.db) = %q, want myapp.db", got)
	}
	if got := d.DSN("file:myapp.db"); got != "myapp.db" {
		t.Errorf("DSN(file:myapp.db) = %q, want myapp.db", got)
	}
	if got := d.DSN("/data/app.db"); got != "/data/app.db" {
		t.Errorf("DSN(/data/app.db) = %q, want /data/app.db", got)
	}
}

// ---------- PostgresDialect ----------

func TestPostgresDialectPlaceholder(t *testing.T) {
	d := PostgresDialect{}
	tests := []struct {
		idx  int
		want string
	}{
		{1, "$1"},
		{2, "$2"},
		{10, "$10"},
		{100, "$100"},
	}
	for _, tc := range tests {
		if got := d.Placeholder(tc.idx); got != tc.want {
			t.Errorf("PostgresDialect.Placeholder(%d) = %q, want %q", tc.idx, got, tc.want)
		}
	}
}

func TestPostgresDialectDriverName(t *testing.T) {
	d := PostgresDialect{}
	if d.DriverName() != "pgx" {
		t.Error("expected driver name 'pgx'")
	}
}

func TestPostgresDialectDSN(t *testing.T) {
	d := PostgresDialect{}
	url := "postgres://user:pass@localhost:5432/mydb"
	if got := d.DSN(url); got != url {
		t.Errorf("DSN should pass through, got %q", got)
	}
}

func TestPostgresDialectTypeMapping(t *testing.T) {
	d := PostgresDialect{}
	tests := []struct {
		fieldType parser.FieldType
		expected  string
	}{
		{parser.FieldText, "TEXT"},
		{parser.FieldBool, "BOOLEAN"},
		{parser.FieldInt, "INTEGER"},
		{parser.FieldFloat, "DOUBLE PRECISION"},
		{parser.FieldTimestamp, "TIMESTAMP"},
		{parser.FieldReference, "BIGINT"},
	}
	for _, tc := range tests {
		got := d.FieldToSQLType(parser.Field{Type: tc.fieldType})
		if got != tc.expected {
			t.Errorf("PostgresDialect.FieldToSQLType(%s) = %s, want %s", tc.fieldType, got, tc.expected)
		}
	}
}

func TestPostgresDialectDefaults(t *testing.T) {
	d := PostgresDialect{}

	// Timestamp auto -> NOW()
	f := parser.Field{Type: parser.FieldTimestamp, Auto: true}
	if got := d.FieldToDefault(f); got != " DEFAULT NOW()" {
		t.Errorf("expected DEFAULT NOW(), got %q", got)
	}

	// Bool auto -> FALSE
	f = parser.Field{Type: parser.FieldBool, Auto: true}
	if got := d.FieldToDefault(f); got != " DEFAULT FALSE" {
		t.Errorf("expected DEFAULT FALSE, got %q", got)
	}

	// Bool true -> TRUE
	f = parser.Field{Type: parser.FieldBool, Default: "true"}
	if got := d.FieldToDefault(f); got != " DEFAULT TRUE" {
		t.Errorf("expected DEFAULT TRUE, got %q", got)
	}
}

func TestPostgresDialectAutoIncrementPK(t *testing.T) {
	d := PostgresDialect{}
	if got := d.AutoIncrementPK(); got != "id BIGSERIAL PRIMARY KEY" {
		t.Errorf("expected BIGSERIAL PK, got %q", got)
	}
}

func TestPostgresDialectInternalTableDDL(t *testing.T) {
	d := PostgresDialect{}
	stmts := d.InternalTableDDL()
	if len(stmts) != 4 {
		t.Fatalf("expected 4 internal tables, got %d", len(stmts))
	}
	// Sessions table should use TIMESTAMP, not DATETIME
	if !strings.Contains(stmts[0], "TIMESTAMP") {
		t.Errorf("sessions DDL should use TIMESTAMP, got: %s", stmts[0])
	}
	// Migrations table should use BIGSERIAL, not AUTOINCREMENT
	if !strings.Contains(stmts[2], "BIGSERIAL") {
		t.Errorf("migrations DDL should use BIGSERIAL, got: %s", stmts[2])
	}
}

// ---------- bindParams with PostgresDialect ----------

func TestBindParamsPostgresPlaceholders(t *testing.T) {
	d := PostgresDialect{}
	query, args := bindParams(d,
		"SELECT * FROM t WHERE a = :x AND b = :y AND c = :x",
		map[string]string{"x": "1", "y": "2"},
	)
	// Should produce $1, $2, $3 (not ?)
	if strings.Contains(query, "?") {
		t.Errorf("PostgreSQL query should not contain ?, got: %s", query)
	}
	if !strings.Contains(query, "$1") || !strings.Contains(query, "$2") || !strings.Contains(query, "$3") {
		t.Errorf("expected $1, $2, $3 placeholders, got: %s", query)
	}
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d", len(args))
	}
}

func TestPostgresDialectTableExistsSQL(t *testing.T) {
	d := PostgresDialect{}
	sql := d.TableExistsSQL()
	if !strings.Contains(sql, "information_schema") {
		t.Errorf("expected information_schema query, got: %s", sql)
	}
	if !strings.Contains(sql, "$1") {
		t.Errorf("expected $1 placeholder, got: %s", sql)
	}
}

func TestPostgresDialectColumnsSQL(t *testing.T) {
	d := PostgresDialect{}
	sql := d.ColumnsSQL("users")
	if !strings.Contains(sql, "information_schema.columns") {
		t.Errorf("expected information_schema.columns, got: %s", sql)
	}
	if !strings.Contains(sql, "'users'") {
		t.Errorf("expected table name in query, got: %s", sql)
	}
}
