## rail0 payouts resend

Re-broadcast the signed transfer a submit stored

### Synopsis

Re-broadcast the signed transfer the gateway stored on submit, after the node
refused it or did not answer. The same bytes, so the same nonce and hash: it can be
mined once at most, and resending is always safe. Nothing is signed again.

422 payout_not_signed when nothing is stored: sign it with 'payouts submit <id>'.

```
rail0 payouts resend <payout_id> [flags]
```

### Examples

```
  rail0 payouts resend <payout_id>
```

### Options

```
  -h, --help   help for resend
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

