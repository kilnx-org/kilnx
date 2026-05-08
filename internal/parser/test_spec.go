package parser

import "github.com/kilnx-org/kilnx/internal/spec"

func init() {
	spec.Register(spec.Entity{
		Name:    "test",
		Kind:    spec.KindKeyword,
		Summary: "End-to-end browser-style test scenario.",
		Description: "A `test` block declares a sequence of steps that drive the application " +
			"like a user: visiting URLs, filling form fields, submitting, and asserting the " +
			"result. Tests run via `kilnx test <file.kilnx>` and fail the run if any " +
			"`expect` step does not hold.",
		Syntax: `test "<name>"`,
		Args: []spec.Arg{
			{Name: "name", Type: "string", Required: true},
		},
		Children:   []string{"visit", "fill", "submit", "expect", "as"},
		Repeatable: true,
		Since:      "0.1.0",
		Examples: []spec.Example{
			{
				Title: "Create a user via the form",
				Code: `test "Create user"
  visit /users/new
  fill name "John"
  submit
  expect page /users contains "John"`,
			},
		},
	})

	for _, e := range []spec.Entity{
		{Name: "visit", Summary: "Navigate to a URL.", Syntax: "visit <path>"},
		{Name: "fill", Summary: "Fill a form field with a value.", Syntax: `fill <name> "<value>"`},
		{Name: "submit", Summary: "Submit the current form.", Syntax: "submit"},
		{Name: "expect", Summary: "Assert a condition on the current state.", Syntax: "expect <assertion>"},
		{Name: "as", Summary: "Run subsequent steps as a given role or user.", Syntax: "as <role>"},
	} {
		e.Kind = spec.KindAttribute
		e.ParentScope = []string{"test"}
		e.Since = "0.1.0"
		spec.Register(e)
	}
}
