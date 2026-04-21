package database

import (
	"strings"
	"testing"

	"github.com/kilnx-org/kilnx/internal/parser"
)

func TestFieldDefsTableDDL_SQLite(t *testing.T) {
	ddl := fieldDefsTableDDL("deal", false)
	if !strings.Contains(ddl, `"_deal_field_defs"`) {
		t.Error("DDL missing _deal_field_defs table name")
	}
	if !strings.Contains(ddl, "AUTOINCREMENT") {
		t.Error("DDL missing AUTOINCREMENT for SQLite")
	}
	if !strings.Contains(ddl, "INTEGER NOT NULL DEFAULT 0") {
		t.Error("DDL missing INTEGER required column for SQLite")
	}
	if !strings.Contains(ddl, "IF NOT EXISTS") {
		t.Error("DDL must be CREATE TABLE IF NOT EXISTS")
	}
}

func TestFieldDefsTableDDL_Postgres(t *testing.T) {
	ddl := fieldDefsTableDDL("deal", true)
	if !strings.Contains(ddl, "BIGSERIAL") {
		t.Error("DDL missing BIGSERIAL for PostgreSQL")
	}
	if !strings.Contains(ddl, "BOOLEAN NOT NULL DEFAULT FALSE") {
		t.Error("DDL missing BOOLEAN required column for PostgreSQL")
	}
}

func TestMigrate_DynamicFields_CreatesFieldDefsTable(t *testing.T) {
	db, err := Open("sqlite://")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	models := []parser.Model{
		{Name: "deal", DynamicFields: true, Fields: []parser.Field{
			{Name: "name", Type: parser.FieldText, Required: true},
		}},
	}

	stmts, err := db.PlanMigration(models)
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}

	hasFieldDefs := false
	hasCustomCol := false
	for _, s := range stmts {
		if strings.Contains(s, "_deal_field_defs") {
			hasFieldDefs = true
		}
		if strings.Contains(s, `"custom"`) {
			hasCustomCol = true
		}
	}
	if !hasFieldDefs {
		t.Error("expected _deal_field_defs CREATE TABLE in migration plan")
	}
	if !hasCustomCol {
		t.Error("expected custom column in deal table")
	}

	// Apply and verify idempotency
	if _, err := db.Migrate(models); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	// Second migrate must not error
	if _, err := db.Migrate(models); err != nil {
		t.Fatalf("Migrate second run: %v", err)
	}
}

func TestSchemaHash_DynamicFields(t *testing.T) {
	m1 := []parser.Model{{Name: "deal", Fields: []parser.Field{{Name: "name", Type: parser.FieldText}}}}
	m2 := []parser.Model{{Name: "deal", DynamicFields: true, Fields: []parser.Field{{Name: "name", Type: parser.FieldText}}}}

	h1 := schemaHash(m1)
	h2 := schemaHash(m2)
	if h1 == h2 {
		t.Error("schema hash must differ when DynamicFields toggles")
	}
}
