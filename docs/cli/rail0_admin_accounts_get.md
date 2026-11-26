## rail0 admin accounts get

One account's whole record: profile, status, operator grant

```
rail0 admin accounts get <account> [flags]
```

### Examples

```
  rail0 admin accounts get 019e748b-…
  rail0 admin accounts get 019e748b-… --pretty
```

### Options

```
  -h, --help   help for get
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

* [rail0 admin accounts](rail0_admin_accounts.md)	 - Every account: find one, read it, switch it off or on (lists when run bare)

