## rail0 tokens

List supported tokens (symbols used in --token)

```
rail0 tokens [flags]
```

### Examples

```
  rail0 tokens
  rail0 tokens --chain-id 5042002
  rail0 tokens --symbol USDC
```

### Options

```
  -c, --chain-id int    Filter by EVM chain ID (e.g. 5042002)
  -h, --help            help for tokens
  -s, --symbol string   Filter by token symbol (case-insensitive, e.g. USDC)
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

* [rail0](rail0.md)	 - CLI for the RAIL0 stablecoin payment protocol

