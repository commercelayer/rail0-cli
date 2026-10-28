## rail0 webhooks test

Send a test event to the webhook's endpoint and show what it answered

### Synopsis

Send one test event to the webhook's callback URL now, and print what the
endpoint did: delivered (a 2xx), the status code and the start of its response
body — or, when no response came, why (a refused address, TLS, a timeout).

The event has topic "webhooks.test", "test": true and no payment, and is signed
and delivered exactly like a real one, so it checks reachability, TLS and your
signature verification without waiting for a real payment. It is not logged in
'event-callbacks', does not count toward the circuit breaker, and works on a
disabled webhook too — test the endpoint before re-enabling it.

```
rail0 webhooks test <webhook_id> [flags]
```

### Examples

```
  rail0 webhooks test whk_abc123
```

### Options

```
  -h, --help   help for test
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

