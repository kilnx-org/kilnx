package parser

import "github.com/kilnx-org/kilnx/internal/spec"

func init() {
	spec.Register(spec.Entity{
		Name:    "limit",
		Kind:    spec.KindKeyword,
		Summary: "Rate-limit requests matching a path pattern.",
		Description: "A `limit` block applies a rate limit to all routes matching the " +
			"pattern. Limits are scoped per `user`, per `ip`, or globally. Exceeding the " +
			"budget returns HTTP 429 with the configured `message` (or a default).",
		Syntax: "limit <path-pattern>",
		Args: []spec.Arg{
			{Name: "path-pattern", Type: "string", Required: true},
		},
		Children:   []string{"requests", "delay", "message"},
		Repeatable: true,
		Since:      "0.1.0",
		Examples: []spec.Example{
			{
				Title: "Throttle the API per user",
				Code: `limit /api/*
  requests: 100 per minute per user
  delay: 10
  message: "Slow down."`,
			},
		},
		SeeAlso: []string{"requires"},
	})

	spec.Register(spec.Entity{
		Name: "requests", Kind: spec.KindAttribute,
		Summary: "Rate-limit budget (in `limit`) or request log strategy (in `log`).",
		Description: "Inside a `limit` block: maximum number of allowed requests per " +
			"window per scope, e.g. `requests: 100 per minute per user`. Excess requests " +
			"return HTTP 429 (or are delayed if `delay` is set).",
		Syntax:      "requests: <n> per <window> per <scope>",
		Args:        []spec.Arg{{Name: "n", Type: "int", Required: true}},
		ParentScope: []string{"limit"}, Required: true, Since: "0.1.0",
	})

	spec.Register(spec.Entity{
		Name: "delay", Kind: spec.KindAttribute,
		Summary:     "Seconds to delay over-budget requests before responding.",
		Description: "If set, requests over the limit are held for `delay` seconds before responding. Useful as a soft limit.",
		Syntax:      "delay: <seconds>",
		Args:        []spec.Arg{{Name: "seconds", Type: "int", Required: true}},
		ParentScope: []string{"limit"}, Since: "0.1.0",
	})

	spec.Register(spec.Entity{
		Name: "message", Kind: spec.KindAttribute,
		Summary:     "Custom error message returned when the limit is exceeded.",
		Syntax:      `message: "<text>"`,
		Args:        []spec.Arg{{Name: "text", Type: "string", Required: true}},
		ParentScope: []string{"limit"}, Since: "0.1.0",
	})
}
