## rail0 wallets update

Update label or active status on a wallet (by id or 0x address)

```
rail0 wallets update <wallet> [flags]
```

### Examples

```
  rail0 wallets update 0xABCD… --label "New label" --active true
```

### Options

```
      --active string   Set active flag ('true' or 'false')
  -h, --help            help for update
  -l, --label string    New wallet label
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

* [rail0 wallets](rail0_wallets.md)	 - Manage the logged-in account's wallets (lists when run bare)

