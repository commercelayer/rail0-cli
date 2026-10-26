## rail0 analytics breakdown

Group the counts by token, chain, mode, status or operation (--by is required)

### Synopsis

Group the account's orders by one dimension.

--by is required: token, chain, mode, status or operation. token and chain rows
carry volume (they identify a scale); mode and status rows are counts only,
because a group spanning several tokens has no single unit to add up in.
operation groups the account's own CONFIRMED transactions — how many captures,
refunds, disputes… actually landed — and carries a transaction count beside the
order count.

```
rail0 analytics breakdown [flags]
```

### Examples

```
  rail0 analytics breakdown --by token
  rail0 analytics breakdown --by status --mode charge
  rail0 analytics breakdown --by operation
```

### Options

```
      --by string       Group by: token, chain, mode, status or operation (required)
      --chain-id int    Filter by EVM chain ID
      --from string     Only payments created at/after this ISO-8601 time
  -h, --help            help for breakdown
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

