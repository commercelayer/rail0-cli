## rail0 webhooks

Webhook subscription management (requires login)

### Synopsis

Manage webhook subscriptions for the authenticated account.

A webhook subscribes to exactly one payment-lifecycle topic (e.g.
payments.captured) and delivers a signed callback to your endpoint. The
shared secret used to verify delivery signatures is returned only once, on
create and rotate-secret.

### Options

```
  -h, --help   help for webhooks
```

### Options inherited from parent commands

```
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
* [rail0 webhooks reset-circuit](rail0_webhooks_reset-circuit.md)	 - Reset the delivery circuit breaker and re-enable the webhook
* [rail0 webhooks rotate-secret](rail0_webhooks_rotate-secret.md)	 - Generate a new shared secret (returned once on the response)
* [rail0 webhooks update](rail0_webhooks_update.md)	 - Update a webhook's name, callback URL, or topic

