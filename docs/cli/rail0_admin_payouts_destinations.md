## rail0 admin payouts destinations

Every account's payout destinations, read-only (lists when run bare)

### Synopsis

List the payout destinations of every account: each with its account, wallet, chain,
token, address, label and status (unverified, verified, disabled), who registered it and
who confirmed it.

Read-only: every payout write is the destination wallet's own, and the gateway holds no
funds. With the payouts feature off on this gateway the command answers 404
('rail0 admin config payouts' says whether it is on).

```
rail0 admin payouts destinations [flags]
```

### Examples

```
  rail0 admin payouts destinations --status unverified --pretty
  rail0 admin payouts destinations --account 019e748b-…
  rail0 admin payouts destinations get 01a3e9b1-…
  rail0 admin payouts destinations events 01a3e9b1-…
```

### Options

```
      --account string   Only this account's destinations (account id)
      --chain chain      Only destinations on this chain: a slug (arc, base, …) or a chain id; the environment's gateway decides testnet vs mainnet
  -h, --help             help for destinations
      --page int         Page number (1-based)
      --per-page int     Destinations per page
      --sort string      Sort by created_at, updated_at, label or status (prefix - for descending)
      --status string    Filter by status: unverified, verified, disabled
      --wallet string    Only this wallet's destinations (wallet id)
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
* [rail0 admin payouts destinations events](rail0_admin_payouts_destinations_events.md)	 - A destination's audit trail, newest first: its own events and its payouts'
* [rail0 admin payouts destinations get](rail0_admin_payouts_destinations_get.md)	 - One payout destination of any account: the whole record

