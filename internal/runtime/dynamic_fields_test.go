package runtime

import (
	"testing"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

func newTestServerWithDB(t *testing.T) *Server {
	t.Helper()
	db, err := database.Open("sqlite://")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	s := &Server{db: db}
	return s
}

func TestMergeDBFieldDefs_EmptyTable(t *testing.T) {
	s := newTestServerWithDB(t)

	// Create the _deal_field_defs table.
	_, err := s.db.Conn().Exec(`CREATE TABLE "_deal_field_defs" (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		"name" TEXT NOT NULL, "kind" TEXT NOT NULL, "label" TEXT NOT NULL,
		"required" INTEGER NOT NULL DEFAULT 0, "options" TEXT,
		"reference_model" TEXT, "tenant_id" TEXT, "sort_order" INTEGER DEFAULT 0)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	base := &parser.CustomFieldManifest{ModelName: "deal", Fields: []parser.CustomFieldDef{
		{Name: "revenue", Kind: parser.CustomFieldKindNumber, Label: "Revenue"},
	}}
	merged := s.mergeDBFieldDefs("deal", base)

	if len(merged.Fields) != 1 {
		t.Errorf("expected 1 field (from base), got %d", len(merged.Fields))
	}
	if merged.Fields[0].Name != "revenue" {
		t.Errorf("expected field 'revenue', got %q", merged.Fields[0].Name)
	}
}

func TestMergeDBFieldDefs_MergesRows(t *testing.T) {
	s := newTestServerWithDB(t)

	_, err := s.db.Conn().Exec(`CREATE TABLE "_deal_field_defs" (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		"name" TEXT NOT NULL, "kind" TEXT NOT NULL, "label" TEXT NOT NULL,
		"required" INTEGER NOT NULL DEFAULT 0, "options" TEXT,
		"reference_model" TEXT, "tenant_id" TEXT, "sort_order" INTEGER DEFAULT 0)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	_, err = s.db.Conn().Exec(`INSERT INTO "_deal_field_defs" (name,kind,label) VALUES ('panel_brand','text','Panel Brand')`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	base := &parser.CustomFieldManifest{ModelName: "deal"}
	merged := s.mergeDBFieldDefs("deal", base)

	if len(merged.Fields) != 1 {
		t.Fatalf("expected 1 merged field, got %d", len(merged.Fields))
	}
	f := merged.Fields[0]
	if f.Name != "panel_brand" || f.Label != "Panel Brand" || f.Kind != parser.CustomFieldKindText {
		t.Errorf("unexpected field: %+v", f)
	}
}

func TestMergeDBFieldDefs_StaticFieldWins(t *testing.T) {
	s := newTestServerWithDB(t)

	_, err := s.db.Conn().Exec(`CREATE TABLE "_deal_field_defs" (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		"name" TEXT NOT NULL, "kind" TEXT NOT NULL, "label" TEXT NOT NULL,
		"required" INTEGER NOT NULL DEFAULT 0, "options" TEXT,
		"reference_model" TEXT, "tenant_id" TEXT, "sort_order" INTEGER DEFAULT 0)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	// Same name as a static field — must be skipped.
	_, err = s.db.Conn().Exec(`INSERT INTO "_deal_field_defs" (name,kind,label) VALUES ('revenue','text','Override Attempt')`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	base := &parser.CustomFieldManifest{ModelName: "deal", Fields: []parser.CustomFieldDef{
		{Name: "revenue", Kind: parser.CustomFieldKindNumber, Label: "Revenue"},
	}}
	merged := s.mergeDBFieldDefs("deal", base)

	if len(merged.Fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(merged.Fields))
	}
	if merged.Fields[0].Kind != parser.CustomFieldKindNumber {
		t.Error("static field must win over DB row with same name")
	}
}

func TestMergeDBFieldDefs_InvalidIdentifierSkipped(t *testing.T) {
	s := newTestServerWithDB(t)

	_, err := s.db.Conn().Exec(`CREATE TABLE "_deal_field_defs" (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		"name" TEXT NOT NULL, "kind" TEXT NOT NULL, "label" TEXT NOT NULL,
		"required" INTEGER NOT NULL DEFAULT 0, "options" TEXT,
		"reference_model" TEXT, "tenant_id" TEXT, "sort_order" INTEGER DEFAULT 0)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	_, err = s.db.Conn().Exec(`INSERT INTO "_deal_field_defs" (name,kind,label) VALUES ('bad name!','text','Bad')`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	base := &parser.CustomFieldManifest{ModelName: "deal"}
	merged := s.mergeDBFieldDefs("deal", base)

	if len(merged.Fields) != 0 {
		t.Errorf("expected 0 fields (invalid name skipped), got %d", len(merged.Fields))
	}
}

func TestMergeDBFieldDefs_TableMissing_GracefulDegrade(t *testing.T) {
	s := newTestServerWithDB(t)
	// Table does NOT exist.
	base := &parser.CustomFieldManifest{ModelName: "deal", Fields: []parser.CustomFieldDef{
		{Name: "revenue", Kind: parser.CustomFieldKindNumber},
	}}
	merged := s.mergeDBFieldDefs("deal", base)
	if len(merged.Fields) != 1 || merged.Fields[0].Name != "revenue" {
		t.Error("expected base manifest returned unchanged when table missing")
	}
}

func TestInvalidateDynamicManifestCache(t *testing.T) {
	s := &Server{}
	s.manifestCache.Store("__dynamic__:deal", &parser.CustomFieldManifest{ModelName: "deal"})

	// SQL that targets _deal_field_defs should invalidate.
	s.invalidateDynamicManifestCache(`INSERT INTO "_deal_field_defs" (name,kind,label) VALUES (?,?,?)`)
	if _, ok := s.manifestCache.Load("__dynamic__:deal"); ok {
		t.Error("cache entry must be deleted after invalidation")
	}

	// SQL not targeting _field_defs must be a no-op.
	s.manifestCache.Store("__dynamic__:deal", &parser.CustomFieldManifest{ModelName: "deal"})
	s.invalidateDynamicManifestCache(`INSERT INTO "deal" (name) VALUES (?)`)
	if _, ok := s.manifestCache.Load("__dynamic__:deal"); !ok {
		t.Error("cache entry must not be deleted for unrelated SQL")
	}
}

func TestModelHasCustomFields(t *testing.T) {
	s := &Server{}
	app := &parser.App{
		Models: []parser.Model{
			{Name: "deal", DynamicFields: true},
			{Name: "user"},
		},
		CustomManifests: map[string]*parser.CustomFieldManifest{
			"user": {ModelName: "user"},
		},
	}

	if !s.modelHasCustomFields(app, "deal") {
		t.Error("deal has DynamicFields=true — must return true")
	}
	if !s.modelHasCustomFields(app, "user") {
		t.Error("user has manifest — must return true")
	}
	if s.modelHasCustomFields(app, "other") {
		t.Error("other has no custom fields — must return false")
	}
}
