## rail0 webhooks rotate-secret

Generate a new shared secret (returned once on the response)

```
rail0 webhooks rotate-secret <webhook_id> [flags]
```

### Examples

```
  rail0 webhooks rotate-secret whk_abc123
```

### Options

```
  -h, --help   help for rotate-secret
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

