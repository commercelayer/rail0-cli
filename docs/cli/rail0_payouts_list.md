## rail0 payouts list

List the account's payouts, newest first

### Synopsis

List the account's payouts, newest first. --status takes several values, repeated
or comma-separated, and matches any of them.

```
rail0 payouts list [flags]
```

### Examples

```
  rail0 payouts list --destination <destination_id>
  rail0 payouts list --status failed --created-from 2026-10-01T00:00:00Z
```

### Options

```
      --created-from string   Only payouts created at/after this ISO-8601 time
      --created-to string     Only payouts created at/before this ISO-8601 time
      --destination string    Only payouts to this destination (destination id)
  -h, --help                  help for list
      --page int              Page number (1-based)
      --per-page int          Payouts per page (max 100)
      --purpose string        Filter by purpose: payout or proof
      --sort string           Sort fields (e.g. -created_at)
      --status strings        Filter by status (pending, submitted, confirmed, failed); repeat or comma-separate for any of several
      --wallet string         Only payouts from this wallet (wallet id)
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

* [rail0 payouts](rail0_payouts.md)	 - Send a wallet's tokens to your own venue (lists when run bare; requires login)

