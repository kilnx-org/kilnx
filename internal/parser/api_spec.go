package parser

import "github.com/kilnx-org/kilnx/internal/spec"

func init() {
	spec.Register(spec.Entity{
		Name:    "api",
		Kind:    spec.KindKeyword,
		Summary: "Declare a JSON endpoint.",
		Description: "An `api` block defines a JSON-returning HTTP endpoint. Query results " +
			"and computed values are serialized to JSON automatically. Use `api` for " +
			"machine-to-machine integrations or SPA backends; use `page` for HTML, `action` " +
			"for state-changing flows, and `fragment` for htmx.",
		Syntax: "api <path> [method <verb>] [requires <clause>]",
		Args: []spec.Arg{
			{Name: "path", Type: "path", Required: true},
		},
		Children:   []string{"method", "requires", "redirect"},
		Repeatable: true,
		Since:      "0.1.0",
		Examples: []spec.Example{
			{
				Title: "List users as JSON",
				Code: `api /api/v1/users requires auth
  query users: SELECT id, name, email FROM user ORDER BY id`,
			},
		},
		SeeAlso: []string{"page", "action", "fragment"},
	})
}
