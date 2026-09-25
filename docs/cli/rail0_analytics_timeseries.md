## rail0 analytics timeseries

Order counts bucketed by day, week or month

### Synopsis

Order counts over time, one row per bucket.

Each row carries a volume only when a SINGLE token and chain are filtered: summing
different tokens' base units would be meaningless, so the gateway returns null
instead of a number that looks addable.

```
rail0 analytics timeseries [flags]
```

### Examples

```
  rail0 analytics timeseries
  rail0 analytics timeseries --interval month --chain-id 5042002 --token 0x3600…
```

### Options

```
      --chain-id int      Filter by EVM chain ID
      --from string       Only payments created at/after this ISO-8601 time
  -h, --help              help for timeseries
      --interval string   Bucket width: day (default), week or month
      --mode string       Filter by mode: authorize or charge
      --payee string      Restrict to one of your account's wallets (0x address or @alias); default: the whole account
      --status string     Filter by payment status (e.g. captured)
      --to string         Only payments created at/before this ISO-8601 time
      --token string      Filter by token address (0x…)
```

### Options inherited from parent commands

```
      --base-url string   RAIL0 gateway base URL (env: RAIL0_BASE_URL)
      --debug             Print HTTP request/response details to stderr
      --json              Force plain (uncolored) JSON output (useful in scripts)
      --pretty            Use the key-value/table view instead of JSON (terminal only)
  -q, --quiet             Print only the result's id/hash, one per line (useful in scripts)
```

### SEE ALSO

* [rail0 analytics](rail0_analytics.md)	 - Account sales analytics: summary, timeseries, breakdown (requires login)

