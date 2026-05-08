package parser

import "github.com/kilnx-org/kilnx/internal/spec"

func init() {
	spec.Register(spec.Entity{
		Name:    "action",
		Kind:    spec.KindKeyword,
		Summary: "Declare a state-changing endpoint (POST/PUT/DELETE).",
		Description: "An `action` is a non-GET endpoint that executes side effects: writing " +
			"to the database, sending email, enqueuing jobs, or redirecting. Unlike `page`, " +
			"actions do not return a full HTML document by default; they typically end with " +
			"a `redirect` or a `respond` (for htmx fragments).",
		Syntax: "action <path> [method <verb>] [requires <clause>]",
		Args: []spec.Arg{
			{Name: "path", Type: "path", Required: true},
		},
		Children:   []string{"method", "requires", "redirect"},
		Repeatable: true,
		Since:      "0.1.0",
		Examples: []spec.Example{
			{
				Title: "Create a user, then redirect",
				Code: `action /users/create method POST requires auth
  validate user
  query: INSERT INTO user (email, name) VALUES (:email, :name)
  redirect /users`,
			},
		},
		SeeAlso: []string{"page", "fragment", "api"},
	})
}
