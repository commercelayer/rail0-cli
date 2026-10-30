## rail0 payouts report-hash

Report a transfer you broadcast yourself

### Synopsis

Tell the gateway the hash of a payout transfer you signed and broadcast yourself
(a wallet that signs and sends in one step), instead of 'payouts submit'.

The hash must be THIS transfer — from the wallet, to the token, this amount to
this destination: a hash a node already knows that is anything else is refused
(foreign_transfer_hash), and one another payout records is 409 payout_hash_in_use.
A hash no node has seen yet is accepted and settled from its receipt.

Also the way back for a payout that failed submission_timeout or dropped because
the transaction that went out was sped up or replaced: report its hash and the
payout returns to submitted.

```
rail0 payouts report-hash <payout_id> <transaction_hash> [flags]
```

### Examples

```
  rail0 payouts report-hash <payout_id> 0x5c50…
```

### Options

```
  -h, --help   help for report-hash
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

* [rail0 payouts](rail0_payouts.md)	 - Send a wallet's tokens to your own venue (lists when run bare; requires login)

