## rail0 payouts destinations add

Register a payout destination (asks you to confirm the details)

### Synopsis

Register a deposit address as a payout destination of the session's wallet.

Before running it, open the venue's deposit page with the NETWORK set to --chain
and the asset set to --token, and copy the address and the minimum deposit from
there. The command shows what it will register and the details statement, and you
must accept it: a y/N prompt (default no) on a terminal, or --confirm-details when
there is no terminal or with --agent. Without it nothing is sent. --yes does not
accept it for you.

--token is a symbol (USDC) resolved on --chain, or the token's 0x address;
--chain and --token fall back to the configured defaults ('rail0 config').
--min-amount and --reserve are decimals in token units (10.00), converted with the
token's decimals. --wallet defaults to the wallet you are signed in with, and must
be it.

The destination is born "unverified". Next: send the proof with
'rail0 payouts send --destination <id> --proof', and once the venue shows it
credited, 'rail0 payouts destinations confirm <id>'.

Refused (422): the zero address, the wallet itself, a contract the gateway knows on
that chain, or no confirmation; 409 when the wallet already has this address for
this token and chain.

```
rail0 payouts destinations add [flags]
```

### Examples

```
  rail0 payouts destinations add -c arc -t USDC -A 0xVenueDeposit… \
    -l "Exchange USDC (Arc)" --min-amount 10

  # non-interactive: accept the statement explicitly
  rail0 payouts destinations add -c arc -t USDC -A 0xVenueDeposit… --confirm-details
```

### Options

```
  -A, --address string      The venue's deposit address (0x…, or @alias)
  -c, --chain chain         Chain the deposit address was issued for (default: 'rail0 config' chain): a slug (arc, base, …) or a chain id; the environment's gateway decides testnet vs mainnet
      --confirm-details     Accept the details statement without a prompt (needed without a TTY or with --agent); --yes does NOT imply it
  -h, --help                help for add
      --kind string         Informational: exchange, circle_mint, self_custody, provider (default exchange)
  -l, --label string        Human-readable label
      --min-amount string   The venue's minimum deposit, decimal in token units (e.g. 10.00)
      --reserve string      Refund reserve left in the wallet on '--amount all', decimal in token units
  -t, --token string        Token symbol (USDC) or 0x address on that chain (default: 'rail0 config' token)
      --wallet string       Wallet that will send to it, id or address (default: the signed-in wallet)
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

