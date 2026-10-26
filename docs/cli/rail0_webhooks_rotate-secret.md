## rail0 webhooks rotate-secret

Generate a new shared secret (returned once on the response)

### Synopsis

Issue a new shared secret for a webhook, returned once on the response.

The old secret stops verifying immediately — there is no overlap window. Any
delivery already in flight was signed with the old one, so a receiver that has
not yet been updated will reject it and the delivery counts as failed, which
feeds the circuit breaker. Update the receiver first where you can, or expect a
short window of failures.

Rotate on suspected exposure, and after anyone who had the secret loses
access.

With -q/--quiet the command prints just the new secret.

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
  -y, --yes    Skip the confirmation prompt (required to run non-interactively)
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

