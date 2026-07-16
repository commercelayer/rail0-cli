## rail0 payments transactions

List on-chain transactions for a payment

```
rail0 payments transactions <rail0Id> [flags]
```

### Examples

```
  rail0 payments transactions 0xabc123…
  rail0 payments transactions 0xabc123… --sort -submitted_at
```

### Options

```
  -h, --help           help for transactions
      --page int       Page number (1-based)
      --per-page int   Results per page
      --sort string    Sort fields (e.g. -submitted_at,operation)
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

* [rail0 payments](rail0_payments.md)	 - Payment lifecycle operations

