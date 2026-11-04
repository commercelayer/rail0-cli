## rail0 payouts destinations reserves

What a payout to a destination would keep back now, and what it could move

### Synopsis

Show what a payout to the destination would keep back in the wallet right now,
and what '--amount all' would move. Nothing is stored: the gateway computes it
from the wallet's payments and the chain's current fees, so the figures can move
by the time you send.

  balance         the wallet's balance of the destination's token
  refund reserve  the refundable balance of the wallet's payments in this token
                  still inside their refund window, and how many payments that is
                  (a refund is pulled from the wallet: an emptied wallet fails it)
  gas reserve     only on a chain whose gas is paid in this very token (Arc's
                  USDC): the gas for those refunds and for capturing the wallet's
                  open authorizations plus a base of new captures
  gas allowance   the payout transfer's own fee, always kept back on such a chain
  available       what '--amount all' moves with both reserves kept (the default)

Both reserves are kept by default; 'payouts send --no-refund-reserve' /
'--no-gas-reserve' skip one for a single payout. Any wallet of the account may
read it; a deleted destination still reads. --json prints the gateway's record.

```
rail0 payouts destinations reserves <destination_id> [flags]
```

### Examples

```
  rail0 payouts destinations reserves <destination_id>
  rail0 payouts destinations reserves <destination_id> --json
```

### Options

```
  -h, --help   help for reserves
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

