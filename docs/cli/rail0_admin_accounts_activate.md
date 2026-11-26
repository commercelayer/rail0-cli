## rail0 admin accounts activate

Switch a deactivated account back on (write mode, audited)

### Synopsis

Switch a deactivated account back on: it can start payments and write payouts again.
No confirmation is asked: 'rail0 admin accounts deactivate' reverses it.

A write: it needs write mode ('rail0 admin write-mode on').

```
rail0 admin accounts activate <account> [flags]
```

### Examples

```
  rail0 admin accounts activate 019e748b-…
```

### Options

```
  -h, --help   help for activate
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

