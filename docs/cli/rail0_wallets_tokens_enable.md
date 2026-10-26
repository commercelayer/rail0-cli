## rail0 wallets tokens enable

Re-enable a holding the wallet already has

### Synopsis

Re-enable an existing, disabled holding. <token> is the token's symbol or 0x
address (add --chain-id when the wallet holds that symbol on several chains), or
the token id shown as token_id by 'rail0 wallets'. A token the wallet has never
held is not enabled here — use 'wallets tokens add'.

```
rail0 wallets tokens enable <wallet> <token> [flags]
```

### Examples

```
  rail0 wallets tokens enable 0xABCD… USDC --chain-id 84532
```

### Options

```
      --chain-id int   Narrow <token> to one chain when the wallet holds it on several
  -h, --help           help for enable
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

