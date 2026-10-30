## rail0 payouts get

Fetch one payout

### Synopsis

Fetch one payout: its status, amount (base units and amount_formatted), the
transaction hash once signed, and — when failed — error_code: reverted,
foreign_transfer, dropped, submission_timeout or cancelled. submission_timeout and
dropped are not final: 'report-hash' with the transaction that did go out puts the
payout back to submitted.

```
rail0 payouts get <payout_id> [flags]
```

### Examples

```
  rail0 payouts get <payout_id>
```

### Options

```
  -h, --help   help for get
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

