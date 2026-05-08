package parser

import "github.com/kilnx-org/kilnx/internal/spec"

func init() {
	spec.Register(spec.Entity{
		Name:    "job",
		Kind:    spec.KindKeyword,
		Summary: "Asynchronous background task triggered by `enqueue`.",
		Description: "A `job` runs asynchronously when called via `enqueue <job>`. Jobs are " +
			"useful for slow operations (sending email batches, generating reports) that " +
			"should not block HTTP responses. Failed jobs retry up to `retry <n>` times.",
		Syntax: "job <name>",
		Args: []spec.Arg{
			{Name: "name", Type: "identifier", Required: true},
		},
		Children:   []string{"retry", "redirect"},
		Repeatable: true,
		Since:      "0.1.0",
		Examples: []spec.Example{
			{
				Title: "Generate and email a report",
				Code: `job generate-report
  retry 3
  query: SELECT * FROM order WHERE created > :start_date
  send email to :requested_by
    subject: "Your report is ready"`,
			},
		},
		SeeAlso: []string{"schedule", "enqueue"},
	})

	spec.Register(spec.Entity{
		Name: "retry", Kind: spec.KindAttribute,
		Summary:     "Maximum retry attempts before the job is marked failed.",
		Syntax:      "retry <n>",
		Args:        []spec.Arg{{Name: "n", Type: "int", Required: true}},
		ParentScope: []string{"job"}, Default: "0", Since: "0.1.0",
	})
}
