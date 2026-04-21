package parser

import "testing"

func TestModelDynamicFieldsDirective(t *testing.T) {
	src := `
config
  database: "sqlite://app.db"
  port: 8080
  secret: "s"

model deal
  name: text required
  dynamic fields
`
	app := parse(t, src)
	if len(app.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(app.Models))
	}
	if !app.Models[0].DynamicFields {
		t.Error("expected DynamicFields=true")
	}
}

func TestModelDynamicFieldsWithStaticManifest(t *testing.T) {
	src := `
config
  database: "sqlite://app.db"
  port: 8080
  secret: "s"

model deal
  name: text required
  custom fields from "base.kilnx"
  dynamic fields
`
	app := parse(t, src)
	m := app.Models[0]
	if !m.DynamicFields {
		t.Error("expected DynamicFields=true")
	}
	if m.CustomFieldsFile != "base.kilnx" {
		t.Errorf("expected CustomFieldsFile=base.kilnx, got %q", m.CustomFieldsFile)
	}
}

func TestModelDynamicFieldsDuplicate(t *testing.T) {
	src := `
config
  database: "sqlite://app.db"
  port: 8080
  secret: "s"

model deal
  name: text required
  dynamic fields
  dynamic fields
`
	_, err := parseAllowErrors(t, src)
	if err == nil {
		t.Fatal("expected parse error for duplicate dynamic fields directive")
	}
}

func TestModelDynamicFieldNotDirective(t *testing.T) {
	// A field literally named "dynamic" with type text must parse correctly.
	src := `
config
  database: "sqlite://app.db"
  port: 8080
  secret: "s"

model deal
  dynamic: text required
`
	app := parse(t, src)
	m := app.Models[0]
	if m.DynamicFields {
		t.Error("expected DynamicFields=false for regular field named 'dynamic'")
	}
	if len(m.Fields) != 1 || m.Fields[0].Name != "dynamic" {
		t.Errorf("expected field named 'dynamic', got %+v", m.Fields)
	}
}
