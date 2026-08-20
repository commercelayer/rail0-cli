## rail0 wallets update

Update label or active status on a wallet (by id or 0x address)

### Synopsis

Change a wallet's label, or turn it on and off with --active.

--active false is the merchant's REVOCATION switch, and it does more than hide a
row: a deactivated wallet stops being offered by 'payment-methods', so no new
payment can name it, and webhooks stop being dispatched for payments to it. That
is deliberate — deactivation says "stop trusting this address", and an outbound
push to a revoked wallet's payments would contradict it.

Existing payments are unaffected: they carry their own payee address and settle
whatever the flag says now. Reversible with --active true.

'wallets delete' is the same operation under the API's DELETE verb — a soft
delete, not a removal — so the two are interchangeable.

```
rail0 wallets update <wallet> [flags]
```

### Examples

```
  rail0 wallets update 0xABCD… --label "New label" --active true
```

### Options

```
      --active string   Set active flag ('true' or 'false')
  -h, --help            help for update
  -l, --label string    New wallet label
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

