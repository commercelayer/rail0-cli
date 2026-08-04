## rail0 config get

Show a setting's effective value and origin

```
rail0 config get <key> [flags]
```

### Examples

```
  rail0 config get base-url
```

### Options

```
  -h, --help   help for get
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

* [rail0 config](rail0_config.md)	 - Manage persisted CLI defaults (base-url, chain-id, token, output, http-timeout)

