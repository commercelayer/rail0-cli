## rail0 payouts submit

Verify, sign and submit a prepared payout

### Synopsis

Sign a pending payout prepared earlier ('payouts prepare') and hand it to the
gateway, which broadcasts it.

The transaction is checked against the destination, read again from the gateway —
exactly transfer(destination address, payout amount) on the destination's token
and chain, value 0, from your key's wallet — and refused otherwise before the key
touches it. The command shows the amount, token, deposit address and network and
asks to confirm (y/N); --yes skips it and is required without a terminal.

Only a pending, unsigned payout can be signed. One already signed is 'resend's:
its transaction may already be in a mempool, and signing again would risk a second
transfer.

```
rail0 payouts submit <payout_id> [flags]
```

### Examples

```
  rail0 payouts submit <payout_id> -p @merchant-key
```

### Options

```
  -h, --help                 help for submit
  -p, --private-key string   The destination wallet's key: 0x hex, @name for an OS-keychain key ('rail0 keys add'), or RAIL0_PRIVATE_KEY
  -y, --yes                  Skip the confirmation prompt (required to run non-interactively)
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

