## rail0 webhooks update

Update a webhook's name, callback URL, or topic

```
rail0 webhooks update <webhook_id> [flags]
```

### Examples

```
  rail0 webhooks update whk_abc123 --callback-url https://example.com/new
```

### Options

```
  -u, --callback-url string   New callback URL
  -h, --help                  help for update
  -n, --name string           New webhook name
  -t, --topic string          New topic
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

