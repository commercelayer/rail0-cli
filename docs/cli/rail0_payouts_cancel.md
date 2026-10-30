## rail0 payouts cancel

Cancel a pending payout that was never signed

### Synopsis

Cancel a pending payout the wallet has not signed: it fails with error_code
"cancelled" and its destination is free for a new one. Nothing moved, so nothing
is lost.

A SIGNED payout cannot be cancelled (409 payout_already_signed): its transaction
may still be mined. Resend it and let it settle.

```
rail0 payouts cancel <payout_id> [flags]
```

### Examples

```
  rail0 payouts cancel <payout_id>
```

### Options

```
  -h, --help   help for cancel
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

