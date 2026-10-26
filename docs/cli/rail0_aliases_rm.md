## rail0 aliases rm

Remove an address alias

```
rail0 aliases rm <name> [flags]
```

### Examples

```
  rail0 aliases rm payee
```

### Options

```
  -h, --help   help for rm
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

* [rail0 aliases](rail0_aliases.md)	 - Address book — list saved addresses, or manage them with add/rm

