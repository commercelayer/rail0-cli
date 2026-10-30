## rail0 webhooks

Webhook subscription management (requires login)

### Synopsis

Manage webhook subscriptions for the authenticated account.

A webhook subscribes to one or more payment-lifecycle topics and delivers a
signed callback to your endpoint. One subscription means ONE shared secret and
ONE circuit breaker however many topics it carries, and each delivery names the
event that fired in X-Rail0-Topic — so four events on one endpoint is one
subscription, not four to keep in step. The secret is returned only once, on
create and rotate-secret.

Two subscriptions for the same callback URL must not overlap: the same event
arriving twice under two different secrets is indistinguishable from a duplicate
at the receiving end, so the gateway refuses it and names the topic that
collided.

A webhook delivers only when it is active, and there are TWO independent ways
for it to stop being active:

  disabled       you turned it off              → 'enable' brings it back
  circuit open   10 consecutive failed events   → 'reset-circuit', and
                 tripped the breaker              nothing else, brings it back

They are not interchangeable. A tripped breaker also marks the webhook
disabled, so 'enable' appears to work — it returns 200 — while the webhook
stays silent, because the breaker is still open. Read 'active' in the response
rather than trusting the exit code: it is false whenever either switch is off,
and 'circuit_state' says which.

Topics (-t/--topic completes them):

  payments.created, payments.signed, payments.authorized, payments.charged,
  payments.captured, payments.voided, payments.released, payments.refunded,
  payments.expired, payments.failed, payments.disputed, payments.dispute_closed
                                   a payment's lifecycle, one event per transition
  payments.authorization_expiring  a WARNING, not a transition: fires once, while
                                   there is still time to capture, when an
                                   authorization with capturable funds is about to
                                   lapse (inside the gateway's
                                   AUTHORIZATION_EXPIRING_NOTICE window, 24h by
                                   default). The status does not change; a window
                                   that already closed sends payments.expired.
  payouts.confirmed                a payout's transfer was mined with the expected
                                   ERC-20 Transfer
  payouts.failed                   a payout failed — its error_code says how
                                   (reverted, dropped, cancelled, …)
  payout_destinations.verified     a payout destination was confirmed

### Examples

```
  rail0 webhooks list
  rail0 webhooks create --name "order hook" \
    --callback-url https://example.com/hook -t payments.captured -t payments.refunded
  rail0 webhooks event-callbacks whk_abc123 --status failed   # why isn't it arriving?
  rail0 webhooks redeliver whk_abc123 <callback_id>           # replay a lost event
```

### Options

```
  -h, --help   help for webhooks
```

### Options inherited from parent commands

```
      --agent             Agent mode: one JSON envelope on stdout, JSON events on stderr, no prompts (env: RAIL0_AGENT=1)
      --base-url string   RAIL0 gateway base URL (env: RAIL0_BASE_URL)
      --debug             Print HTTP request/response details to stderr
      --json              Force plain (uncolored) JSON output (useful in scripts)
      --pretty            Use the key-value/table view instead of JSON (terminal only)
  -q, --quiet             Print only the result's id/hash, one per line (useful in scripts)
```

### SEE ALSO

* [rail0](rail0.md)	 - CLI for the RAIL0 stablecoin payment protocol
* [rail0 webhooks create](rail0_webhooks_create.md)	 - Register a new webhook (returns the one-time shared secret)
* [rail0 webhooks delete](rail0_webhooks_delete.md)	 - Remove a webhook
* [rail0 webhooks disable](rail0_webhooks_disable.md)	 - Disable a webhook
* [rail0 webhooks enable](rail0_webhooks_enable.md)	 - Re-enable a disabled webhook
* [rail0 webhooks event-callbacks](rail0_webhooks_event-callbacks.md)	 - List delivery attempts for a webhook
* [rail0 webhooks get](rail0_webhooks_get.md)	 - Fetch a single webhook
* [rail0 webhooks list](rail0_webhooks_list.md)	 - List webhooks for the authenticated account
* [rail0 webhooks redeliver](rail0_webhooks_redeliver.md)	 - Replay one recorded delivery attempt's exact payload
* [rail0 webhooks reset-circuit](rail0_webhooks_reset-circuit.md)	 - Reset the delivery circuit breaker and re-enable the webhook
* [rail0 webhooks rotate-secret](rail0_webhooks_rotate-secret.md)	 - Generate a new shared secret (returned once on the response)
* [rail0 webhooks test](rail0_webhooks_test.md)	 - Send a test event to the webhook's endpoint and show what it answered
* [rail0 webhooks update](rail0_webhooks_update.md)	 - Update a webhook's name, callback URL, or topics

