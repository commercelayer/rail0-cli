## rail0 analytics summary

Headline figures: orders, disputes, refund/dispute rates, volume

```
rail0 analytics summary [flags]
```

### Examples

```
  rail0 analytics summary --chain-id 5042002
```

### Options

```
      --chain-id int    Filter by EVM chain ID
      --from string     Only payments created at/after this ISO-8601 time
  -h, --help            help for summary
      --mode string     Filter by mode: authorize or charge
      --status string   Filter by payment status (e.g. captured)
      --to string       Only payments created at/before this ISO-8601 time
      --token string    Filter by token address (0x…)
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

