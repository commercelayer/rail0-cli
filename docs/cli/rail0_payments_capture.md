## rail0 payments capture

Capture a payment (prepare, sign, and broadcast)

### Synopsis

Prepare the transaction that calls capture() on the RAIL0 contract.

The payment must be in 'authorized' or 'partially_captured' status.
Partial captures are supported: pass --amount less than capturableAmount
to capture a portion; the remaining balance stays available until
authorizationExpiry.
The API returns 202 immediately. The payment stays at "authorized" until
rail0-indexer confirms and the API transitions it to "captured" or
"partially_captured". Poll 'payments get <rail0Id>' until the payment
reaches the expected state and the capture transaction shows confirmed.

Pass --private-key (-p): the CLI signs locally and broadcasts in one step.

```
rail0 payments capture <rail0Id> [flags]
```

### Examples

```
  # Full capture
  rail0 payments capture 0xrail0Id… --amount 1.00 -p 0xPayeeKey…

  # Partial capture
  rail0 payments capture 0xrail0Id… --amount 0.50 -p 0xPayeeKey…
```

### Options

```
  -a, --amount string        Amount to capture as a decimal in token units (e.g. 1.00); > 0 and ≤ capturable
  -h, --help                 help for capture
  -p, --private-key string   Caller's secp256k1 private key (0x-prefixed hex); signs and broadcasts in one step
      --timeout duration     Max time to wait when --wait is set (default 2m0s)
  -w, --wait                 Block until the operation confirms on-chain (or fails)
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

