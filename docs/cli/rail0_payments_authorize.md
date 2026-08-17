## rail0 payments authorize

Authorize a payment (prepare, sign, and broadcast)

### Synopsis

Prepare the transaction that calls authorize() on the RAIL0 contract,
pulling funds from the payer into escrow.

The payer must have created and signed the payment first via
'payments create -p' (create + sign in one atomic step).
The API returns 202 immediately. The payment stays at "signed" until
rail0-indexer confirms the on-chain event and the API transitions it to
"authorized". Poll 'payments get <rail0Id>' until payment=authorized
and the authorize transaction shows confirmed.

Pass --private-key (-p): the CLI signs locally and broadcasts in one step.

```
rail0 payments authorize <rail0Id> [flags]
```

### Examples

```
  # 1. Payer creates and signs the payment in one step
  rail0 payments create --from 0xBuyer… --to 0xMerchant… \
    --token 0x3600… --amount 1.00 --chain-id 5042002 -p 0xPayerKey…

  # 2. Payee authorizes
  rail0 payments authorize <rail0Id> -p 0xPayeeKey…
```

### Options

```
  -h, --help                 help for authorize
  -p, --private-key string   Caller's secp256k1 private key (0x-prefixed hex); signs and broadcasts in one step
      --timeout duration     Max time to wait when --wait is set — the default covers the slowest chain's finality (default 15m0s)
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

