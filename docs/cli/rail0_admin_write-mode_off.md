## rail0 admin write-mode off

Switch write mode off: revoke its token and remove it

```
rail0 admin write-mode off [flags]
```

### Options

```
  -h, --help   help for off
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

* [rail0 admin write-mode](rail0_admin_write-mode.md)	 - Operator write mode: see it, switch it on or off (shows it when run bare)

