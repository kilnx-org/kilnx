package parser

import "github.com/kilnx-org/kilnx/internal/spec"

func init() {
	spec.Register(spec.Entity{
		Name:    "model",
		Kind:    spec.KindKeyword,
		Summary: "Declare a database table with typed fields and constraints.",
		Description: "A `model` declares a persistent entity backed by a SQL table. The " +
			"runtime auto-migrates the schema: adding fields, indexes, and unique " +
			"constraints triggers `ALTER TABLE` on next start. Field declarations use " +
			"`name: <type> [constraint...]`. Built-in types include `text`, `email`, " +
			"`int`, `float`, `bool`, `timestamp`, `date`, `uuid`, `password`, `image`, " +
			"`file`, `json`, `reference <model>`, `option [a, b, c]`, `tags`, and others.",
		Syntax: "model <name>",
		Args: []spec.Arg{
			{Name: "name", Type: "identifier", Required: true},
		},
		Children:   []string{"required", "unique", "auto", "auto_update", "default", "min", "max", "index", "tenant", "custom", "dynamic_fields"},
		Repeatable: true,
		Since:      "0.1.0",
		Examples: []spec.Example{
			{
				Title: "User model with constraints",
				Code: `model user
  email: email required unique
  name: text required
  role: option [admin, editor, viewer] default "viewer"
  age: int min 18 max 120
  created_at: timestamp auto`,
			},
			{
				Title: "Multi-tenant model",
				Code: `model invoice
  tenant: account
  amount: decimal required
  status: option [draft, sent, paid] default "draft"
  index (tenant, status)`,
			},
		},
		SeeAlso: []string{"query", "auth"},
	})
}
