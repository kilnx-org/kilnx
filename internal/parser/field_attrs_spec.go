package parser

import "github.com/kilnx-org/kilnx/internal/spec"

// Field-level constraints applied inside a `model` block, e.g.:
//
//	model user
//	  email: email required unique
//	  age: int min 18 max 120
//	  role: option [admin, editor] default "editor"
func init() {
	spec.Register(spec.Entity{
		Name: "required", Kind: spec.KindAttribute,
		Summary:     "Field must have a non-null value.",
		Description: "Maps to a `NOT NULL` column in the database; also enforced by `validate`.",
		Syntax:      "<field>: <type> required",
		ParentScope: []string{"model"}, Since: "0.1.0",
		Examples: []spec.Example{{Title: "Required email", Code: `model user
  email: email required`}},
	})

	spec.Register(spec.Entity{
		Name: "unique", Kind: spec.KindAttribute,
		Summary: "Field value must be unique across all rows.",
		Description: "Single-field unique constraint. For composite uniqueness, use a model-level " +
			"`unique (field1, field2)` directive instead.",
		Syntax:      "<field>: <type> unique",
		ParentScope: []string{"model"}, Since: "0.1.0",
	})

	spec.Register(spec.Entity{
		Name: "auto", Kind: spec.KindAttribute,
		Summary: "Auto-populate the field on insert.",
		Description: "Behavior depends on field type: `uuid` generates a v4 UUID, `timestamp` sets " +
			"the current time, integer ID fields auto-increment.",
		Syntax:      "<field>: <type> auto",
		ParentScope: []string{"model"}, Since: "0.1.0",
	})

	spec.Register(spec.Entity{
		Name: "auto_update", Kind: spec.KindAttribute,
		Summary:     "Auto-populate on insert and update.",
		Description: "Typically used for `updated_at: timestamp auto_update` to track last-modified time.",
		Syntax:      "<field>: <type> auto_update",
		ParentScope: []string{"model"}, Since: "0.1.0",
	})

	spec.Register(spec.Entity{
		Name: "default", Kind: spec.KindAttribute,
		Summary: "Default value when no value is supplied.",
		Description: "Applied at the database level (`DEFAULT` clause). The literal must match " +
			"the field type.",
		Syntax:      "<field>: <type> default <value>",
		Args:        []spec.Arg{{Name: "value", Type: "literal", Required: true}},
		ParentScope: []string{"model"}, Since: "0.1.0",
	})

	spec.Register(spec.Entity{
		Name: "min", Kind: spec.KindAttribute,
		Summary: "Minimum value (numeric) or length (string).",
		Description: "Numeric fields enforce `value >= n`; string-like fields enforce length >= n. " +
			"Validation happens at `validate` time and at the runtime layer.",
		Syntax:      "<field>: <type> min <n>",
		Args:        []spec.Arg{{Name: "n", Type: "int", Required: true}},
		ParentScope: []string{"model"}, Since: "0.1.0",
	})

	spec.Register(spec.Entity{
		Name: "max", Kind: spec.KindAttribute,
		Summary:     "Maximum value (numeric) or length (string).",
		Description: "Mirror of `min`. For string fields, this also influences column size where applicable.",
		Syntax:      "<field>: <type> max <n>",
		Args:        []spec.Arg{{Name: "n", Type: "int", Required: true}},
		ParentScope: []string{"model"}, Since: "0.1.0",
	})

	spec.Register(spec.Entity{
		Name: "index", Kind: spec.KindAttribute,
		Summary:     "Non-unique composite index for query performance.",
		Description: "Declared at model scope. Creates a regular index covering the listed fields in order.",
		Syntax:      "index (<field>, ...)",
		ParentScope: []string{"model"}, Since: "0.1.0",
		Examples: []spec.Example{{Title: "Index on user_id + created_at", Code: `model order
  user_id: reference user
  created_at: timestamp auto
  index (user_id, created_at)`}},
	})

	spec.Register(spec.Entity{
		Name: "tenant", Kind: spec.KindAttribute,
		Summary: "Scope all rows of this model to a tenant.",
		Description: "Auto-synthesizes a required reference field to the tenant model and " +
			"transparently filters queries by the current tenant.",
		Syntax:      "tenant: <model>",
		Args:        []spec.Arg{{Name: "model", Type: "identifier", Required: true}},
		ParentScope: []string{"model"}, Since: "0.1.0",
	})

	spec.Register(spec.Entity{
		Name: "custom", Kind: spec.KindAttribute,
		Summary:     "Load custom field definitions from an external manifest.",
		Description: "Resolves at parse time from a path that may contain `{placeholder}` segments.",
		Syntax:      `custom "<path>"`,
		Args:        []spec.Arg{{Name: "path", Type: "string", Required: true}},
		ParentScope: []string{"model"}, Since: "0.1.0",
	})

	spec.Register(spec.Entity{
		Name: "dynamic_fields", Kind: spec.KindAttribute,
		Summary:     "Allow fields to be defined at runtime via the database.",
		Description: "Written `dynamic fields` (with a space) in source. Used for low-code apps where end users add custom columns.",
		Syntax:      "dynamic fields",
		ParentScope: []string{"model"}, Since: "0.1.0",
	})
}
