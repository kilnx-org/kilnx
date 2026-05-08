package parser

import "github.com/kilnx-org/kilnx/internal/spec"

func init() {
	spec.Register(spec.Entity{
		Name:    "fragment",
		Kind:    spec.KindKeyword,
		Summary: "Return partial HTML (no document wrapper) for htmx or includes.",
		Description: "A `fragment` returns a slice of HTML without `<html>`/`<body>` " +
			"wrapping. Fragments come in two flavors: route-based (`fragment <path>`), " +
			"used as htmx targets that respond to AJAX requests, and component-based " +
			"(`fragment <name>(<args>)`), used as reusable template includes invoked from " +
			"other fragments or pages.",
		Syntax: "fragment <path-or-name> [(<args>)] [requires <clause>]",
		Args: []spec.Arg{
			{Name: "path-or-name", Type: "string", Required: true},
		},
		Children:   []string{"requires", "redirect"},
		Repeatable: true,
		Since:      "0.1.0",
		Examples: []spec.Example{
			{
				Title: "Route-based htmx fragment",
				Code: `fragment /users/:id/card
  query user: SELECT name, email FROM user WHERE id = :id
  html
    <div class="card">{user.name}</div>`,
			},
			{
				Title: "Reusable component",
				Code: `fragment badge(status, color="blue")
  html
    <span class="{color}">{status}</span>`,
			},
		},
		SeeAlso: []string{"page", "action", "html"},
	})
}
