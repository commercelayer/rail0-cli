## rail0 keys add

Save a signing key in the OS keychain (prompts for the key; names must be unique)

```
rail0 keys add <name> [flags]
```

### Examples

```
  rail0 keys add payer
```

### Options

```
  -h, --help   help for add
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

* [rail0 keys](rail0_keys.md)	 - Manage signing keys in the OS keychain (lists names when run bare)

