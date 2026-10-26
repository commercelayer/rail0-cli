## rail0 payments dispute

Open a dispute (payer-submitted, signal-only): prepare, sign, broadcast

### Synopsis

Prepare the transaction that calls dispute() on the RAIL0 contract, then
sign and submit it like any other operation (the gateway broadcasts it).

Disputes are payer-driven and signal-only: they move no money. Only the payer
may dispute, while the refund window is open and the merchant holds the funds.

A reason is optional: --reason takes a code from the gateway's dispute-reason
dictionary (e.g. not_received, duplicate, other) or exactly its bytes32. It is
checked locally before anything is sent. Leave it out to open the dispute with
no reason (the zero bytes32, read back as "No reason given"); on a terminal,
leaving it out offers the list with "No reason" first. 'rail0 disputes reasons'
lists the codes and what they mean.

Signal-only does not mean instant: it is still an on-chain transaction, so the
API returns 202 and the dispute is not visible until it confirms. Add -w/--wait
to block until it does (or fails).

Pass --private-key (-p): the CLI signs locally with the payer key and broadcasts.

```
rail0 payments dispute <rail0Id> [flags]
```

### Examples

```
  rail0 payments dispute 0xrail0Id… -p @buyer-key --reason not_received --wait
```

### Options

```
  -h, --help                     help for dispute
      --idempotency-key string   Make a re-run safe: the same key returns the first transaction instead of opening a second
  -p, --private-key string       Caller's secp256k1 private key (0x-prefixed hex); signs and broadcasts in one step
      --reason string            Optional. Why the dispute is opened: a code from 'rail0 disputes reasons' (open list) or its bytes32; omit for no reason
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
* [rail0 payments dispute close](rail0_payments_dispute_close.md)	 - Close a dispute (payer only): prepare, sign, broadcast

