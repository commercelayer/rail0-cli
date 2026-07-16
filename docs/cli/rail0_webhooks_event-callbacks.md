## rail0 webhooks event-callbacks

List delivery attempts for a webhook

```
rail0 webhooks event-callbacks <webhook_id> [flags]
```

### Examples

```
  rail0 webhooks event-callbacks whk_abc123 --status failed
  rail0 webhooks event-callbacks whk_abc123 --topic payments.captured
  rail0 webhooks event-callbacks whk_abc123 --payment-id 0x… --since 2026-01-01T00:00:00Z
```

### Options

```
  -h, --help                help for event-callbacks
      --payment-id string   Filter by the payment the delivery is for
      --since string        Only deliveries at/after this ISO-8601 time
      --sort string         Sort fields (e.g. -created_at)
      --status string       Filter by delivery status ('pending', 'delivered', 'failed')
      --topic string        Filter by event topic (e.g. payments.captured)
      --until string        Only deliveries at/before this ISO-8601 time
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

* [rail0 webhooks](rail0_webhooks.md)	 - Webhook subscription management (requires login)

