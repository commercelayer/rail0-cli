## rail0 account get

Show the account's profile (the same as 'rail0 account')

```
rail0 account get [flags]
```

### Examples

```
  rail0 account get
  rail0 account get --account 019e748b-…
```

### Options

```
  -h, --help   help for get
```

### Options inherited from parent commands

```
      --account string    Account id (default: the logged-in session account; another account's needs an admin session)
      --base-url string   RAIL0 gateway base URL (env: RAIL0_BASE_URL)
      --debug             Print HTTP request/response details to stderr
      --json              Force plain (uncolored) JSON output (useful in scripts)
      --pretty            Use the key-value/table view instead of JSON (terminal only)
  -q, --quiet             Print only the result's id/hash, one per line (useful in scripts)
```

### SEE ALSO

* [rail0 account](rail0_account.md)	 - Show or update the signed-in account's profile (shows when run bare)

