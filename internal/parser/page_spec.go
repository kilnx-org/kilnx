package parser

import "github.com/kilnx-org/kilnx/internal/spec"

// Spec entries for the `page` keyword and its attributes are co-located
// here so the documentation lives next to the parser logic that
// implements them. Edits here are picked up by cmd/kilnx-gendocs via
// `go generate`.

func init() {
	spec.Register(spec.Entity{
		Name:    "page",
		Kind:    spec.KindKeyword,
		Summary: "Define an HTTP route and its view.",
		Description: "A `page` declares a URL path that the Kilnx runtime serves over " +
			"HTTP. Its body contains the rendered view (HTML, fragments) and " +
			"optional data-loading directives (queries, redirects, validations).",
		Syntax: "page <path> [layout <name>] [title <text>] [requires <clause>] [method <verb>]",
		Args: []spec.Arg{
			{Name: "path", Type: "path", Required: true},
		},
		Children:   []string{"method", "requires", "layout", "title", "redirect"},
		Repeatable: true,
		Since:      "0.1.0",
		Examples: []spec.Example{
			{
				Title: "Hello world",
				Code: `page /
  html
    <h1>Hello World</h1>`,
			},
			{
				Title: "Authenticated page with layout and title",
				Code: `page /dashboard layout app title "Dashboard" requires auth
  query @user_count: SELECT count(*) FROM users
  html
    <h1>{{ .user_count }} users</h1>`,
			},
		},
		SeeAlso: []string{"action", "fragment", "api", "layout"},
	})
}
