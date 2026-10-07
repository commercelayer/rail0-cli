## rail0 wallets tokens

Choose which tokens (and chains) a wallet accepts

### Synopsis

Manage a wallet's token holdings — the (chain, token) pairs it accepts.

They are what 'payment-methods' offers buyers and what payment creation checks:
a wallet with no active holding cannot be paid. Every holding survives being
disabled, so re-enabling one keeps its history.

A wallet with active holdings always has exactly ONE default, the method a buyer
is offered first. The first holding you add takes it; to move it, run
'wallets tokens add' on the new one with --default. While other holdings are
active the gateway refuses to disable the default (422 default_payment_method) —
move it first. The LAST active holding can be disabled even as the default: the
wallet then accepts nothing, and the next holding enabled becomes the default.

### Examples

```
  rail0 wallets tokens add 0xABCD… --chain-id 84532 --token USDC --default
  rail0 wallets tokens disable 0xABCD… USDC --chain-id 84532
  rail0 wallets tokens enable 0xABCD… USDC --chain-id 84532
```

### Options

```
  -h, --help   help for tokens
```

### Options inherited from parent commands

```
      --base-url string   RAIL0 gateway base URL (env: RAIL0_BASE_URL)
      --debug             Print HTTP request/response details to stderr
      --json              Force plain (uncolored) JSON output (useful in scripts)
      --pretty            Use the key-value/table view instead of JSON (terminal only)
  -q, --quiet             Print only the result's id/hash, one per line (useful in scripts)
```

### SEE ALSO

* [rail0 wallets](rail0_wallets.md)	 - Manage the logged-in account's wallets (lists when run bare)
* [rail0 wallets tokens add](rail0_wallets_tokens_add.md)	 - Accept a token on a wallet (re-enables a disabled holding)
* [rail0 wallets tokens disable](rail0_wallets_tokens_disable.md)	 - Stop accepting a token on a wallet (soft, reversible)
* [rail0 wallets tokens enable](rail0_wallets_tokens_enable.md)	 - Re-enable a holding the wallet already has

