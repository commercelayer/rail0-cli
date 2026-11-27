## rail0 admin payouts

Every account's payouts, read-only (lists when run bare)

### Synopsis

List the payouts of every account, newest first: each with its account, destination,
amount, status and, when it failed, why (error_code, error_reason).

Read-only: every payout write is the destination wallet's own, and the gateway holds no
funds. With the payouts feature off on this gateway the command answers 404
('rail0 admin config payouts' says whether it is on).

```
rail0 admin payouts [flags]
```

### Examples

```
  rail0 admin payouts --status failed --pretty
  rail0 admin payouts --account 019e748b-… --created-from 2026-11-01T00:00:00Z
  rail0 admin payouts get 01a3f0c2-…
  rail0 admin payouts events 01a3f0c2-…
  rail0 admin payouts destinations --status unverified
```

### Options

```
      --account string        Only this account's payouts (account id)
      --chain chain           Only payouts on this chain: a slug (arc, base, …) or a chain id; the environment's gateway decides testnet vs mainnet
      --created-from string   Only payouts created at/after this ISO-8601 time
      --created-to string     Only payouts created at/before this ISO-8601 time
      --destination string    Only payouts to this destination (destination id)
  -h, --help                  help for payouts
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

* [rail0 admin](rail0_admin.md)	 - Operator commands: diagnostics, transactions, sync errors, webhooks, disputes, accounts, wallets, onboarding, payouts, operators, audit, configuration, catalogue, write mode
* [rail0 admin payouts destinations](rail0_admin_payouts_destinations.md)	 - Every account's payout destinations, read-only (lists when run bare)
* [rail0 admin payouts events](rail0_admin_payouts_events.md)	 - A payout's audit trail, newest first
* [rail0 admin payouts get](rail0_admin_payouts_get.md)	 - One payout of any account: the whole record but its signed payload

