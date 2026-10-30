## rail0 payouts destinations list

List the account's payout destinations (every wallet's)

```
rail0 payouts destinations list [flags]
```

### Examples

```
  rail0 payouts destinations list --status verified
  rail0 payouts destinations list --wallet <wallet_id>
```

### Options

```
  -h, --help            help for list
      --page int        Page number (1-based)
      --per-page int    Destinations per page
      --sort string     Sort fields (e.g. -created_at)
      --status string   Filter by status: unverified, verified, disabled
      --wallet string   Only this wallet's destinations (wallet id)
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

