## rail0 payments wait

Block until a payment settles (or reaches a given status)

### Synopsis

Poll a payment until it settles, then print its final state.

By default it waits until no transaction is still in flight (submitted, or signed
and pending broadcast). A prepare that was never signed is not in flight — nothing
will ever send it — so it does not hold the wait open.
With --until it instead waits for a specific payment status, and stops with an
error as soon as the payment can no longer reach it — a charge-mode payment never
becomes 'captured', a captured one never becomes 'authorized' again.

```
rail0 payments wait <rail0Id> [flags]
```

### Examples

```
  rail0 payments wait 0xrail0Id…
  rail0 payments wait 0xrail0Id… --until captured
```

### Options

```
  -h, --help               help for wait
      --timeout duration   Max time to wait — the default covers the slowest chain's finality (default 15m0s)
      --until string       Wait until the payment reaches this status (unsigned, signed, authorized, partially_captured, captured, charged, voided, released, refunded, expired)
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

* [rail0 payments](rail0_payments.md)	 - Payment lifecycle operations

