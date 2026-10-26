## rail0 payments release

Release a payment after expiry (prepare, sign, and broadcast)

### Synopsis

Prepare the transaction that calls release() on the RAIL0 contract.

Release is only available after authorizationExpiry has passed on-chain.
Returns the remaining (uncaptured) escrowed balance to the payer. Only the payer
or the payee may release — the contract accepts no other sender, and the gateway
refuses anyone else with 422 release_submitter_not_a_party. The caller is whoever
the signing key belongs to (derived from the key), so pass the payer's key to
release as the payer.

The API returns 202 immediately: broadcasting is not settling. Add -w/--wait to
block until the operation confirms on-chain (or fails) and exit non-zero if it
didn't land.

Note that the payment STATUS advances to "released" only on a TOTAL release (an
untouched authorization, nothing captured); if a capture left a refundable
residual the status stays unchanged and that residual remains refundable. A
release that leaves BOTH balances at zero — the captured part was already
refunded — closes the payment as "refunded" instead: fully settled. This
does not weaken --wait: it waits on THIS operation's transaction reaching
confirmed or failed, not on a status change — so it settles a partial release
correctly, where waiting for a new status would hang until the deadline.

Pass --private-key (-p): the CLI signs locally and broadcasts in one step.

```
rail0 payments release <rail0Id> [flags]
```

### Examples

```
  # Payee releases
  rail0 payments release 0xrail0Id… -p @merchant-key --wait

  # Payer releases (just use the payer's key)
  rail0 payments release 0xrail0Id… -p @buyer-key --wait
```

### Options

```
  -h, --help                     help for release
      --idempotency-key string   Make a re-run safe: the same key returns the first transaction instead of opening a second
  -p, --private-key string       Caller's secp256k1 private key (0x-prefixed hex); signs and broadcasts in one step
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

