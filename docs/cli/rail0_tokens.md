## rail0 tokens

List supported tokens (symbols used in --token)

### Synopsis

List the token catalog.

Includes RETIRED tokens by default (an existing payment references its token
forever, so they stay listed and resolvable); each row carries an "active" flag.
Only an active token can be used for a NEW payment — pass --active to list just
those, or --active=false to see only the retired ones.

```
rail0 tokens [flags]
```

### Examples

```
  rail0 tokens
  rail0 tokens --chain arc
  rail0 tokens --symbol USDC
  rail0 tokens --active           # only what a new payment can use
```

### Options

```
      --active          Filter by active flag (--active, or --active=false for retired only); omit for every token
  -c, --chain chain     Filter by chain: a slug (arc, base, …) or a chain id; the environment's gateway decides testnet vs mainnet
  -h, --help            help for tokens
  -s, --symbol string   Filter by token symbol (case-insensitive, e.g. USDC)
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

