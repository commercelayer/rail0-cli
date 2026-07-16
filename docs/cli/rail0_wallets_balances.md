## rail0 wallets balances

Read a wallet's live on-chain balances (by id or 0x address)

### Synopsis

Read a wallet's live on-chain balances across the configured chains.

Each chain reports the native gas-token balance and the active token balances,
or an error if that chain's RPC was unreachable (reported per chain, so one dead
RPC never hides the others). Use --chain-id / --symbol to narrow the result.

```
rail0 wallets balances <wallet> [flags]
```

### Examples

```
  rail0 wallets balances 0xABCD… --chain-id 84532 --symbol USDC
```

### Options

```
      --chain-id int    Restrict to one chain id
  -h, --help            help for balances
  -s, --symbol string   Restrict tokens to this symbol, e.g. USDC (alias: --token)
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

