## rail0 payments release

Release a payment after expiry (prepare, sign, and broadcast)

### Synopsis

Prepare the transaction that calls release() on the RAIL0 contract.

Release is only available after authorizationExpiry has passed on-chain.
Returns the remaining (uncaptured) escrowed balance to the payer. Permissionless
— both payer and payee can call it; the caller is whoever the signing key belongs
to (derived from the key), so pass the payer's key to release as the payer.
The API returns 202 immediately; poll 'payments get <rail0Id>'. The status
advances to "released" only on a TOTAL release (an untouched authorization,
nothing captured); if a capture left a refundable residual the status stays
unchanged and that residual remains refundable.

Pass --private-key (-p): the CLI signs locally and broadcasts in one step.

```
rail0 payments release <rail0Id> [flags]
```

### Examples

```
  # Payee releases
  rail0 payments release 0xrail0Id… -p 0xPayeeKey…

  # Payer releases (just use the payer's key)
  rail0 payments release 0xrail0Id… -p 0xPayerKey…
```

### Options

```
  -h, --help                 help for release
  -p, --private-key string   Caller's secp256k1 private key (0x-prefixed hex); signs and broadcasts in one step
      --timeout duration     Max time to wait when --wait is set — the default covers the slowest chain's finality (default 15m0s)
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

