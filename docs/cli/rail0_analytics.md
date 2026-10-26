## rail0 analytics

Account sales analytics: summary, timeseries, breakdown (requires login)

### Synopsis

Read the account's sales analytics.

  summary     — headline figures: order count, disputed count, refund and dispute
                rates, per-status counts, and per-token volume
  timeseries  — order counts bucketed by day, week or month
  breakdown   — the same counts grouped by token, chain, mode, status or operation

All three take the same filters (--mode, --status, --token, --chain-id, --from,
--to, --payee). By default the figures cover the whole account — every wallet it
owns; --payee restricts them to ONE of the account's wallets (a 0x address or an
@alias). A wallet the account does not own is refused with 403.

Requires an account: a wallet-only session gets 403 account_required, since
these are the MERCHANT's figures, not an address's.

### Examples

```
  rail0 analytics summary
  rail0 analytics summary --from 2026-01-01T00:00:00Z
  rail0 analytics timeseries --interval week
  rail0 analytics breakdown --by token
  rail0 analytics summary --payee @shop-eu
```

### Options

```
  -h, --help   help for analytics
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

* [rail0](rail0.md)	 - CLI for the RAIL0 stablecoin payment protocol
* [rail0 analytics breakdown](rail0_analytics_breakdown.md)	 - Group the counts by token, chain, mode, status or operation (--by is required)
* [rail0 analytics summary](rail0_analytics_summary.md)	 - Headline figures: orders, disputes, refund/dispute rates, volume
* [rail0 analytics timeseries](rail0_analytics_timeseries.md)	 - Order counts bucketed by day, week or month

