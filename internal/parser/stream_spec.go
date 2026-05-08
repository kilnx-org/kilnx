package parser

import "github.com/kilnx-org/kilnx/internal/spec"

func init() {
	spec.Register(spec.Entity{
		Name:    "stream",
		Kind:    spec.KindKeyword,
		Summary: "Server-Sent Events (SSE) endpoint that pushes data on an interval.",
		Description: "A `stream` is an HTTP endpoint that holds the connection open and " +
			"pushes events to the client at the cadence given by `every`. On each tick the " +
			"declared `query` runs and its results are emitted as the named `event`. Useful " +
			"for live counters, notification feeds, and polling dashboards without writing " +
			"client-side JavaScript.",
		Syntax: "stream <path> [requires <clause>]",
		Args: []spec.Arg{
			{Name: "path", Type: "path", Required: true},
		},
		Children:   []string{"every", "event", "requires"},
		Repeatable: true,
		Since:      "0.1.0",
		Examples: []spec.Example{
			{
				Title: "Notifications feed",
				Code: `stream /notifications requires auth
  query: SELECT message, created_at FROM notifications WHERE seen = false
  every 5s
  event: message`,
			},
		},
		SeeAlso: []string{"socket", "page"},
	})

	spec.Register(spec.Entity{
		Name: "every", Kind: spec.KindAttribute,
		Summary: "Cadence at which the parent task runs.",
		Description: "Accepts a duration like `5s`, `1m`, `1h`, `24h` or a cron expression " +
			"(in `schedule`). For `stream`, controls how often the query is executed and " +
			"pushed to clients.",
		Syntax:      "every <duration|cron>",
		Args:        []spec.Arg{{Name: "duration", Type: "string", Required: true}},
		ParentScope: []string{"stream", "schedule"}, Since: "0.1.0",
	})

	spec.Register(spec.Entity{
		Name: "event", Kind: spec.KindAttribute,
		Summary:     "SSE event name to emit on each tick.",
		Syntax:      "event: <name>",
		Args:        []spec.Arg{{Name: "name", Type: "identifier", Required: true}},
		ParentScope: []string{"stream"}, Since: "0.1.0",
	})
}
