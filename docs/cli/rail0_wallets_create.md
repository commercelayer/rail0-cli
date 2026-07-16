## rail0 wallets create

Register a new EVM wallet address on the account

```
rail0 wallets create [flags]
```

### Examples

```
  rail0 wallets create --address 0xABCD… --label "Main wallet"
```

### Options

```
  -A, --address string   EVM wallet address (0x-prefixed hex, or @alias)
  -h, --help             help for create
  -l, --label string     Human-readable wallet label
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

