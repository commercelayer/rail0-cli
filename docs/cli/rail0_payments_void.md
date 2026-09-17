## rail0 payments void

Void a payment (prepare, sign, and broadcast)

### Synopsis

Prepare the transaction that calls void() on the RAIL0 contract.

The payment must be in 'authorized' status with NOTHING captured yet: void is
forbidden once any (even partial) capture has occurred (the contract reverts
AlreadyCaptured) — recover the uncaptured remainder via 'release' instead. Void
cancels the authorization and returns the full escrowed balance to the payer.
Only callable before authorizationExpiry, by the payee.

The API returns 202 immediately: broadcasting is not settling. Add -w/--wait to
block until the operation confirms on-chain (or fails) and exit non-zero if it
didn't land, instead of polling 'payments get <rail0Id>' yourself.

Pass --private-key (-p): the CLI signs locally and broadcasts in one step.

```
rail0 payments void <rail0Id> [flags]
```

### Examples

```
  rail0 payments void 0xrail0Id… -p @merchant-key --wait
```

### Options

```
  -h, --help                     help for void
      --idempotency-key string   Make a re-run safe: the same key returns the first transaction instead of opening a second
  -p, --private-key string       Caller's secp256k1 private key (0x-prefixed hex); signs and broadcasts in one step
      --timeout duration         Max time to wait when --wait is set — the default covers the slowest chain's finality (default 15m0s)
  -w, --wait                     Block until the operation confirms on-chain (or fails)
  -y, --yes                      Skip the confirmation prompt (required to run non-interactively)
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

