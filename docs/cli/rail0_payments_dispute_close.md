## rail0 payments dispute close

Close a dispute (payer only): prepare, sign, broadcast

### Synopsis

Prepare the transaction that calls closeDispute() on the RAIL0 contract, then
sign and submit it like any other operation.

Only the PAYER may withdraw a dispute — a merchant cannot dismiss one. The
merchant's only way to close a dispute is a full 'refund', which closes it
automatically. A withdrawn dispute can be reopened while the refund window is
open.

A reason is optional; when given it comes from the CLOSE list
(resolved_with_merchant, withdrawn, item_received, other) or is exactly its
bytes32 — not the open list. full_refund is recorded by the protocol when a
refund closes the dispute and can't be chosen. Leave --reason out to close with
no reason ("No reason given"); on a terminal, leaving it out offers the list with
"No reason" first. 'rail0 disputes reasons --kind close' lists the codes.

Like 'dispute', this is an on-chain transaction: the API returns 202 and the
close is not visible until it confirms. Add -w/--wait to block until it does
(or fails).

Pass --private-key (-p): the CLI signs locally with the payer key and broadcasts.

```
rail0 payments dispute close <rail0Id> [flags]
```

### Examples

```
  rail0 payments dispute close 0xrail0Id… -p @buyer-key --reason withdrawn --wait
```

### Options

```
  -h, --help                     help for close
      --idempotency-key string   Make a re-run safe: the same key returns the first transaction instead of opening a second
  -p, --private-key string       Caller's secp256k1 private key (0x-prefixed hex); signs and broadcasts in one step
      --reason string            Optional. Why the dispute is closed: a code from 'rail0 disputes reasons' (close list) or its bytes32; omit for no reason
      --timeout duration         Max time to wait when --wait is set; unset, 10× the chain's measured p90 settlement, kept within 5m–15m (default 15m0s)
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

* [rail0 payments dispute](rail0_payments_dispute.md)	 - Open a dispute (payer-submitted, signal-only): prepare, sign, broadcast

