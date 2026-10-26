## rail0 payments disputes

List a payment's dispute open/close history

```
rail0 payments disputes <rail0Id> [flags]
```

### Examples

```
  rail0 payments disputes 0xrail0Id…
  rail0 payments disputes 0xrail0Id… --status open
```

### Options

```
  -h, --help            help for disputes
      --page int        Page number (1-based)
      --per-page int    Results per page
      --sort string     Sort fields (e.g. -opened_at)
      --status string   Filter by dispute status: open or closed
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

* [rail0 payments](rail0_payments.md)	 - Payment lifecycle operations

