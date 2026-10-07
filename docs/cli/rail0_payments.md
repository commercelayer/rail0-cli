## rail0 payments

Payment lifecycle operations

### Synopsis

Manage the full RAIL0 payment lifecycle.

Every on-chain operation is atomic: pass --private-key (-p) and the CLI prepares
the unsigned transaction, signs it locally, and broadcasts it in a single step.

  rail0 payments authorize <rail0Id> -p @merchant-key --wait

Add -w/--wait to any of them to block until the chain confirms the operation,
rather than returning as soon as it is broadcast. A key is given as a raw 0x
hex key, as @name for one saved with 'rail0 keys add', or omitted to be
prompted for (hidden).

Authorize-and-capture flow — the buyer signs off-chain, the merchant submits:
  1. payments create -F @buyer -T @merchant -p @buyer-key   — buyer creates + signs
  2. payments authorize <rail0Id> -p @merchant-key -w       — merchant escrows
  3. payments capture   <rail0Id> -a <n> -p @merchant-key -w  — merchant takes it

Charge (one-shot) flow — no escrow, funds move straight through:
  1. payments create -C -F @buyer -T @merchant -p @buyer-key
  2. payments charge <rail0Id> -p @merchant-key -w

### Options

```
  -h, --help   help for payments
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

* [rail0](rail0.md)	 - CLI for the RAIL0 stablecoin payment protocol
* [rail0 payments authorize](rail0_payments_authorize.md)	 - Authorize a payment (prepare, sign, and broadcast)
* [rail0 payments capture](rail0_payments_capture.md)	 - Capture a payment (prepare, sign, and broadcast)
* [rail0 payments charge](rail0_payments_charge.md)	 - Charge a payment — authorize + capture in one call (no escrow)
* [rail0 payments create](rail0_payments_create.md)	 - Create a payment and sign it with the payer's key
* [rail0 payments dispute](rail0_payments_dispute.md)	 - Open a dispute (payer-submitted, signal-only): prepare, sign, broadcast
* [rail0 payments disputes](rail0_payments_disputes.md)	 - List a payment's dispute open/close history
* [rail0 payments get](rail0_payments_get.md)	 - Fetch the current state of a payment
* [rail0 payments history](rail0_payments_history.md)	 - Show a payment's chronological history
* [rail0 payments list](rail0_payments_list.md)	 - List payments for the authenticated wallet (requires login)
* [rail0 payments redrive](rail0_payments_redrive.md)	 - Re-send a signed transaction whose broadcast never reached the chain (payee)
* [rail0 payments refund](rail0_payments_refund.md)	 - Refund a payment (prepare, sign, and broadcast)
* [rail0 payments release](rail0_payments_release.md)	 - Release a payment after expiry (prepare, sign, and broadcast)
* [rail0 payments sign](rail0_payments_sign.md)	 - Deposit the payer signature on an already-created payment
* [rail0 payments transaction](rail0_payments_transaction.md)	 - Fetch one of a payment's transactions
* [rail0 payments transactions](rail0_payments_transactions.md)	 - List on-chain transactions for a payment
* [rail0 payments update](rail0_payments_update.md)	 - Change a payment's description
* [rail0 payments void](rail0_payments_void.md)	 - Void a payment (prepare, sign, and broadcast)
* [rail0 payments wait](rail0_payments_wait.md)	 - Block until a payment settles (or reaches a given status)

