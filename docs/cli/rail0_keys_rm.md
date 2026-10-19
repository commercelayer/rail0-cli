## rail0 keys rm

Remove a saved signing key from the OS keychain

```
rail0 keys rm <name> [flags]
```

### Examples

```
  rail0 keys rm payer
```

### Options

```
  -h, --help   help for rm
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

* [rail0 keys](rail0_keys.md)	 - Generate signing keys locally and manage them in the OS keychain

