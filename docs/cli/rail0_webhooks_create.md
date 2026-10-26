## rail0 webhooks create

Register a new webhook (returns the one-time shared secret)

### Synopsis

Register a webhook for one or more topics on the logged-in account.

Repeat -t/--topic for each event, or pass them comma-separated. They share one
secret and one circuit breaker, which is the point: a per-topic subscription
means a key to verify against and a rotation to coordinate for each one.

The response carries the shared SECRET, and it is the only time it is shown:
the gateway stores a hash, not the value, so it cannot be read back. Capture it
now — 'rotate-secret' issues a new one, which is the recovery, not a lookup.

With -q/--quiet the command prints the secret rather than the id, since the id can
be looked up again ('webhooks list') and the secret cannot.

An account holds at most 20 webhooks (the gateway's WEBHOOKS_PER_ACCOUNT default).
Past the limit the create is refused with 422 webhook_limit_reached. Disabled
webhooks count toward it, deleted ones do not: delete one ('webhooks delete'), or
add topics to an existing webhook ('webhooks update') instead.

Use the secret to verify the X-Rail0-Signature header on each delivery. A
delivery whose signature you cannot verify should be treated as untrusted, not
as a malformed request.

```
rail0 webhooks create [flags]
```

### Examples

```
  rail0 webhooks create --name "order lifecycle" \
    --callback-url https://example.com/hook \
    -t payments.authorized -t payments.captured -t payments.voided -t payments.refunded

  # warned before an uncaptured authorization lapses
  rail0 webhooks create --name "capture reminder" \
    --callback-url https://example.com/hook -t payments.authorization_expiring
```

### Options

```
  -u, --callback-url string   HTTPS endpoint that receives deliveries
  -h, --help                  help for create
  -n, --name string           Human-readable webhook name
  -t, --topic strings         Topic to subscribe to; repeat for several (e.g. -t payments.captured -t payments.refunded)
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

