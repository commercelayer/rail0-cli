## rail0 config unset

Remove a setting from the config file

```
rail0 config unset <key> [flags]
```

### Examples

```
  rail0 config unset base-url
```

### Options

```
  -h, --help   help for unset
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

* [rail0 config](rail0_config.md)	 - Manage persisted CLI defaults (base-url, chain, token, output, http-timeout)

