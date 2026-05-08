package parser

import "github.com/kilnx-org/kilnx/internal/spec"

func init() {
	spec.Register(spec.Entity{
		Name:    "permissions",
		Kind:    spec.KindKeyword,
		Summary: "Role-based access control rules.",
		Description: "The `permissions` block declares per-role rules that drive authorization " +
			"checks. Rules are written in a small DSL: `all`, `read <resource>`, " +
			"`write <resource>`, optionally with `where <expression>` clauses that reference " +
			"`current_user` or row attributes. Routes opt in by writing `requires <role>`.",
		Syntax:     "permissions",
		Repeatable: true,
		Since:      "0.1.0",
		Examples: []spec.Example{
			{
				Title: "Three-role hierarchy",
				Code: `permissions
  admin: all
  editor: read post, write post where author = current_user
  viewer: read post where status = published`,
			},
		},
		SeeAlso: []string{"auth", "requires"},
	})
}
