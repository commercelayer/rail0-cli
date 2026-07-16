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
The API returns 202 immediately; poll 'payments get <rail0Id>'. The status
advances to "refunded" only when the payment is fully settled (both escrow and
refundable drained); a partial refund leaves the status unchanged.

Pass --private-key (-p): the CLI signs locally and broadcasts in one step.

```
rail0 payments refund <rail0Id> [flags]
```

### Examples

```
  # Full refund
  rail0 payments refund 0xrail0Id… --amount 1.00 -p 0xPayeeKey…

  # Partial refund
  rail0 payments refund 0xrail0Id… --amount 0.25 -p 0xPayeeKey…
```

### Options

```
  -a, --amount string        Amount to refund as a decimal in token units (e.g. 1.00); > 0 and ≤ refundable
  -h, --help                 help for refund
  -p, --private-key string   Caller's secp256k1 private key (0x-prefixed hex); signs and broadcasts in one step
      --timeout duration     Max time to wait when --wait is set (default 2m0s)
  -w, --wait                 Block until the operation confirms on-chain (or fails)
  -y, --yes                  Skip the confirmation prompt (required to run non-interactively)
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

