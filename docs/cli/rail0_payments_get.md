## rail0 payments get

Fetch the current state of a payment

### Synopsis

Fetch a payment's current status from the RAIL0 gateway together with live
on-chain balances (capturableAmount, refundableAmount) when the payment
is in an active on-chain state.

Requires login — a payment is readable only by its payer or payee. Run
'rail0 auth login -p 0xKey…' first (any wallet can log in, no account needed).

Status values:
  unsigned   — created, payer signature not yet submitted
  signed              — payer signature stored, awaiting payee action
  authorized          — funds held in escrow, capture available
  charged             — one-shot charge executed, refund available
  captured            — funds fully captured by payee
  partially_captured  — partial capture executed; remaining escrow still capturable
  voided              — authorization cancelled (only before any capture), funds returned to payer
  released            — full authorization released back to payer (total release, nothing captured)
  refunded            — payment fully settled by refund (escrow and refundable both drained)
  partially_refunded  — legacy; no longer produced (a partial refund leaves the status unchanged)
  failed              — terminal failure; payment cannot proceed

```
rail0 payments get <rail0Id> [flags]
```

### Examples

```
  rail0 payments get 0xabc123…
```

### Options

```
  -h, --help   help for get
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

