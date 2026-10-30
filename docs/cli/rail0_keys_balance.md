## rail0 keys balance

Read an address's on-chain balance straight from the chain (like cast balance)

### Synopsis

Read the native and/or ERC-20 balance of an address directly from the chain's
RPC nodes — no gateway session, nothing signed.

The address is a 0x address, or the name of a saved key ('rail0 keys add'), with or
without the @: its address is derived from the keychain entry, the key itself is
never printed. A name that is not a saved key is tried in the address book
('rail0 aliases').

--chain picks the chain by slug (arc, base, …) or id, resolved by the current
environment's gateway ('rail0 env'), which also publishes the chain's keyless RPC
endpoints: they are tried in order, each with a timeout, until one answers. Without
--chain the configured default chain applies ('rail0 config set chain'). --rpc-url
reads from that node instead (its eth_chainId names the chain; --chain is then
optional and, if passed, must agree).

Default output is the native balance, in wei and in units (18 decimals, symbol from
the chain). NOTE: on Arc the native gas token is USDC with 18 decimals, while the
USDC ERC-20 (--token USDC) uses 6 — the same money, two different scales.

--token reads one ERC-20: a symbol (USDC) from the gateway's token list for the
chain, or a 0x address, whose symbol() and decimals() are read on-chain. --tokens
reads the native balance plus every active rail0 token of the chain.

-q prints the raw value in wei / base units; with several assets, one
"<asset> <symbol> <value>" line each (asset = native or the token address — on Arc
the native coin and the ERC-20 are both USDC).

```
rail0 keys balance <address|@name|name> [flags]
```

### Examples

```
  rail0 keys balance 0xABCD… --chain arc
  rail0 keys balance @payer --chain arc --token USDC
  rail0 keys balance payer --tokens
  rail0 keys balance 0xABCD… --token 0x3600000000000000000000000000000000000000
  rail0 keys balance 0xABCD… --rpc-url https://rpc.example.org -q
```

### Options

```
  -c, --chain chain      Chain to read (default: 'rail0 config' chain): a slug (arc, base, …) or a chain id; the environment's gateway decides testnet vs mainnet
  -h, --help             help for balance
      --rpc-url string   Read from this JSON-RPC endpoint instead of the gateway's public ones (its eth_chainId names the chain)
  -t, --token string     ERC-20 to read: a symbol from 'rail0 tokens' (USDC) or a 0x token address
      --tokens           Read the native balance plus every active rail0 token of the chain
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

* [rail0 keys](rail0_keys.md)	 - Generate signing keys locally and manage them in the OS keychain

