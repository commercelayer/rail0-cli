## rail0 wallets tokens add

Accept a token on a wallet (re-enables a disabled holding)

### Synopsis

Accept a token, on one chain, on a wallet (by id, 0x address or @alias).

An upsert: a holding that was disabled is re-enabled rather than duplicated.
--token is a symbol (USDC) resolved on --chain, or the token's 0x address.
--default makes it the wallet's default method, demoting the previous one;
--chain falls back to the configured default ('rail0 config set chain').

```
rail0 wallets tokens add <wallet> [flags]
```

### Examples

```
  rail0 wallets tokens add 0xABCD… --chain base --token USDC
  rail0 wallets tokens add 0xABCD… --chain base --token 0x036C… --default
```

### Options

```
      --chain chain    Chain of the token (default: the configured chain): a slug (arc, base, …) or a chain id; the environment's gateway decides testnet vs mainnet
      --default        Make it the wallet's default payment method
  -h, --help           help for add
  -t, --token string   Token symbol (e.g. USDC) or 0x address (default: the configured token)
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

* [rail0 wallets tokens](rail0_wallets_tokens.md)	 - Choose which tokens (and chains) a wallet accepts

