# Jobs and schedules

Some work should not block an HTTP response. Some work should run on a clock. Some work is triggered by an external system. Kilnx covers the three cases with [`job`](../reference/keywords/job.md), [`schedule`](../reference/keywords/schedule.md), and [`webhook`](../reference/keywords/webhook.md).

## Jobs: asynchronous work

A `job` is a named block that runs out-of-band. Pages and actions enqueue it; the runtime executes it on a worker.

```kilnx
job generate-report
  retry 3
  query: SELECT * FROM order WHERE created > :start_date
  send email to :requested_by
    subject: "Your report is ready"
```

`retry 3` tells the runtime to re-run the job up to three times if it fails. After exhausting retries, the job is marked failed and surfaced to the logs.

To trigger the job, use [`enqueue`](../reference/attributes/enqueue.md) inside any route or another job:

```kilnx
action /reports/request method POST requires auth
  enqueue generate-report
    start_date: :form.start_date
    requested_by: :current_user.email
  redirect /reports
```

The action returns immediately after enqueuing. The job runs in the background and the user is redirected.

Jobs are useful for slow operations: bulk emails, PDF generation, third-party API calls, report rollups. The pattern is "respond fast, finish later".

## Schedules: recurring work

A `schedule` runs on a cadence, set by [`every`](../reference/attributes/every.md). The argument is a duration or a cron expression.

```kilnx
schedule cleanup-sessions every 1h
  query: DELETE FROM session WHERE expires_at < now()
```

The runtime invokes the body once per interval. Common cadences:

- `every 30s`, `every 5m`, `every 1h`, `every 24h`: durations.
- `every "0 0 * * *"`: cron, runs at midnight.
- `every "*/15 * * * *"`: cron, every fifteen minutes.

Schedules can do anything a job can: run queries, send emails, fetch external APIs, or enqueue jobs themselves:

```kilnx
schedule daily-digest every "0 8 * * *"
  query users: SELECT id, email FROM user WHERE digest_opt_in = true
  enqueue send-digest
    user_ids: :users.id
```

This pattern (schedule fans out work to a job) keeps schedule bodies short and lets the job system handle retries per item.

Schedules execute in-process. If you run multiple replicas of the binary, every replica fires its own schedule. For distributed cron without duplicates, run one replica with schedules enabled, or use an external scheduler that calls a route.

## Webhooks: inbound events

A `webhook` is the inverse of a schedule: someone else calls you. The runtime verifies an HMAC signature against a shared secret read from an env var, then dispatches to a handler matching the event name.

```kilnx
webhook /stripe/payment secret env STRIPE_SECRET
  on event payment_intent.succeeded
    query: UPDATE order SET status = 'paid' WHERE stripe_id = :event_id

  on event payment_intent.failed
    query: UPDATE order SET status = 'failed' WHERE stripe_id = :event_id
```

The path is the URL the third party calls. The secret env var holds the signing key. Each `on event` matches a specific event name in the payload. Unrecognized events are ignored (or you can add a default handler).

Webhooks are the right tool for payment confirmations, VCS pushes, mail provider events, and anything else where the world tells your app something changed.

## Putting them together

A small order-processing flow uses all three:

```kilnx
model order
  amount: float required
  status: option [pending, paid, failed, fulfilled] default "pending"
  stripe_id: text unique
  created: timestamp auto

job fulfill-order
  retry 5
  query: UPDATE order SET status = 'fulfilled' WHERE id = :order_id
  send email to :customer_email
    subject: "Your order shipped"

webhook /stripe/payment secret env STRIPE_SECRET
  on event payment_intent.succeeded
    query: UPDATE order SET status = 'paid' WHERE stripe_id = :event_id
    enqueue fulfill-order
      order_id: :event.metadata.order_id
      customer_email: :event.metadata.email

schedule chase-pending every 6h
  query stale: SELECT id FROM order WHERE status = 'pending' AND created < now() - interval '24 hours'
  enqueue chase-customer
    order_ids: :stale.id
```

Stripe posts to the webhook. The webhook updates the order and enqueues fulfillment. The fulfillment job retries up to five times and emails the customer. A schedule chases stale orders every six hours.

## Read next

- [Auth and permissions](auth-and-permissions.md): webhooks bypass session auth, but routes that trigger jobs typically require it.
- [Reference](../reference/index.md): full attribute lists for `job`, `schedule`, and `webhook`.
