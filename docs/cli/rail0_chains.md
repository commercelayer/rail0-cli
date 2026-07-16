## rail0 chains

List supported blockchains (chain IDs used in --chain-id)

### Synopsis

List the blockchains supported by the environment.

Filter with --symbol (the chain's native symbol, e.g. ETH / USDC / CELO) and/or
--network-type (testnet | mainnet). Both match case-insensitively; combine them
to narrow further.

```
rail0 chains [flags]
```

### Examples

```
  rail0 chains
  rail0 chains --symbol USDC
  rail0 chains --network-type testnet
  rail0 chains -s ETH -n mainnet
```

### Options

```
  -h, --help                  help for chains
  -n, --network-type string   Filter by network type: testnet or mainnet
  -s, --symbol string         Filter by native symbol (e.g. ETH, USDC, CELO)
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

* [rail0](rail0.md)	 - CLI for the RAIL0 stablecoin payment protocol

