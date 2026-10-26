## rail0 payments capture

Capture a payment (prepare, sign, and broadcast)

### Synopsis

Prepare the transaction that calls capture() on the RAIL0 contract.

The payment must be in 'authorized' or 'partially_captured' status.
Partial captures are supported: pass --amount less than capturableAmount
to capture a portion; the remaining balance stays available until
authorizationExpiry.

The API returns 202 immediately: broadcasting is not settling. The payment
stays at "authorized" until rail0-indexer sees the on-chain event and the API
transitions it to "captured" or "partially_captured".

Add -w/--wait to block until that happens (or the operation fails) and exit
non-zero if it didn't land. On partial captures it is close to required:
capturableAmount is the LAST CONFIRMED balance, so it reads stale while a
capture of yours is still in flight, and picking the next amount from it is how
you over- or under-capture. ('payments wait' settles it separately if you'd
rather not block the capture itself.)

Pass --private-key (-p): the CLI signs locally and broadcasts in one step.

```
rail0 payments capture <rail0Id> [flags]
```

### Examples

```
  # Full capture, waiting for the chain to confirm it
  rail0 payments capture 0xrail0Id… --amount 1.00 -p @merchant-key --wait

  # Partial capture — --wait settles the balance before the next one
  rail0 payments capture 0xrail0Id… --amount 0.50 -p @merchant-key --wait
```

### Options

```
  -a, --amount string            Amount to capture as a decimal in token units (e.g. 1.00); > 0 and ≤ the last confirmed capturable
  -h, --help                     help for capture
      --idempotency-key string   Make a re-run safe: the same key returns the first transaction instead of opening a second
  -p, --private-key string       Caller's secp256k1 private key (0x-prefixed hex); signs and broadcasts in one step
      --timeout duration         Max time to wait when --wait is set; unset, 10× the chain's measured p90 settlement, kept within 5m–15m (default 15m0s)
  -w, --wait                     Block until the operation confirms on-chain (or fails)
  -y, --yes                      Skip the confirmation prompt (required to run non-interactively)
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

