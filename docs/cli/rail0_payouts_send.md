## rail0 payouts send

Prepare, verify, sign and submit a payout in one step

### Synopsis

Pay out to a destination: the gateway prepares the ERC-20 transfer, the CLI
checks it is exactly that transfer, signs it locally with -p and hands it to the
gateway, which broadcasts it. The result is the payout, "submitted" with its
transaction hash; it turns "confirmed" once mined ('payouts get <id>').

--amount is a decimal in token units (250.00), or "all": the wallet's balance minus
the destination's refund reserve, computed by the gateway. --proof sends the proof
transfer instead (1 token unit, raised to the destination's minimum) — the step
before 'payouts destinations confirm'. An unverified destination takes the proof
only.

Before signing, the command shows what will be sent — amount, token, deposit
address, network, from — and asks to confirm (y/N); --yes skips the prompt and is
required without a terminal. With "all" or --proof the gateway sets the amount: to
see the exact figure before signing, run 'payouts prepare' and then
'payouts submit <id>', which shows it.

The key is the destination's wallet's: the CLI signs in with it. Given as a raw 0x
hex key, as @name for one saved with 'rail0 keys add', through RAIL0_PRIVATE_KEY,
or omitted to be prompted for (hidden).

Refused: payout_below_minimum (below the destination's minimum),
insufficient_balance, payout_destination_unverified (send the proof and confirm
first), payout_destination_disabled, payout_already_signed (a signed payout is
pending: 'payouts resend <id>'). A node refusal (502) leaves the payout pending and
signed: 'payouts resend <id>'.

```
rail0 payouts send [flags]
```

### Examples

```
  rail0 payouts send --destination <destination_id> --proof -p @merchant-key
  rail0 payouts send --destination <destination_id> --amount 250.00 -p @merchant-key
  rail0 payouts send --destination <destination_id> --amount all -p @merchant-key --yes
```

### Options

```
  -a, --amount string        Decimal in token units (e.g. 250.00), or 'all' for the balance minus the refund reserve
  -d, --destination string   Destination id ('rail0 payouts destinations')
  -h, --help                 help for send
  -p, --private-key string   The destination wallet's key: 0x hex, @name for an OS-keychain key ('rail0 keys add'), or RAIL0_PRIVATE_KEY
      --proof                Send the proof transfer (the gateway sets its amount) instead of a payout
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

