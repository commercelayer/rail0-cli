## rail0 webhooks list

List webhooks for the authenticated account

```
rail0 webhooks list [flags]
```

### Examples

```
  rail0 webhooks list --topic payments.captured --active true
```

### Options

```
      --active string          Filter by active flag ('true' or 'false')
      --circuit-state string   Filter by circuit state ('closed' or 'open')
  -h, --help                   help for list
      --page int               Page number (1-based)
      --per-page int           Results per page
      --sort string            Sort fields (e.g. -created_at)
      --topic string           Show subscriptions that INCLUDE this topic (e.g. payments.captured)
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

