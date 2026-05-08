package parser

import "github.com/kilnx-org/kilnx/internal/spec"

func init() {
	spec.Register(spec.Entity{
		Name:    "layout",
		Kind:    spec.KindKeyword,
		Summary: "Define a reusable HTML wrapper for pages.",
		Description: "A `layout` block declares a named HTML template that wraps the body of " +
			"any page that opts into it via `page <path> layout <name>`. The layout's " +
			"body usually contains `<html>`, `<head>`, and a placeholder for page content " +
			"(`{page.content}`). Layouts may also load shared data via `query` nodes.",
		Syntax: "layout <name>",
		Args: []spec.Arg{
			{Name: "name", Type: "identifier", Required: true},
		},
		Children:   []string{},
		Repeatable: true,
		Since:      "0.1.0",
		Examples: []spec.Example{
			{
				Title: "Define and use a layout",
				Code: `layout main
  html
    <html>
      <head><title>{page.title}</title></head>
      <body>{page.content}</body>
    </html>

page /dashboard layout main title "Dashboard"
  html
    <h1>Welcome</h1>`,
			},
		},
		SeeAlso: []string{"page", "html"},
	})
}
