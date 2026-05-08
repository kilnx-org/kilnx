package parser

import "github.com/kilnx-org/kilnx/internal/spec"

func init() {
	spec.Register(spec.Entity{
		Name:    "schedule",
		Kind:    spec.KindKeyword,
		Summary: "Background task executed on a fixed interval or cron expression.",
		Description: "The runtime invokes the schedule's body at the cadence declared by " +
			"`every`. Bodies typically run cleanup queries, send periodic emails, or " +
			"enqueue jobs. Schedules execute in-process; for distributed setups, see " +
			"`webhook` or external schedulers.",
		Syntax: "schedule <name> every <duration|cron>",
		Args: []spec.Arg{
			{Name: "name", Type: "identifier", Required: true},
			{Name: "duration", Type: "string", Required: true},
		},
		Children:   []string{"every", "redirect"},
		Repeatable: true,
		Since:      "0.1.0",
		Examples: []spec.Example{
			{
				Title: "Hourly session cleanup",
				Code: `schedule cleanup-sessions every 1h
  query: DELETE FROM session WHERE expires_at < now()`,
			},
		},
		SeeAlso: []string{"job", "stream"},
	})
}
