## rail0 payments dispute

Open a dispute (payer-submitted, signal-only): prepare, sign, broadcast

### Synopsis

Prepare the transaction that calls dispute() on the RAIL0 contract, then
sign and submit it like any other operation (the gateway broadcasts it).

Disputes are payer-driven and signal-only: they move no money. Only the payer
may dispute, while the refund window is open and the merchant holds the funds.

Signal-only does not mean instant: it is still an on-chain transaction, so the
API returns 202 and the dispute is not visible until it confirms. Add -w/--wait
to block until it does (or fails).

Pass --private-key (-p): the CLI signs locally with the payer key and broadcasts.

```
rail0 payments dispute <rail0Id> [flags]
```

### Examples

```
  rail0 payments dispute 0xrail0Id… -p @buyer-key --reason 0x… --wait
```

### Options

```
  -h, --help                 help for dispute
  -p, --private-key string   Caller's secp256k1 private key (0x-prefixed hex); signs and broadcasts in one step
      --reason string        bytes32 reason code (0x…); defaults to zero
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
* [rail0 payments dispute close](rail0_payments_dispute_close.md)	 - Close a dispute (payer only): prepare, sign, broadcast

