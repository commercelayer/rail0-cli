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
| —            | `RAIL0_TOKEN`        | Bearer token sent in the `Authorization` header  |
| `--debug`    | —                    | Print HTTP request/response details to stderr    |
| `--json`     | —                    | Force plain JSON output (useful in scripts)      |

Minimal setup:

```sh
export RAIL0_BASE_URL=https://api.rail0.xyz
export RAIL0_TOKEN=your_jwt_token
```

Or use `rail0 auth login` to obtain and cache a JWT via SIWE authentication.

---

## Signing modes

### Payer — `payments create`

The payer creates the payment and signs the EIP-712 payload. Two modes:

| Mode | Command | Description |
|------|---------|-------------|
| Direct key | `payments create … -p 0xPayerKey…` | Creates the payment, signs locally, submits signature in one step |
| External wallet | `payments create … -s 0x1a2b…` with `<rail0Id>` | Submit a signature produced by MetaMask / `eth_signTypedData_v4` |

### Payee — action commands

Every on-chain action command (`authorize`, `charge`, `capture`, `void`, `release`, `refund`, `dispute`) is **atomic**: pass `--private-key` (`-p`) and the CLI prepares the unsigned transaction, signs it locally, and broadcasts it in a single step.

| Flag | Short | Description |
|------|-------|-------------|
| `--private-key` | `-p` | Caller's secp256k1 private key (0x-prefixed hex); signs and broadcasts in one step |

---

## Commands

### Authentication

#### `auth login`

Obtain a JWT via SIWE (Sign-In With Ethereum) and cache it in `~/.cache/rail0/token`.

```sh
rail0 auth login -p 0xYourPrivateKey…
```

#### `auth status`

Print the wallet address, account id, and expiry encoded in the cached JWT.

```sh
rail0 auth status
```

#### `auth logout`

Remove the cached JWT.

```sh
rail0 auth logout
```

---

### Accounts

#### `accounts wallets <account_id>`

List the account's wallet-token rows (one row per wallet/token pairing on a chain). Requires authentication.

```sh
rail0 accounts wallets acct_abc123
rail0 accounts wallets acct_abc123 --chain-id 5042002 --symbol USDC --active true
```

| Flag | Short | Description |
|------|-------|-------------|
| `--chain-id` | — | Filter by chain id |
| `--symbol` | `-s` | Filter by token symbol (e.g. `USDC`) |
| `--active` | — | Filter by active flag (`true` or `false`) |

---

#### `accounts wallet <account_id> <wallet_id>`

Fetch a single wallet.

```sh
rail0 accounts wallet acct_abc123 wlt_xyz789
```

---

#### `accounts create-wallet <account_id>`

Register a new EVM wallet address on an account. Requires authentication.

```sh
rail0 accounts create-wallet acct_abc123 --address 0xABCD… --label "Treasury"
```

| Flag | Short | Required | Description |
|------|-------|----------|-------------|
| `--address` | `-A` | yes | EVM wallet address (0x-prefixed) |
| `--label` | `-l` | no | Human-readable label |

---

#### `accounts update-wallet <account_id> <wallet_id>`

Update label or active status on a wallet. Requires authentication.

```sh
rail0 accounts update-wallet acct_abc123 wlt_xyz789 --label "New label"
rail0 accounts update-wallet acct_abc123 wlt_xyz789 --active false
```

| Flag | Short | Description |
|------|-------|-------------|
| `--label` | `-l` | New wallet label |
| `--active` | — | Set active flag (`true` or `false`) |

---

#### `accounts delete-wallet <account_id> <wallet_id>`

Remove a wallet from an account. Requires authentication.

```sh
rail0 accounts delete-wallet acct_abc123 wlt_xyz789
```

---

#### `accounts balances <account_id> <wallet_id>`

Read a wallet's live on-chain balances across the configured chains. Each chain reports the native gas-token balance and the active token balances, or an error if that chain's RPC was unreachable (reported per chain, so one dead RPC never hides the others). Requires authentication.

```sh
rail0 accounts balances acct_abc123 wlt_xyz789
rail0 accounts balances acct_abc123 wlt_xyz789 --chain-id 84532 --symbol USDC
```

| Flag | Short | Description |
|------|-------|-------------|
| `--chain-id` | | Restrict to one chain id |
| `--symbol` | `-s` | Restrict tokens to this symbol (e.g. USDC) |

---

### Chains

#### `chains`

List all supported blockchains. Use the `chain_id` field with `--chain-id` in `payments create`.

```sh
rail0 chains
```

---

### Tokens

#### `tokens`

List the token catalog. Filter by chain with `--chain-id`.

```sh
rail0 tokens
rail0 tokens --chain-id 5042002
```

---

### Webhooks

Manage webhook subscriptions for the authenticated account. The shared secret used to verify delivery signatures is returned only once — on `create` and `rotate-secret`. All webhook commands require authentication.

```sh
rail0 webhooks list --topic payments.captured --active true
rail0 webhooks get <webhook_id>
rail0 webhooks create --name "capture hook" --callback-url https://example.com/hook --topic payments.captured
rail0 webhooks update <webhook_id> --callback-url https://example.com/new
rail0 webhooks enable <webhook_id>
rail0 webhooks disable <webhook_id>
rail0 webhooks rotate-secret <webhook_id>
rail0 webhooks reset-circuit <webhook_id>
rail0 webhooks event-callbacks <webhook_id> --status failed
rail0 webhooks delete <webhook_id>
```

| Command | Description |
|---------|-------------|
| `list` | List webhooks (`--topic`, `--active`, `--circuit-state`, `--sort`) |
| `get` | Fetch a single webhook |
| `create` | Register a webhook (`--name`, `--callback-url`, `--topic`; returns shared secret) |
| `update` | Patch `--name`, `--callback-url`, or `--topic` |
| `enable` / `disable` | Toggle delivery |
| `rotate-secret` | Issue a new shared secret (returned once) |
| `reset-circuit` | Reset the delivery circuit breaker and re-enable |
| `event-callbacks` | List delivery attempts (`--status`, `--sort`) |
| `delete` | Remove a webhook |

---

### Health

#### `health`

Report gateway liveness/readiness, including database connectivity and active chain/contract counts.

```sh
rail0 health
```

---

### Payments

#### `payments create`

Create a payment, sign the EIP-712 payload, and submit the payer's signature in one step.

| Flag | Short | Required | Default | Description |
|------|-------|----------|---------|-------------|
| `--from` | `-F` | yes | — | Payer wallet address (0x-prefixed) |
| `--to` | `-T` | yes | — | Payee wallet address (0x-prefixed) |
| `--token` | `-t` | yes | — | Token symbol as in `tokens` (e.g. `USDC`) |
| `--amount` | `-a` | yes | — | Amount in token base units (e.g. `1000000` for 1 USDC) |
| `--chain-id` | `-c` | yes | — | EVM chain ID as in `chains` (e.g. `5042002`) |
| `--mode` | `-m` | no | `authorize` | `authorize` (escrow + capture) or `charge` (immediate) |
| `--payment-file` | `-f` | no | — | JSON file with the full request body |
| `--private-key` | `-p` | no* | — | Payer's key — creates and signs locally in one step |
| `--signature` | `-s` | no* | — | Submit an externally-produced EIP-712 signature for an existing `<rail0Id>` |

\* one of `-p` or `-s` required (mutually exclusive)

```sh
# Payer with direct key access — create + sign in one step
rail0 payments create \
  -F 0xBuyer… -T 0xMerchant… -t USDC -a 1000000 -c 5042002 \
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

#### `payments list`

List payments for the authenticated wallet (requires login).

```sh
rail0 payments list
rail0 payments list --sort -created_at
```

---

#### `payments transactions <rail0Id>`

List on-chain transactions for a payment.

```sh
rail0 payments transactions 0xabc123…
```

---

#### `payments authorize <rail0Id>`

Prepare the `authorize()` transaction pulling funds into escrow. Payer signature must be submitted first.

```sh
rail0 payments authorize <rail0Id> -p 0xPayeeKey…
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

#### `payments refund <rail0Id> -a <n>`

Prepare the `refund()` transaction. Partial refunds supported.

```sh
rail0 payments refund <rail0Id> -a 1000000 -p 0xPayeeKey…
```

---

## Full flow examples

### Authorize-and-capture — direct key

```sh
export RAIL0_BASE_URL=https://api.rail0.xyz
export RAIL0_TOKEN=your_jwt_token

# 1. Payer creates payment and signs EIP-712 locally
RAIL0_ID=$(rail0 payments create \
  -F 0xBuyer… -T 0xMerchant… -t USDC -a 1000000 -c 5042002 \
  -p 0xPayerKey… --json | jq -r .rail0_id)

# 2. Payee authorizes (funds → escrow) and polls until confirmed
rail0 payments authorize "$RAIL0_ID" -p 0xPayeeKey…
rail0 payments get "$RAIL0_ID"   # poll until payment=authorized, authorize:confirmed

# 3. Payee captures
rail0 payments capture "$RAIL0_ID" -a 1000000 -p 0xPayeeKey…
rail0 payments get "$RAIL0_ID"   # poll until payment=captured, capture:confirmed
```

---

### Payer signs externally (MetaMask)

The payer's EIP-712 signature can be produced by an external wallet and submitted
separately — the payee's on-chain operations remain atomic (`-p`).

```sh
# 1. Payer creates payment, gets signingPayload
rail0 payments create \
  -F 0xBuyer… -T 0xMerchant… -t USDC -a 1000000 -c 5042002 \
  --json
# → note the rail0_id and signingPayload

# 2. Payer signs with MetaMask (eth_signTypedData_v4) → SIGNATURE, then submits it
rail0 payments create <rail0Id> -s "$SIGNATURE"

# 3. Payee authorizes and captures with a direct key
rail0 payments authorize <rail0Id> -p 0xPayeeKey…
rail0 payments get       <rail0Id>   # poll until authorized
rail0 payments capture   <rail0Id> -a 1000000 -p 0xPayeeKey…
rail0 payments get       <rail0Id>   # poll until captured
```

---

### Charge (one-shot) — direct key

```sh
RAIL0_ID=$(rail0 payments create \
  -F 0xBuyer… -T 0xMerchant… -t USDC -a 1000000 -c 5042002 -m charge \
  -p 0xPayerKey… --json | jq -r .rail0_id)

rail0 payments charge "$RAIL0_ID" -p 0xPayeeKey…
rail0 payments get "$RAIL0_ID"   # poll until charged
```

---

### Refund — direct key

```sh
rail0 payments refund "$RAIL0_ID" -a 1000000 -p 0xPayeeKey…
rail0 payments get "$RAIL0_ID"   # poll until refunded
```

---

## Output modes

**Terminal (default):** lipgloss-styled output with highlighted hex values, amounts, and status fields.

**JSON (`--json` or piped):** plain indented JSON on stdout, suitable for `jq` and shell pipelines.

```sh
# Extract the unsigned transaction for external signing
rail0 payments authorize <rail0Id> --prepare --json | jq -r .unsignedTransaction

# Check payment status in a script
STATUS=$(rail0 payments get <rail0Id> --json | jq -r .status)
```
