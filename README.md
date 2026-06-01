# rail0-cli

Command-line interface for the [RAIL0](https://rail0.xyz) stablecoin payment protocol. Wraps the RAIL0 REST API and exposes every payment lifecycle step as a subcommand with human-friendly styled output on a terminal and machine-readable JSON when piped or when `--json` is passed.

---

## Requirements

- Go ≥ 1.22
- The [rail0-go](https://github.com/DrPierlu/rail0-go) SDK cloned as a sibling directory (used via a `replace` directive in `go.mod`)
- Access to a RAIL0 API instance

---

## Installation

Both repositories must be present as siblings:

```
parent-directory/
├── rail0-cli/   ← this repo
└── rail0-go/    ← Go SDK (replace directive points here)
```

```sh
git clone https://github.com/DrPierlu/rail0-cli
git clone https://github.com/DrPierlu/rail0-go
cd rail0-cli
go build -o rail0 .
mv rail0 /usr/local/bin/
```

---

## Configuration

Set the API endpoint and credentials via environment variables or flags. Flags take priority over environment variables.

| Flag         | Environment variable | Description                                      |
|--------------|----------------------|--------------------------------------------------|
| `--base-url` | `RAIL0_BASE_URL`     | RAIL0 API base URL                               |
| `--api-key`  | `RAIL0_API_KEY`      | Bearer token sent in the `Authorization` header  |
| `--debug`    | —                    | Print HTTP request/response details to stderr    |
| `--json`     | —                    | Force plain JSON output (useful in scripts)      |

Minimal setup:

```sh
export RAIL0_BASE_URL=https://api.rail0.xyz
export RAIL0_API_KEY=your_api_key
```

---

## Signing modes

### Payer — `payments create`

The payer creates the payment and signs the EIP-712 payload. Two modes:

| Mode | Command | Description |
|------|---------|-------------|
| Direct key | `payments create … -p 0xPayerKey…` | Creates the payment, signs locally, submits signature in one step |
| External wallet | `payments create … -s 0x1a2b…` with `<rail0Id>` | Submit a signature produced by MetaMask / `eth_signTypedData_v4` |

### Payee — action commands

Every on-chain action command (`authorize`, `charge`, `capture`, `void`, `release`, `approve`, `refund`) requires exactly one of:

| Flag | Short | Description |
|------|-------|-------------|
| `--private-key` | `-p` | Sign locally and broadcast in one step |
| `--prepare` | `-w` | Return the unsigned tx for external wallet signing, then broadcast with `payments submit -x` |

`--private-key` and `--prepare` are mutually exclusive.

---

## Commands

### Accounts

#### `accounts payment-methods <accountId>`

List the accepted payment methods for an account. The returned `walletAddress`, `tokenAddress`, and `chainId` map directly to `--to`, `--token`, and `--chain-id` of `payments create`.

```sh
rail0 accounts payment-methods 019e6f9e-…
rail0 accounts payment-methods 019e6f9e-… --json
```

---

### Payments

#### `payments create`

Create a payment, sign the EIP-712 payload, and submit the payer's signature in one step.

| Flag | Short | Required | Default | Description |
|------|-------|----------|---------|-------------|
| `--from` | `-F` | yes | — | Payer wallet address (0x-prefixed) |
| `--to` | `-T` | yes | — | Payee wallet address (0x-prefixed) |
| `--token` | `-t` | yes | — | ERC-20 token address (0x-prefixed) |
| `--amount` | `-a` | yes | — | Amount in token base units (e.g. `1000000` for 1 USDC) |
| `--chain-id` | `-c` | yes | — | EVM chain ID (e.g. `5042002` for Arc Testnet) |
| `--mode` | `-m` | no | `authorize` | `authorize` (escrow + capture) or `charge` (immediate) |
| `--payment-file` | `-f` | no | — | JSON file with the full request body |
| `--private-key` | `-p` | no* | — | Payer's key — creates and signs locally in one step |
| `--signature` | `-s` | no* | — | Submit an externally-produced EIP-712 signature for an existing `<rail0Id>` |

\* one of `-p` or `-s` required (mutually exclusive)

```sh
# Payer with direct key access — create + sign in one step
rail0 payments create \
  -F 0xBuyer… -T 0xMerchant… -t 0x3600… -a 1000000 -c 5042002 \
  -p 0xPayerKey…

# Payer using MetaMask — submit externally-produced signature
rail0 payments create <rail0Id> -s 0x1a2b…(130 hex chars)
```

If required flags are missing and stdin is a TTY, an interactive form launches automatically.

---

#### `payments get <rail0Id>`

Fetch the current status of a payment, including live on-chain balances and the transaction list.

```sh
rail0 payments get 0xabc123…
```

| Status | Description |
|--------|-------------|
| `unsigned` | Created; payer signature not yet submitted |
| `signed` | Payer signature stored; awaiting payee action |
| `authorized` | Funds held in escrow; capture available |
| `charged` | One-shot charge executed; refund available |
| `captured` | Funds fully captured by payee |
| `partially_captured` | Partial capture; remaining escrow still capturable |
| `voided` | Authorization cancelled; funds returned to payer |
| `released` | Expired authorization released back to payer |
| `refunded` | Captured funds fully refunded to payer |
| `partially_refunded` | Partial refund; remaining balance still refundable |
| `failed` | Terminal failure; payment cannot proceed |

---

#### `payments authorize <rail0Id>`

Prepare the `authorize()` transaction pulling funds into escrow. Payer signature must be submitted first.

```sh
# Direct key
rail0 payments authorize <rail0Id> -p 0xPayeeKey…

# External wallet
rail0 payments authorize <rail0Id> -w
rail0 payments submit    <rail0Id> -x 0x02f8…
```

---

#### `payments charge <rail0Id>`

Prepare the `charge()` transaction — authorize + capture in one call (no escrow). Payment must have been created with `-m charge`.

```sh
rail0 payments charge <rail0Id> -p 0xPayeeKey…
```

---

#### `payments capture <rail0Id> -a <n>`

Prepare the `capture()` transaction. Partial captures supported: pass `-a` less than `capturableAmount`.

```sh
# Full capture
rail0 payments capture <rail0Id> -a 1000000 -p 0xPayeeKey…

# Partial capture
rail0 payments capture <rail0Id> -a 500000 -p 0xPayeeKey…
```

---

#### `payments void <rail0Id>`

Prepare the `void()` transaction — cancels authorization and returns escrowed funds to payer. Only callable before `authorizationExpiry`, by the payee.

```sh
rail0 payments void <rail0Id> -p 0xPayeeKey…
```

---

#### `payments release <rail0Id>`

Prepare the `release()` transaction — returns remaining escrowed funds to payer after `authorizationExpiry`. Permissionless.

Use `-C` to build the transaction for the payer; omit it for the payee (default).

```sh
# Payee releases
rail0 payments release <rail0Id> -p 0xPayeeKey…

# Payer releases
rail0 payments release <rail0Id> -C 0xBuyer… -p 0xBuyerKey…
```

---

#### `payments approve <rail0Id> -a <n>`

Prepare the ERC-20 `approve()` transaction granting the RAIL0 contract an allowance for a future refund. Required before `refund` if allowance is insufficient.

```sh
rail0 payments approve <rail0Id> -a 1000000 -p 0xPayeeKey…
```

---

#### `payments refund <rail0Id> -a <n>`

Prepare the `refund()` transaction. Partial refunds supported.

```sh
rail0 payments refund <rail0Id> -a 1000000 -p 0xPayeeKey…
```

---

#### `payments submit <rail0Id> -x <0x…>`

Broadcast a signed transaction from an external wallet. Used after any action command run with `-w`/`--prepare`.

Returns 202 immediately. The payment stays in its current state until rail0-indexer confirms the on-chain event. Poll `payments get <rail0Id>` until the payment reaches the expected state and the transaction shows `confirmed`.

```sh
rail0 payments submit <rail0Id> -x 0x02f8…
```

---

## Full flow examples

### Authorize-and-capture — direct key

```sh
export RAIL0_BASE_URL=https://api.rail0.xyz
export RAIL0_API_KEY=your_api_key

# 1. Payer creates payment and signs EIP-712 locally
RAIL0_ID=$(rail0 payments create \
  -F 0xBuyer… -T 0xMerchant… -t 0x3600… -a 1000000 -c 5042002 \
  -p 0xPayerKey… --json | jq -r .rail0_id)

# 2. Payee authorizes (funds → escrow) and polls until confirmed
rail0 payments authorize "$RAIL0_ID" -p 0xPayeeKey…
rail0 payments get "$RAIL0_ID"   # poll until payment=authorized, authorize:confirmed

# 3. Payee captures
rail0 payments capture "$RAIL0_ID" -a 1000000 -p 0xPayeeKey…
rail0 payments get "$RAIL0_ID"   # poll until payment=captured, capture:confirmed
```

---

### Authorize-and-capture — external wallet (MetaMask)

```sh
# 1. Payer gets the signingPayload from the API
rail0 payments create \
  -F 0xBuyer… -T 0xMerchant… -t 0x3600… -a 1000000 -c 5042002 \
  --json
# → note the rail0_id and signingPayload

# 2. Payer signs with MetaMask (eth_signTypedData_v4) → SIGNATURE
rail0 payments create <rail0Id> -s "$SIGNATURE"

# 3. Payee obtains unsigned authorize tx, signs with MetaMask, broadcasts
rail0 payments authorize <rail0Id> -w
rail0 payments submit    <rail0Id> -x "$SIGNED_TX"
rail0 payments get       <rail0Id>   # poll until authorized

# 4. Payee captures
rail0 payments capture <rail0Id> -a 1000000 -w
rail0 payments submit  <rail0Id> -x "$SIGNED_TX"
rail0 payments get     <rail0Id>   # poll until captured
```

---

### Charge (one-shot) — direct key

```sh
RAIL0_ID=$(rail0 payments create \
  -F 0xBuyer… -T 0xMerchant… -t 0x3600… -a 1000000 -c 5042002 -m charge \
  -p 0xPayerKey… --json | jq -r .rail0_id)

rail0 payments charge "$RAIL0_ID" -p 0xPayeeKey…
rail0 payments get "$RAIL0_ID"   # poll until charged
```

---

### Refund — direct key

```sh
rail0 payments approve "$RAIL0_ID" -a 1000000 -p 0xPayeeKey…
rail0 payments refund  "$RAIL0_ID" -a 1000000 -p 0xPayeeKey…
rail0 payments get "$RAIL0_ID"   # poll until refunded
```

---

## Output modes

**Terminal (default):** lipgloss-styled output with highlighted hex values, amounts, and status fields.

**JSON (`--json` or piped):** plain indented JSON on stdout, suitable for `jq` and shell pipelines.

```sh
# Extract the unsigned transaction for external signing
rail0 payments authorize <rail0Id> -w --json | jq -r .unsignedTransaction

# Check payment status in a script
STATUS=$(rail0 payments get <rail0Id> --json | jq -r .status)
```
