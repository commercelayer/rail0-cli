## rail0 payments history

Show a payment's chronological history

### Synopsis

Print the timeline of everything that happened to a payment since it was
created: the created and signed milestones and every on-chain transaction
(authorize/capture/void/release/refund/dispute/close_dispute), oldest first.
Only the most significant fields are shown per entry (time, event, status,
amount, tx hash, block).

The argument is a UUID or a rail0_id (0x…, 64 hex). Requires login — the
timeline is readable only by the payment's payer or payee.

```
rail0 payments history <rail0Id> [flags]
```

### Examples

```
  rail0 payments history 0xabc123…
  rail0 payments history 019e748b-…
```

### Options

```
  -h, --help   help for history
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

