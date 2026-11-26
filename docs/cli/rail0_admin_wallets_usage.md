## rail0 admin wallets usage

What still names a wallet's address: payments as payee and payer, transactions sent

### Synopsis

Count what still names a wallet's address, across every account: the payments it is
the payee of, the payments it is the payer of, and the transactions it sent. How much
history the address carries: what 'rail0 admin wallets deactivate' shows before it asks.

```
rail0 admin wallets usage <wallet> [flags]
```

### Examples

```
  rail0 admin wallets usage 0x2D2B… --account 019e748b-…
```

### Options

```
  -h, --help   help for usage
```

### Options inherited from parent commands

```
      --account string    The account the wallets belong to (required)
      --agent             Agent mode: one JSON envelope on stdout, JSON events on stderr, no prompts (env: RAIL0_AGENT=1)
      --base-url string   RAIL0 gateway base URL (env: RAIL0_BASE_URL)
      --debug             Print HTTP request/response details to stderr
      --json              Force plain (uncolored) JSON output (useful in scripts)
      --pretty            Use the key-value/table view instead of JSON (terminal only)
  -q, --quiet             Print only the result's id/hash, one per line (useful in scripts)
```

### SEE ALSO

* [rail0 admin wallets](rail0_admin_wallets.md)	 - Any account's wallets: list them, see what names one, switch it off or on (lists when run bare)

