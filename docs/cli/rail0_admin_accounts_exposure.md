## rail0 admin accounts exposure

What an account still has in motion: what a deactivation would leave half-way

### Synopsis

What an account still has in motion, as a payee:

  in_flight   payments created or signed and not yet on chain, authorization window open
  in_escrow   buyer funds held by the RAIL0 contract, not yet captured
  disputed    open disputes
  payouts     payouts prepared (pending) or sent and not yet settled (submitted)

by_token totals the payments per chain and token. A payment can be in two categories at
once (a partial capture with escrow left and a dispute on the captured part), so the
three counts do not add up to a number of payments; by_token's count does.

Nothing of this is lost by a deactivation: escrow always returns to the buyer by void or
release, and the wallet is the merchant's. It is what 'rail0 admin accounts deactivate'
reads and shows before it asks.

```
rail0 admin accounts exposure <account> [flags]
```

### Examples

```
  rail0 admin accounts exposure 019e748b-…
  rail0 admin accounts exposure 019e748b-… --pretty
```

### Options

```
  -h, --help   help for exposure
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

* [rail0 admin accounts](rail0_admin_accounts.md)	 - Every account: find one, read it, switch it off or on (lists when run bare)

