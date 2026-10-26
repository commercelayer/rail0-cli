## rail0 wallets tokens disable

Stop accepting a token on a wallet (soft, reversible)

### Synopsis

Stop accepting a token on a wallet. The holding is kept, disabled, so
'wallets tokens enable' brings it back. <token> is resolved as for 'enable'.

Buyers stop being offered this method at once and no new payment can use it;
payments already created with it are unaffected. The wallet's DEFAULT method
cannot be disabled — make another one the default first ('wallets tokens add …
--default').

```
rail0 wallets tokens disable <wallet> <token> [flags]
```

### Examples

```
  rail0 wallets tokens disable 0xABCD… USDC --chain-id 84532
```

### Options

```
      --chain-id int   Narrow <token> to one chain when the wallet holds it on several
  -h, --help           help for disable
  -y, --yes            Skip the confirmation prompt (required to run non-interactively)
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

