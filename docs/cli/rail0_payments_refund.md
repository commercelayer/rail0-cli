## rail0 payments refund

Refund a payment (prepare, sign, and broadcast)

### Synopsis

Prepare the transaction that calls refund() on the RAIL0 contract,
pulling refundAmount from the payee and returning it to the payer.

The payment must be in 'charged', 'captured', or 'partially_captured' status
with a positive refundableAmount. Partial refunds are supported: pass --amount
less than refundableAmount.

No separate approval step is needed: the CLI signs the payee's EIP-3009
ReceiveWithAuthorization payload locally with the same key, so the refund
funds are pulled from the payee without a prior allowance grant.

The API returns 202 immediately: broadcasting is not settling. Add -w/--wait to
block until the operation confirms on-chain (or fails) and exit non-zero if it
didn't land.

Note that the payment STATUS advances to "refunded" only when the payment is
fully settled (both escrow and refundable drained); a partial refund leaves it
unchanged. This does not weaken --wait: it waits on THIS operation's transaction
reaching confirmed or failed, not on a status change — so it settles a partial
refund correctly, where waiting for a new status would hang until the deadline.

Pass --private-key (-p): the CLI signs locally and broadcasts in one step.

```
rail0 payments refund <rail0Id> [flags]
```

### Examples

```
  # Full refund
  rail0 payments refund 0xrail0Id… --amount 1.00 -p @merchant-key --wait

  # Partial refund
  rail0 payments refund 0xrail0Id… --amount 0.25 -p @merchant-key --wait
```

### Options

```
  -a, --amount string            Amount to refund as a decimal in token units (e.g. 1.00); > 0 and ≤ refundable
  -h, --help                     help for refund
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

