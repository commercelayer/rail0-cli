## rail0 payouts destinations confirm

Attest the venue credited the proof: the destination becomes verified

### Synopsis

Record that the venue CREDITED the proof transfer to your account there. The
destination becomes "verified" and accepts payouts of any amount.

Confirm only after you see the deposit at the venue — a transfer mined on-chain is
not the same as a deposit credited: a wrong network, a missing memo or an amount
below the venue's minimum can all be mined and never credited. The proof must be
confirmed on-chain first (422 payout_destination_proof_missing otherwise:
'rail0 payouts list --destination <id> --purpose proof' shows it).

You are asked to confirm (y/N); pass --yes to skip the prompt in a script.

```
rail0 payouts destinations confirm <destination_id> [flags]
```

### Examples

```
  rail0 payouts destinations confirm <destination_id>
```

### Options

```
  -h, --help   help for confirm
  -y, --yes    Skip the confirmation prompt (required to run non-interactively)
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

* [rail0 payouts destinations](rail0_payouts_destinations.md)	 - Manage where payouts go (lists when run bare)

