## rail0 webhooks update

Update a webhook's name, callback URL, or topics

### Synopsis

Change a webhook's name, callback URL, or topics in place. The id and the shared
secret are kept, so the receiver keeps verifying with the secret it already has.

--topic REPLACES the whole set, which is also how a topic is REMOVED: pass the
ones you want to keep. Adding one means passing the existing topics plus the new
one, so read them first with 'webhooks get <id>'.

This does NOT change whether the webhook is active. Repointing a broken webhook
at a URL that works leaves it exactly as inactive as it was — which is the
second half of the repair sequence worth knowing:

  1. 'webhooks event-callbacks <id> --status failed'   what was it failing on
  2. 'webhooks update <id> --callback-url …'           fix it
  3. 'webhooks reset-circuit <id>'                     let it deliver again
  4. 'webhooks redeliver <id> <callback_id>'           replay what was lost

Step 3 is not optional after a tripped breaker, and doing it before step 2 just
spends another 10 events reopening it. Step 4 replays one failed attempt's
payload per call; skipping step 3 makes the replay drop silently.

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
  -t, --topic strings         REPLACE the topic set; repeat for several. Omitting a topic removes it
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

