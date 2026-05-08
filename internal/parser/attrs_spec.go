package parser

import "github.com/kilnx-org/kilnx/internal/spec"

// Spec entries for attributes shared across multiple keywords. Attributes
// exclusive to a single keyword should live in that keyword's _spec.go
// (e.g. identity/password/login belong in auth_spec.go when introduced).

func init() {
	spec.Register(spec.Entity{
		Name:        "method",
		Kind:        spec.KindAttribute,
		Summary:     "Restrict the HTTP method for the parent route.",
		Description: "Specifies which HTTP verb the parent endpoint accepts. When omitted, defaults to `GET`.",
		Syntax:      "method <verb>",
		Args: []spec.Arg{
			{Name: "verb", Type: "string", Required: true},
		},
		ParentScope: []string{"page", "action", "api"},
		Default:     "GET",
		Since:       "0.1.0",
		Examples: []spec.Example{
			{
				Title: "POST page",
				Code: `page /contact method POST
  html
    <p>Form submitted.</p>`,
			},
		},
		SeeAlso: []string{"requires"},
	})

	spec.Register(spec.Entity{
		Name:        "requires",
		Kind:        spec.KindAttribute,
		Summary:     "Require authentication or a specific role/permission.",
		Description: "Gates the parent endpoint behind authentication. Accepts `auth` (any logged-in user), a role name, or a permission expression.",
		Syntax:      "requires <clause>",
		Args: []spec.Arg{
			{Name: "clause", Type: "identifier", Required: true},
		},
		ParentScope: []string{"page", "action", "api"},
		Repeatable:  true,
		Since:       "0.1.0",
		Examples: []spec.Example{
			{
				Title: "Require any logged-in user",
				Code:  `page /me requires auth`,
			},
			{
				Title: "Require a specific role",
				Code:  `page /admin requires admin`,
			},
		},
		SeeAlso: []string{"method", "permissions"},
	})

	spec.Register(spec.Entity{
		Name:        "layout",
		Kind:        spec.KindAttribute,
		Summary:     "Render the page inside a named layout.",
		Description: "References a top-level `layout` block by name. The page's HTML is rendered inside the layout's `{{ content }}` slot.",
		Syntax:      "layout <name>",
		Args: []spec.Arg{
			{Name: "name", Type: "identifier", Required: true},
		},
		ParentScope: []string{"page"},
		Since:       "0.1.0",
		Examples: []spec.Example{
			{
				Title: "Use the app layout",
				Code: `page /dashboard layout app
  html
    <h1>Dashboard</h1>`,
			},
		},
		SeeAlso: []string{"title"},
	})

	spec.Register(spec.Entity{
		Name:        "title",
		Kind:        spec.KindAttribute,
		Summary:     "Set the HTML document title.",
		Description: "Sets the `<title>` tag for the rendered page. Accepts a quoted string or bare identifier.",
		Syntax:      `title "<text>"`,
		Args: []spec.Arg{
			{Name: "text", Type: "string", Required: true},
		},
		ParentScope: []string{"page"},
		Since:       "0.1.0",
		Examples: []spec.Example{
			{
				Title: "Page with title",
				Code: `page / title "Home"
  html
    <h1>Welcome</h1>`,
			},
		},
		SeeAlso: []string{"layout"},
	})

	spec.Register(spec.Entity{
		Name:        "redirect",
		Kind:        spec.KindAttribute,
		Summary:     "Redirect to another URL.",
		Description: "Issues an HTTP redirect from inside a page or action body. Used in flows that change state and then send the user elsewhere.",
		Syntax:      "redirect <path>",
		Args: []spec.Arg{
			{Name: "path", Type: "path", Required: true},
		},
		ParentScope: []string{"page", "action"},
		Since:       "0.1.0",
		Examples: []spec.Example{
			{
				Title: "Redirect after successful action",
				Code: `action /logout method POST
  redirect /entrar`,
			},
		},
		SeeAlso: []string{"action"},
	})
}
