## rail0 payouts prepare

Prepare a payout without signing it (shows the exact amount)

### Synopsis

Ask the gateway to build a payout's unsigned transfer, and print the pending
payout: the exact amount (base units and amount_formatted) — the way to see what
"all" or a proof comes to —, the reserves it keeps back in the wallet
(refund_reserve, gas_reserve) and unsigned_transaction, the EIP-1559 transaction to
sign. Nothing is signed or sent.

Both reserves are kept by default; --no-refund-reserve / --no-gas-reserve skip one
for this payout (see 'payouts send --help': a later refund or capture may then
fail).

Then either 'payouts submit <id>' (verify, sign with -p, submit), or sign it with
a wallet of your own and report the hash with 'payouts report-hash <id> <hash>'.
Check unsigned_transaction before signing it elsewhere: 'to' must be the token,
'data' transfer(destination address, amount), value 0.

A prepare replaces the destination's pending payout while it is unsigned; leave it
and it stays pending until replaced or cancelled ('payouts cancel <id>'). The
session must be the destination's wallet ('rail0 auth login').

```
rail0 payouts prepare [flags]
```

### Examples

```
  rail0 payouts prepare --destination <destination_id> --amount all
  rail0 payouts prepare --destination <destination_id> --amount all --no-gas-reserve
  rail0 payouts prepare --destination <destination_id> --proof
```

### Options

```
  -a, --amount string        Decimal in token units (e.g. 250.00), or 'all' for the balance minus the reserves kept back
  -d, --destination string   Destination id ('rail0 payouts destinations')
  -h, --help                 help for prepare
      --no-gas-reserve       Do not keep the gas reserve back for this payout (native-gas chains; a later refund or capture may fail)
      --no-refund-reserve    Do not keep the refund reserve back for this payout (a later refund may fail)
      --proof                Send the proof transfer (the gateway sets its amount) instead of a payout
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

