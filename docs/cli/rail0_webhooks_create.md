## rail0 webhooks create

Register a new webhook (returns the one-time shared secret)

```
rail0 webhooks create [flags]
```

### Examples

```
  rail0 webhooks create --name "capture hook" \
    --callback-url https://example.com/hook --topic payments.captured
```

### Options

```
  -u, --callback-url string   HTTPS endpoint that receives deliveries
  -h, --help                  help for create
  -n, --name string           Human-readable webhook name
  -t, --topic string          Topic to subscribe to (e.g. payments.captured)
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

