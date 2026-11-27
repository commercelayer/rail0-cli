## rail0 admin deliveries

Webhook deliveries across accounts: what a merchant was told, and whether it landed

### Synopsis

List webhook delivery attempts across every account, newest first, each with the
subscription it went to and that subscription's account.

Start from what is at hand: --payment (what was this payment's merchant told), --webhook
(what is this subscription failing on), or neither with --status failed for everything
that is not landing. --response-code tells a receiver's crashes (500) from its refusals
(401, 429). The gateway keeps 15 days of attempts.

The event body (payload) is left out: it is the whole event, several kilobytes a row.
--payload includes it.

```
rail0 admin deliveries [flags]
```

### Examples

```
  rail0 admin deliveries --status failed --since 2026-11-26T00:00:00Z
  rail0 admin deliveries --payment 0xrail0Id…
  rail0 admin deliveries --webhook 01a0f3c2-… --status failed --response-code 500
  rail0 admin deliveries --topic payments.captured --pretty
```

### Options

```
  -h, --help                   help for deliveries
      --page int               Page number (1-based)
      --payload                Include each delivery's event body
      --payment string         Only deliveries for this payment: its id (UUID) or its rail0_id (0x…)
      --per-page int           Results per page
      --response-code string   Only deliveries the receiver answered with exactly this HTTP status
      --since string           Only deliveries at/after this ISO-8601 time
      --sort string            Sort by created_at or status (prefix - for descending; default: newest first)
      --status string          Only deliveries that were delivered or failed
      --topic string           Only deliveries of this event topic
      --until string           Only deliveries at/before this ISO-8601 time
      --webhook string         Only this subscription's deliveries (webhook id)
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

* [rail0 admin](rail0_admin.md)	 - Operator commands: diagnostics, transactions, sync errors, webhooks, accounts, wallets, onboarding, audit, write mode

