package parser

import "github.com/kilnx-org/kilnx/internal/spec"

func init() {
	spec.Register(spec.Entity{
		Name:    "log",
		Kind:    spec.KindKeyword,
		Summary: "Configure runtime logging output.",
		Description: "The `log` block sets log verbosity, slow-query threshold, request log " +
			"level, and error reporting (with optional stack traces). Logs are emitted in a " +
			"structured format suitable for ingestion by stdout collectors.",
		Syntax:   "log",
		Children: []string{"level", "slow_query", "requests", "errors"},
		Since:    "0.1.0",
		Examples: []spec.Example{
			{
				Title: "Verbose dev logging",
				Code: `log
  level: debug
  slow-query: 100ms
  requests: all
  errors: all stacktrace`,
			},
		},
	})

	for _, e := range []spec.Entity{
		{Name: "level", Summary: "Minimum severity to log (debug, info, warn, error).", Syntax: "level: <severity>", Default: "info"},
		{Name: "slow_query", Summary: "Threshold above which a SQL query is logged as slow.", Syntax: "slow-query: <duration>"},
		{
			Name:        "requests",
			Summary:     "Rate-limit budget (in `limit`) or request log strategy (in `log`).",
			Description: "Inside a `log` block: which requests to log. One of `all`, `errors`, or `none`.",
			Syntax:      "requests: <strategy>",
		},
		{Name: "errors", Summary: "Error reporting strategy. Append `stacktrace` to include stacks.", Syntax: "errors: <strategy> [stacktrace]"},
	} {
		e.Kind = spec.KindAttribute
		e.ParentScope = []string{"log"}
		e.Since = "0.1.0"
		spec.Register(e)
	}
}
