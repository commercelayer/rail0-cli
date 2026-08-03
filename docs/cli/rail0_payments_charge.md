## rail0 payments charge

Charge a payment — authorize + capture in one call (no escrow)

### Synopsis

Prepare the transaction that calls charge() on the RAIL0 contract.
Combines authorize + capture into a single on-chain call; funds move
directly from payer to payee without an escrow window.

The payment must have been created with -C/--charge and signed by the
payer via 'payments create -p' (create + sign in one atomic step).
The API returns 202 immediately. The payment stays at "signed" until
rail0-indexer confirms and the API transitions it to "charged".
Poll 'payments get <rail0Id>' until payment=charged and the charge
transaction shows confirmed.

Pass --private-key (-p): the CLI signs locally and broadcasts in one step.

```
rail0 payments charge <rail0Id> [flags]
```

### Examples

```
  rail0 payments charge 0xrail0Id… -p 0xPayeeKey…
```

### Options

```
  -h, --help                 help for charge
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

