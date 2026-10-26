## rail0 payment-methods

Discover a merchant's accepted wallets/tokens (public, no login)

### Synopsis

List a merchant's active payment methods: its active wallets, each with the
active tokens (and chains) it accepts.

Public — no login required. Identify the merchant with exactly one of --account
(all its active wallets) or --address (just that one wallet; accepts an @alias).

```
rail0 payment-methods [flags]
```

### Examples

```
  rail0 payment-methods --account 019e748b-…
  rail0 payment-methods --address 0xABCD…
  rail0 payment-methods --address @payee
```

### Options

```
      --account string   Merchant account id — list all its active wallets
  -A, --address string   A merchant wallet address (0x… or @alias) — list just that wallet
  -h, --help             help for payment-methods
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

