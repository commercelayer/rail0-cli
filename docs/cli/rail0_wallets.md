## rail0 wallets

Manage the logged-in account's wallets (lists when run bare)

### Synopsis

List and manage the wallets of an account.

All wallet endpoints require a login ('rail0 auth login') — they are scoped to
your own account. When listing, --account defaults to the session account; if
given it must be your own account (the gateway rejects another account's id with
403). To discover ANOTHER merchant's accepted wallets/tokens without a session,
use 'rail0 payment-methods'. The verbs below act on one wallet (id or 0x
address) of the session account.

A session is bound to the wallet it signed in with. Reads ('wallets',
'wallets get', 'wallets balances') cover every wallet of the account, but
'wallets update', 'wallets delete' and 'wallets tokens' act only on the wallet
you signed in with: another wallet of the account is refused (403
not_your_wallet). Adding a wallet ('wallets create') is currently unavailable.

```
rail0 wallets [flags]
```

### Examples

```
  rail0 wallets                                   # session account
  rail0 wallets --account 019e748b-… --chain-id 5042002 --symbol USDC
  rail0 wallets get 0xABCD…
  rail0 wallets balances 0xABCD…
```

### Options

```
      --account string        Your account id to list (default: the logged-in session account; must be your own)
      --active string         Filter wallets by active flag ('true' or 'false')
      --chain-id int          Restrict nested tokens to this chain id
      --default string        Restrict nested token holdings to the default one ('true' or 'false')
  -h, --help                  help for wallets
      --page int              Page number (1-based)
      --per-page int          Wallets per page
      --sort string           Sort wallets (e.g. -created_at)
  -s, --symbol string         Restrict nested tokens to this symbol, e.g. USDC (alias: --token)
      --token-active string   Restrict nested token holdings by active flag ('true' or 'false')
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

* [rail0](rail0.md)	 - CLI for the RAIL0 stablecoin payment protocol
* [rail0 wallets alias](rail0_wallets_alias.md)	 - Save a unique address-book alias for an address (like 'aliases add', args reversed)
* [rail0 wallets balances](rail0_wallets_balances.md)	 - Read a wallet's live on-chain balances (by id, 0x address or @alias)
* [rail0 wallets create](rail0_wallets_create.md)	 - Register a new wallet on the account (currently unavailable)
* [rail0 wallets delete](rail0_wallets_delete.md)	 - Deactivate the session's wallet — soft delete, reversible (by id, 0x address or @alias)
* [rail0 wallets get](rail0_wallets_get.md)	 - Fetch a single wallet (by id, 0x address or @alias)
* [rail0 wallets tokens](rail0_wallets_tokens.md)	 - Choose which tokens (and chains) a wallet accepts
* [rail0 wallets update](rail0_wallets_update.md)	 - Update label or active status on the session's wallet (by id, 0x address or @alias)

