## rail0 admin webhooks reset-circuit

Close a subscription's circuit breaker and re-enable it, on any account (write mode, audited)

### Synopsis

Close the delivery circuit breaker of a subscription and re-enable it, whoever it
belongs to: the same write as the merchant's 'rail0 webhooks reset-circuit'.

It re-arms delivery to an endpoint the operator does not control, so check first that
the endpoint is back: 'rail0 admin deliveries --webhook <webhook> --status failed' shows
what it was failing on. Resetting a breaker in front of a receiver that still fails only
spends another 10 events reopening it. Events that fired while the circuit was open were
not delivered, and the reset does not send them.

The gateway audits it with the callback URL. No confirmation is asked: nothing is lost,
and a subscription that fails again opens its circuit again.

A write: it needs write mode ('rail0 admin write-mode on').

```
rail0 admin webhooks reset-circuit <webhook> [flags]
```

### Examples

```
  rail0 admin webhooks reset-circuit 01a0f3c2-…
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

* [rail0 admin webhooks](rail0_admin_webhooks.md)	 - Every account's webhook subscriptions, with circuit and last attempt (lists when run bare)

