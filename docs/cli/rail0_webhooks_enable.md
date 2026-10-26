## rail0 webhooks enable

Re-enable a disabled webhook

### Synopsis

Turn a manually disabled webhook back on.

This clears ONLY the manual disable. If the delivery circuit breaker tripped,
the webhook stays inactive and no delivery goes out — the command still returns
200, so check 'active' in the response rather than the exit code. When it comes
back false with circuit_state "open", the fix is 'reset-circuit'.

```
rail0 webhooks enable <webhook_id> [flags]
```

### Examples

```
  rail0 webhooks enable whk_abc123
```

### Options

```
  -h, --help   help for enable
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

