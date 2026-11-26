## rail0 admin webhooks

Every account's webhook subscriptions, with circuit and last attempt (lists when run bare)

### Synopsis

List the webhook subscriptions of every account: each with the account that owns it,
whether it is active, its circuit state and failure count, and when and how its newest
delivery attempt went. The open circuits come first.

A circuit opens after 10 consecutive failed events and never closes by itself: nothing is
delivered to that subscription until it is reset, by its merchant ('rail0 webhooks
reset-circuit') or here ('rail0 admin webhooks reset-circuit'). --circuit open lists the
subscriptions that are receiving nothing right now.

No signing key is shown: a shared secret is returned once, to the subscription's owner.

```
rail0 admin webhooks [flags]
```

### Examples

```
  rail0 admin webhooks --circuit open               # who is receiving nothing
  rail0 admin webhooks --account 019e748b-… --pretty
  rail0 admin webhooks --topic payments.captured --sort -circuit_failure_count
  rail0 admin deliveries --webhook 01a0f3c2-… --status failed   # what it was failing on
```

### Options

```
      --account string   Only this account's subscriptions (account id)
      --circuit string   Only subscriptions whose circuit is open or closed
  -h, --help             help for webhooks
      --page int         Page number (1-based)
      --per-page int     Results per page
      --sort string      Sort by created_at, updated_at, circuit_failure_count or name (prefix - for descending; default: open circuits first)
      --topic string     Only subscriptions that include this topic
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

* [rail0 admin](rail0_admin.md)	 - Operator commands: diagnostics, transactions, sync errors, webhooks, disputes, accounts, wallets, onboarding, operators, audit, write mode
* [rail0 admin webhooks reset-circuit](rail0_admin_webhooks_reset-circuit.md)	 - Close a subscription's circuit breaker and re-enable it, on any account (write mode, audited)

