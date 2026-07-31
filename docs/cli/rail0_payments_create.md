## rail0 payments create

Create a payment and sign it with the payer's key

### Synopsis

Create a RAIL0 payment and sign the EIP-3009 payload with the payer's key in
one step (--private-key / -p). The API stores the signature; the payee can then
authorize. If --private-key is omitted, the CLI prompts for it (hidden) on a
terminal — so the key stays out of the shell history and process arguments.

--from (payer) and --to (payee) must be different addresses: a payment can't be
sent from an address to itself.

```
rail0 payments create [flags]
```

### Examples

```
  rail0 payments create \
    -F 0xBuyer… -T 0xMerchant… -t USDC -a 10.50 -c 5042002 \
    -p 0xPayerKey…
```

### Options

```
  -a, --amount string            Amount as a decimal in token units (e.g. '1.00' for 1 USDC); the gateway converts to base units
  -c, --chain-id int             EVM chain ID as listed in GET /blockchains (e.g. 5042002)
  -C, --charge                   Create with mode=charge (immediate transfer, no escrow); default is authorize (escrow then capture)
  -d, --description string       Optional reconciliation note (e.g. order id / invoice); prompted when omitted on a terminal
  -F, --from string              Payer wallet address (from) (0x-prefixed, funds are pulled from here)
  -h, --help                     help for create
      --idempotency-key string   Replay-safety key (default: a fresh one per invocation). Reuse it to retry the SAME create without minting a second payment
  -f, --payment-file string      Path to a JSON file with the full request body (overrides individual flags)
  -p, --private-key string       Payer's secp256k1 private key (0x-prefixed hex); creates the payment and signs locally
  -T, --to string                Payee wallet address (to) (0x-prefixed, address from 'wallets')
  -t, --token string             Token symbol as listed in GET /tokens (e.g. USDC)
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

* [rail0 payments](rail0_payments.md)	 - Payment lifecycle operations

