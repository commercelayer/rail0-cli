## rail0 analytics summary

Headline figures: orders, disputes, refund/dispute rates, volume

### Synopsis

Headline figures for the account: order count, disputed count, refund and
dispute rates, per-status counts, and volume.

Volume here is broken out per (token, chain), so it never hits the problem its
siblings warn about: each figure has a single unit behind it and is safe to read
directly. 'timeseries' and 'breakdown' return null volumes instead wherever a
row would span more than one token, since summing different tokens' base units
would produce a number that looks addable and is not.

```
rail0 analytics summary [flags]
```

### Examples

```
  rail0 analytics summary --chain arc
```

### Options

```
      --chain chain     Filter by chain: a slug (arc, base, …) or a chain id; the environment's gateway decides testnet vs mainnet
      --from string     Only payments created at/after this ISO-8601 time
  -h, --help            help for summary
      --mode string     Filter by mode: authorize or charge
      --payee string    Restrict to one of your account's wallets (0x address or @alias); default: the whole account
      --status string   Filter by payment status (e.g. captured)
      --to string       Only payments created at/before this ISO-8601 time
      --token string    Filter by token address (0x…)
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

* [rail0 analytics](rail0_analytics.md)	 - Account sales analytics: summary, timeseries, breakdown (requires login)

