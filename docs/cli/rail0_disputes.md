## rail0 disputes

List every dispute on your payments (requires login)

### Synopsis

List the disputes across the payments your wallet is party to, as payer or
payee, each with its parent payment embedded.

Open AND closed: a closed dispute stops setting the 'disputed' flag on
'payments list' (which reports current state) but is still listed here, so the
history stays visible. Filter with --status to see only one of the two.

Distinct from 'rail0 payments disputes <rail0Id>', which is the open/close history
of ONE payment.

```
rail0 disputes [flags]
```

### Examples

```
  rail0 disputes
  rail0 disputes --status open
  rail0 disputes --sort -opened_at --per-page 50
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
      --base-url string   RAIL0 gateway base URL (env: RAIL0_BASE_URL)
      --debug             Print HTTP request/response details to stderr
      --json              Force plain (uncolored) JSON output (useful in scripts)
      --pretty            Use the key-value/table view instead of JSON (terminal only)
  -q, --quiet             Print only the result's id/hash, one per line (useful in scripts)
```

### SEE ALSO

* [rail0](rail0.md)	 - CLI for the RAIL0 stablecoin payment protocol
* [rail0 disputes reasons](rail0_disputes_reasons.md)	 - List the dispute reasons: the codes --reason accepts

