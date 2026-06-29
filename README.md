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

Set the API endpoint and credentials via flags, environment variables, or the
config file. Precedence is: **flag  >  env var  >  config file  >  built-in default**.

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

### Persisted defaults (`config`)

Persist non-sensitive defaults in the config file so you don't repeat them on
every command. Run bare to see each setting's effective value and where it comes
from. (Secrets like private keys are never stored here.)

```sh
rail0 config                                  # list settings + origin (flag/env/file/default)
rail0 config set base-url http://localhost:9292
rail0 config get base-url                      # effective value + origin
rail0 config unset base-url
rail0 config path                              # config file location
```

A stored value is the lowest-priority source, so `--base-url` or `RAIL0_BASE_URL`
still override it for a single command.

### Address book (aliases)

Save frequently-used addresses and reference them as `@name` on any address flag
(`--from` / `--to` / `--address`). Stored in `config.yaml` in the OS config dir
(`~/.config/rail0/` on Linux, `~/Library/Application Support/rail0/` on macOS).

Alias **names are unique**: adding a name that already exists is rejected — `rm`
it first to repoint it. The same address may carry more than one name.

```sh
rail0 aliases add payee 0xAbC123…
rail0 aliases add buyer 0xDeF456…
rail0 wallets alias 0xAbC123… payee  # same as 'aliases add', address-first order
rail0 aliases                        # bare command lists the address book
rail0 aliases rm payee               # remove (e.g. to repoint a name)

rail0 payments create --from @buyer --to @payee -t USDC -a 10.50 -c 84532 -p 0xPayerKey…
```

The JWT session token is **not** stored in the config file — it stays in the OS cache dir (written by `rail0 auth login`).

---

## Signing modes

### Payer — `payments create`

The payer creates the payment and signs the EIP-712 payload with their key
(`--private-key` / `-p`), submitting the signature in one step:

```sh
rail0 payments create -F 0xBuyer… -T 0xMerchant… -t USDC -a 10.50 -c 5042002 -p 0xPayerKey…
```

### Payee — action commands

Every on-chain action command (`authorize`, `charge`, `capture`, `void`, `release`, `refund`, `dispute`) is **atomic**: it prepares the unsigned transaction, signs it locally, and broadcasts it in a single step.

| Flag | Short | Description |
|------|-------|-------------|
| `--private-key` | `-p` | Caller's secp256k1 private key (0x-prefixed hex) |

The signing key is resolved in this order: `--private-key` (`-p` / `--pk`) > `RAIL0_PRIVATE_KEY` env var.

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

### Wallets

`wallets` manages the **logged-in account's** wallets — the account is taken from the session (`auth login`), never passed as an argument; with no session the command errors. Run bare to list; the verbs act on one wallet, named by its id or **0x address** (the wallet's own identity, unique per account). Address-book `@aliases` are for external addresses in `payments`, not for your own wallets.

#### `wallets` — list

Lists the account's wallets, each with its token holdings nested under `tokens`. A wallet with no tokens still appears (with an empty `tokens`), so it is never hidden just because nothing is associated.

```sh
rail0 wallets
rail0 wallets --chain-id 5042002 --symbol USDC --active true
```

| Flag | Short | Description |
|------|-------|-------------|
| `--chain-id` | — | Restrict the nested tokens to this chain id (wallets still listed) |
| `--symbol` | `-s` | Restrict the nested tokens to this symbol, e.g. `USDC` (wallets still listed) |
| `--active` | — | Filter the wallets by active flag (`true` or `false`) |

#### `wallets get <wallet>`

```sh
rail0 wallets get 0xABCD…
rail0 wallets get <wallet_uuid>
```

#### `wallets create`

Register a new EVM wallet address on the account.

```sh
rail0 wallets create --address 0xABCD… --label "Treasury"
```

| Flag | Short | Required | Description |
|------|-------|----------|-------------|
| `--address` | `-A` | yes | EVM wallet address (0x-prefixed, or `@alias`) |
| `--label` | `-l` | no | Human-readable label |

#### `wallets update <wallet>`

```sh
rail0 wallets update 0xABCD… --label "New label" --active false
```

| Flag | Short | Description |
|------|-------|-------------|
| `--label` | `-l` | New wallet label |
| `--active` | — | Set active flag (`true` or `false`) |

#### `wallets delete <wallet>`

```sh
rail0 wallets delete 0xABCD…
```

#### `wallets balances <wallet>`

Read a wallet's live on-chain balances across the configured chains. Each chain reports the native gas-token balance and the active token balances, or an error if that chain's RPC was unreachable (reported per chain, so one dead RPC never hides the others).

```sh
rail0 wallets balances 0xABCD… --chain-id 84532 --symbol USDC
```

| Flag | Short | Description |
|------|-------|-------------|
| `--chain-id` | | Restrict to one chain id |
| `--symbol` | `-s` | Restrict tokens to this symbol (e.g. USDC) |

#### `wallets alias <address> <name>`

Save an address-book alias, in address-first order — handy right after copying an address out of `wallets`. It's the same as [`aliases add <name> <address>`](#address-book-aliases) (arguments reversed) and obeys the same **unique-name** rule: an existing name is rejected, `rm` it first.

```sh
rail0 wallets alias 0xABCD… treasury
rail0 payments create --to @treasury -t USDC -a 10.50 -c 84532 -p 0xPayerKey…
```

---

### Chains

#### `chains` (alias: `blockchains`)

List all supported blockchains. Use the `chain_id` field with `--chain-id` in `payments create`.

```sh
rail0 chains
rail0 blockchains   # alias
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

Create a payment, sign the EIP-712 payload with the payer's key, and submit the signature in one step.

| Flag | Short | Required | Default | Description |
|------|-------|----------|---------|-------------|
| `--from` | `-F` | yes | — | Payer wallet address (0x-prefixed) |
| `--to` | `-T` | yes | — | Payee wallet address (0x-prefixed) |
| `--token` | `-t` | yes | — | Token symbol as in `tokens` (e.g. `USDC`) |
| `--amount` | `-a` | yes | — | Amount in token units (e.g. `10.50`) |
| `--chain-id` | `-c` | yes | — | EVM chain ID as in `chains` (e.g. `5042002`) |
| `--mode` | `-m` | no | `authorize` | `authorize` (escrow + capture) or `charge` (immediate) |
| `--payment-file` | `-f` | no | — | JSON file with the full request body |
| `--private-key` | `-p` | yes | — | Payer's key — creates and signs locally in one step |

```sh
rail0 payments create \
  -F 0xBuyer… -T 0xMerchant… -t USDC -a 10.50 -c 5042002 \
  -p 0xPayerKey…
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
