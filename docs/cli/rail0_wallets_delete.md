## rail0 wallets delete

Deactivate the session's wallet — soft delete, reversible (by id, 0x address or @alias)

### Synopsis

Deactivate a wallet. Despite the verb, this is a SOFT delete and it is
reversible: the gateway sets active=false and keeps the row, so it can be
switched back on — by an operator, since a deactivated wallet's own session is
read-only and another wallet's session cannot change it. Nothing is removed, nothing on-chain is
touched, and no funds move.

It is the same operation as 'wallets update <wallet> --active false'. Both
spellings exist because the API exposes a DELETE verb as well as the flag —
use whichever reads better where you are writing it. Either way the account's
only active wallet is refused (422 last_active_wallet): another wallet must be
active first. And either way it acts only on the wallet you signed in with:
another wallet of the account is refused (403 not_your_wallet).

What deactivation actually does: the address stops being offered by
'payment-methods', so buyers can no longer discover it and no new payment can
name it, and webhooks stop being dispatched for payments to it.

Existing payments still SETTLE — they carry their own payee address and the chain
does not care — but you stop being TOLD about the ones still in flight: webhook
dispatch resolves the payee wallet's account, so a deactivated wallet delivers
nothing further and no delivery is recorded as skipped. Capture them, or expect
to poll. (rail0-gateway#237)

```
rail0 wallets delete <wallet> [flags]
```

### Examples

```
  rail0 wallets delete 0xABCD…
```

### Options

```
  -h, --help   help for delete
  -y, --yes    Skip the confirmation prompt (required to run non-interactively)
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

