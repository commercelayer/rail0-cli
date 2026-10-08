## rail0 disputes reasons

List the dispute reasons: the codes --reason accepts

### Synopsis

List the dispute-reason dictionary: the codes 'rail0 payments dispute' and
'rail0 payments dispute close' accept for --reason, with their descriptions.

There are two lists. A dispute is OPENED with an 'open' reason and CLOSED with a
'close' reason; 'other' is in both. The 'system' reason (full_refund) is never
chosen: the protocol records it when a full refund closes an open dispute, and it
is listed so a dispute's close_reason_code can be read.

On-chain each reason is the bytes32 keccak256("rail0.dispute.<code>"); --json (or
piped output) adds it to every row. The list is the one this CLI was built with —
the gateway's is authoritative.

```
rail0 disputes reasons [flags]
```

### Examples

```
  rail0 disputes reasons
  rail0 disputes reasons --kind close
  rail0 disputes reasons --json
```

### Options

```
  -h, --help          help for reasons
      --kind string   Only one list: open, close or system
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

* [rail0 disputes](rail0_disputes.md)	 - List every dispute on your payments (requires login)

