## rail0 keys address

Show the EVM address derived from a saved key (the address, never the key)

```
rail0 keys address <name> [flags]
```

### Examples

```
  rail0 keys address payer
```

### Options

```
  -h, --help   help for address
```

### Options inherited from parent commands

```
      --agent             Agent mode: one JSON envelope on stdout, JSON events on stderr, no prompts (env: RAIL0_AGENT=1)
      --base-url string   RAIL0 gateway base URL (env: RAIL0_BASE_URL)
      --debug             Print HTTP request/response details to stderr
      --json              Force plain (uncolored) JSON output (useful in scripts)
      --pretty            Use the key-value/table view instead of JSON (terminal only)
  -q, --quiet             Print only the result's id/hash, one per line (useful in scripts)
```

### SEE ALSO

* [rail0 keys](rail0_keys.md)	 - Generate signing keys locally and manage them in the OS keychain

