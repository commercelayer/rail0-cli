## rail0 payouts destinations events

A destination's audit trail (its events and its payouts'), newest first

### Synopsis

List a payout destination's audit trail: its own events (created, minimum
changed — with the details confirmation recorded —, verified, disabled, …) AND those
of every payout made to it, newest first.

```
rail0 payouts destinations events <destination_id> [flags]
```

### Examples

```
  rail0 payouts destinations events <destination_id> --per-page 50
```

### Options

```
  -h, --help           help for events
      --page int       Page number (1-based)
      --per-page int   Events per page (max 100)
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

* [rail0 payouts destinations](rail0_payouts_destinations.md)	 - Manage where payouts go (lists when run bare)

