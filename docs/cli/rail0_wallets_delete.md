## rail0 wallets delete

Deactivate a wallet — soft delete, reversible (by id or 0x address)

### Synopsis

Deactivate a wallet. Despite the verb, this is a SOFT delete and it is
reversible: the gateway sets active=false and keeps the row, so 'wallets update
<wallet> --active true' brings it back. Nothing is removed, nothing on-chain is
touched, and no funds move.

It is the same operation as 'wallets update <wallet> --active false'. Both
spellings exist because the API exposes a DELETE verb as well as the flag —
use whichever reads better where you are writing it.

What deactivation actually does: the address stops being offered by
'payment-methods', so buyers can no longer discover it and no new payment can
name it, and webhooks stop being dispatched for payments to it. Existing
payments are unaffected — they carry their own payee address and settle
normally.

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

