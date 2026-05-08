# `webhook`

> Receive external events (Stripe, GitHub, etc.) at a path.

| | |
|---|---|
| **Kind** | Keyword |
| **Since** | `0.1.0` |
| **Repeatable** | yes |

## Syntax

```
webhook <path> [secret env <VAR>]
```

## Arguments

| Name | Type | Required |
|------|------|----------|
| `path` | `path` | yes |

## Description

A `webhook` is an HTTP POST endpoint that authenticates requests via a shared secret (read from an env var) and dispatches to an `on event <name>` handler matching the event payload. Use this to process payment confirmations, VCS pushes, and any other inbound integration.

## Children

- [`redirect`](../attributes/redirect.md)

## Examples

### Handle Stripe payment confirmations

```kilnx
webhook /stripe/payment secret env STRIPE_SECRET
  on event payment_intent.succeeded
    query: UPDATE order SET status = 'paid' WHERE stripe_id = :event_id
```

## See also

- [`on`](../attributes/on.md)
- [`fetch`](../attributes/fetch.md)

## Provenance

| | |
|---|---|
| **Spec last touched** | `5da8498` (2026-05-08) |
| **Source last touched** | `5da8498` (2026-05-08) |
| **Source files** | `internal/parser/parser.go` |

