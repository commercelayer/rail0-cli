## rail0 payments redrive

Re-send a signed transaction whose broadcast never reached the chain (payee)

### Synopsis

Ask the gateway to broadcast again a transaction that was prepared and SIGNED
but never reached the chain — a worker that died between the two, a drained queue.
The same signed bytes go out again; nothing about the payment changes, and no new
signature is asked for.

Only a transaction reading 'redrivable: true' qualifies (see 'payments
transaction' or 'payments transactions'). A pending row WITHOUT a signed
transaction is not stuck, it is unfinished: re-run the operation instead. The
gateway also redrives on its own recovery cadence; this asks for it now.

Payee only. Add -w/--wait to block until the transaction confirms (or fails).

```
rail0 payments redrive <rail0Id> <transaction_id> [flags]
```

### Examples

```
  rail0 payments redrive 0xabc123… 7f1c0c8e-… --wait
```

### Options

```
  -h, --help               help for redrive
      --timeout duration   Max time to wait when --wait is set; unset, 10× the chain's measured p90 settlement, kept within 5m–15m (default 15m0s)
  -w, --wait               Block until the operation confirms on-chain (or fails)
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

