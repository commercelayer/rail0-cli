## rail0 webhooks reset-circuit

Reset the delivery circuit breaker and re-enable the webhook

### Synopsis

Close the delivery circuit breaker and re-enable the webhook.

The breaker opens after 10 consecutive failed EVENTS (a distinct event, not
each retry of one), and opening it also marks the webhook disabled. Once open
it never closes on its own: a later success cannot close it, because nothing is
delivered while it is open. This command is the only way back, and it clears
the manual disable at the same time — so it works whichever way the webhook
went inactive.

Fix the endpoint FIRST. Resetting a breaker in front of a receiver that is
still failing just spends another 10 events reopening it. 'event-callbacks
<id> --status failed' shows what it was failing on.

```
rail0 webhooks reset-circuit <webhook_id> [flags]
```

### Examples

```
  rail0 webhooks reset-circuit whk_abc123
```

### Options

```
  -h, --help   help for reset-circuit
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

* [rail0 webhooks](rail0_webhooks.md)	 - Webhook subscription management (requires login)

