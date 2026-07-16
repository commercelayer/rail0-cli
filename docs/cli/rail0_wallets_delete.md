## rail0 wallets delete

Remove a wallet from the account (by id or 0x address)

```
rail0 wallets delete <wallet> [flags]
```

### Examples

```
  rail0 wallets delete 0xABCD…
```

### Options

```
  -h, --help   help for delete
  -y, --yes    Skip the confirmation prompt (required to run non-interactively)
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

