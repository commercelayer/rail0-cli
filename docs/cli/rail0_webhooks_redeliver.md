## rail0 webhooks redeliver

Replay one recorded delivery attempt's exact payload

### Synopsis

Replay a recorded delivery attempt — the way to recover events that were lost
while a webhook was failing.

Every attempt in 'webhooks event-callbacks' stores the payload it sent, and a
redelivery sends THAT payload verbatim: the same embedded event id, so a
receiver that already processed it can deduplicate, under a fresh timestamped
signature signed with the webhook's current secret. <callback_id> is the id of
a row from 'webhooks event-callbacks <webhook_id>'.

The webhook must be active for the replay to go out. A disabled webhook, or one
whose circuit breaker is open, drops the replay silently like any other
delivery — so run 'webhooks enable <id>' or 'webhooks reset-circuit <id>' first
(reset-circuit covers both).

Delivery is asynchronous: the gateway answers 202 with status "queued" once the
replay is accepted, not once it has arrived. Check 'webhooks event-callbacks
<webhook_id>' for the new attempt to see the outcome. A 404 means the webhook or
callback is not this account's, or the attempt was recorded before payloads
were stored — there is nothing to replay. If the gateway's job queue is down the
replay cannot be queued and the command fails with nothing recorded: retry later.

```
rail0 webhooks redeliver <webhook_id> <callback_id> [flags]
```

### Examples

```
  rail0 webhooks event-callbacks whk_abc123 --status failed   # pick the callback id
  rail0 webhooks reset-circuit whk_abc123                      # make sure it can deliver
  rail0 webhooks redeliver whk_abc123 7f1c0c8e-1d2b-4c3a-9e8f-0123456789ab
```

### Options

```
  -h, --help   help for redeliver
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

