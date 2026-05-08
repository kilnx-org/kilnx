package parser

import "github.com/kilnx-org/kilnx/internal/spec"

func init() {
	spec.Register(spec.Entity{
		Name:    "query",
		Kind:    spec.KindKeyword,
		Summary: "Run a SQL query and bind its result. Used both top-level (named query) and inside bodies.",
		Description: "Top-level `query <name>: <SQL>` declares a reusable named query that " +
			"other pages, actions, or fragments may reference. Inside a page/action/" +
			"fragment/api/job/schedule body, `query [<name>]: <SQL>` runs the SQL and " +
			"binds rows to a template variable. Add `paginate <n>` to paginate. Parameters " +
			"are referenced via `:name` (URL/form values) or `:current_user_id` (session).",
		Syntax: "query [<name>]: <SQL>",
		Args: []spec.Arg{
			{Name: "name", Type: "identifier", Required: false},
			{Name: "SQL", Type: "string", Required: true},
		},
		Repeatable: true,
		Since:      "0.1.0",
		Examples: []spec.Example{
			{
				Title: "Named top-level query",
				Code: `query active_users:
  SELECT id, name, email FROM user WHERE status = 'active'`,
			},
			{
				Title: "Inline query inside a page body",
				Code: `page /users
  query users: SELECT id, name FROM user ORDER BY id paginate 20
  html
    {#each users}
      <li>{name}</li>
    {/each}`,
			},
		},
		SeeAlso: []string{"page", "action", "fragment"},
	})
}
