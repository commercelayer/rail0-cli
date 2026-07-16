## rail0 payments wait

Block until a payment settles (or reaches a given status)

### Synopsis

Poll a payment until it settles, then print its final state.

By default it waits until no transaction is still in flight (pending/submitted).
With --until it instead waits for a specific payment status.

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
      --timeout duration   Max time to wait (default 2m0s)
      --until string       Wait until the payment reaches this status (e.g. captured, refunded)
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

