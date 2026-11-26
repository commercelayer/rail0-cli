## rail0 admin payouts get

One payout of any account: the whole record but its signed payload

### Synopsis

Read-only: every payout write is the destination wallet's own, and the gateway holds no
funds. With the payouts feature off on this gateway the command answers 404
('rail0 admin config payouts' says whether it is on).

```
rail0 admin payouts get <payout> [flags]
```

### Examples

```
  rail0 admin payouts get 01a3f0c2-…
```

### Options

```
  -h, --help   help for get
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

* [rail0 admin payouts](rail0_admin_payouts.md)	 - Every account's payouts, read-only (lists when run bare)

