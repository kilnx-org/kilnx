package parser

import "github.com/kilnx-org/kilnx/internal/spec"

func init() {
	spec.Register(spec.Entity{
		Name:    "webhook",
		Kind:    spec.KindKeyword,
		Summary: "Receive external events (Stripe, GitHub, etc.) at a path.",
		Description: "A `webhook` is an HTTP POST endpoint that authenticates requests via a " +
			"shared secret (read from an env var) and dispatches to an `on event <name>` " +
			"handler matching the event payload. Use this to process payment confirmations, " +
			"VCS pushes, and any other inbound integration.",
		Syntax: "webhook <path> [secret env <VAR>]",
		Args: []spec.Arg{
			{Name: "path", Type: "path", Required: true},
		},
		Children:   []string{"redirect"},
		Repeatable: true,
		Since:      "0.1.0",
		Examples: []spec.Example{
			{
				Title: "Handle Stripe payment confirmations",
				Code: `webhook /stripe/payment secret env STRIPE_SECRET
  on event payment_intent.succeeded
    query: UPDATE order SET status = 'paid' WHERE stripe_id = :event_id`,
			},
		},
		SeeAlso: []string{"on", "fetch"},
	})
}
