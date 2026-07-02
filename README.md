# rail0-cli

Command-line interface for the [RAIL0](https://rail0.xyz) stablecoin payment protocol. Wraps the RAIL0 REST API and exposes every payment lifecycle step as a subcommand. Output is always JSON — syntax-highlighted on a terminal and plain (machine-readable) when piped or when `--json` is passed.

---

## Requirements

- Go ≥ 1.22
- The [rail0-go](https://github.com/DrPierlu/rail0-go) SDK cloned as a sibling directory (used via a `replace` directive in `go.mod`)
- Access to a RAIL0 gateway instance

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
make install            # installs `rail0` + the short `r0` alias into /usr/local/bin
```

`make install` puts the binary on your `PATH` as **`rail0`** and a short alias
**`r0`** (a symlink), so the two are interchangeable — `r0 wallets` is the same
as `rail0 wallets`. Override the location with `make install PREFIX=$HOME/go/bin`.

Manual build (no alias) is still just `go build -o rail0 . && mv rail0 /usr/local/bin/`.

---

## Configuration

Set the API endpoint and credentials via flags, environment variables, or the
config file. Precedence is: **flag  >  env var  >  config file  >  built-in default**.

| Flag         | Environment variable | Description                                      |
|--------------|----------------------|--------------------------------------------------|
| `--base-url` | `RAIL0_BASE_URL`     | RAIL0 gateway base URL                               |
| —            | `RAIL0_TOKEN`        | Bearer token sent in the `Authorization` header  |
| `--debug`    | —                    | Print HTTP request/response details to stderr    |
| `--json`     | —                    | Force plain (uncolored) JSON output (useful in scripts) |
| `--pretty`   | —                    | Key-value/table view instead of JSON (terminal only) |

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

### Signing keys (OS keychain)

Save signing keys in the **OS keychain** and reference them as `@name` on the
`--private-key` flag, instead of pasting a raw key or exporting `RAIL0_PRIVATE_KEY`.
The same `@name` sigil as the address book — the **flag decides** which it means:
`@name` on `--private-key`/`-p` is a key, on `--from`/`--to`/`--address` it's an
address alias.

The secret never touches the config file or your shell history (`keys add`
prompts for it, hidden). Only the key **names** are tracked in `config.yaml` so
they can be listed; the secret lives in the keychain.

```sh
rail0 keys add payer            # prompts for the key (hidden), saves it as "payer"
rail0 keys                      # lists saved key names (never the secret)
rail0 keys address payer        # shows the derived 0x address (not the key)
rail0 keys rm payer             # remove

rail0 payments create -F @buyer -T @payee -t USDC -a 1.00 -c 5042002 -p @payer
```

Backends: macOS Keychain, Windows Credential Manager, Linux Secret Service
(gnome-keyring/KWallet). On a system without one (e.g. **headless Linux**) the
`keys` commands return an error — there, pass `--private-key` or set
`RAIL0_PRIVATE_KEY`.

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
| `--private-key` | `-p` | Caller's secp256k1 private key: 0x-prefixed hex, or `@name` for a key saved in the OS keychain (`rail0 keys add`) |

The signing key is resolved in this order: `--private-key` (`-p` / `--pk`, where `@name` pulls the key from the OS keychain) > `RAIL0_PRIVATE_KEY` env var.

#### Addressing a payment: UUID or rail0_id

Commands that take a payment argument (`get`, `transactions`, and the action commands) accept either form:

- the payment **UUID** `id` — always works, with or without a session;
- the logical **`rail0_id`** (`0x…`, 64 hex) — resolved to the UUID via the authenticated `GET /payments?rail0_id=` filter, so it **requires `rail0 auth login`** (and only finds payments where your wallet is payer or payee). Without a session you'll be asked to log in or pass the UUID.

---

## Commands

### Authentication

#### `auth login`

Obtain a JWT via SIWE (Sign-In With Ethereum) and cache it in `~/.cache/rail0/token`.

```sh
rail0 auth login -p 0xYourPrivateKey…
```

> Note: unlike the payment action commands, `auth login -p` expects a **raw 0x-prefixed hex key** — it does **not** resolve the `@name` keychain form. Pass the hex directly here.

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

`wallets` manages an account's wallets. When **listing**, the account is `--account` if given, otherwise the **logged-in session's** account (`auth login`) — one of the two must be present, else the command errors. Run bare to list; the verbs act on one wallet, named by its id or **0x address** (the wallet's own identity, unique per account). Address-book `@aliases` are for external addresses in `payments`, not for your own wallets.

#### `wallets` — list

Lists the account's wallets, each with its token holdings nested under `tokens`. A wallet with no tokens still appears (with an empty `tokens`), so it is never hidden just because nothing is associated.

```sh
rail0 wallets                                   # logged-in session account
rail0 wallets --account 019e748b-…              # a specific account by id
rail0 wallets --chain-id 5042002 --symbol USDC --active true
```

| Flag | Short | Description |
|------|-------|-------------|
| `--account` | — | Account id to list; defaults to the logged-in session account (one of the two is required) |
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

List the supported blockchains. Use the `chain_id` field with `--chain-id` in `payments create`.

Filter with `--symbol` / `-s` (the chain's native symbol) and/or `--network-type` / `-n`
(`testnet` | `mainnet`); both match case-insensitively.

```sh
rail0 chains
rail0 blockchains                 # alias
rail0 chains --symbol USDC        # by native symbol
rail0 chains --network-type testnet
rail0 chains -s ETH -n mainnet    # combined
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

#### `payments history <rail0Id>`

Print a payment's chronological timeline: the `created` and `signed` milestones plus one entry per on-chain transaction (`authorize`, `capture`, `void`, `release`, `refund`, `dispute`, `close_dispute`), oldest first. Only the significant fields are shown per entry (time, event, status, amount, tx hash, block). It is assembled client-side from a single `payments get` — there is no dedicated gateway endpoint.

Requires a session — run `rail0 auth login` first (the `rail0_id` argument is resolved through the authenticated lookup).

```sh
rail0 payments history 0xabc123…
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
rail0 payments capture <rail0Id> -a 1.00 -p 0xPayeeKey…

# Partial capture
rail0 payments capture <rail0Id> -a 0.50 -p 0xPayeeKey…
```

---

#### `payments void <rail0Id>`

Prepare the `void()` transaction — cancels authorization and returns escrowed funds to payer. Only callable before `authorizationExpiry`, by the payee.

```sh
rail0 payments void <rail0Id> -p 0xPayeeKey…
```

---

#### `payments release <rail0Id>`

Prepare the `release()` transaction — returns remaining escrowed funds to payer after `authorizationExpiry`. Permissionless: both payer and payee can call it. The caller is whoever the signing key belongs to (derived from the key), so pass the payer's key to release as the payer.

```sh
# Payee releases
rail0 payments release <rail0Id> -p 0xPayeeKey…

# Payer releases (just use the payer's key)
rail0 payments release <rail0Id> -p 0xPayerKey…
```

---

#### `payments refund <rail0Id> -a <n>`

Prepare the `refund()` transaction. Partial refunds supported.

```sh
rail0 payments refund <rail0Id> -a 1.00 -p 0xPayeeKey…
```

---

## Full flow examples

### Authorize-and-capture — direct key

```sh
export RAIL0_BASE_URL=https://api.rail0.xyz
export RAIL0_TOKEN=your_jwt_token

# 1. Payer creates payment and signs EIP-712 locally
RAIL0_ID=$(rail0 payments create \
  -F 0xBuyer… -T 0xMerchant… -t USDC -a 1.00 -c 5042002 \
  -p 0xPayerKey… --json | jq -r .rail0_id)

# 2. Payee authorizes (funds → escrow) and polls until confirmed
rail0 payments authorize "$RAIL0_ID" -p 0xPayeeKey…
rail0 payments get "$RAIL0_ID"   # poll until payment=authorized, authorize:confirmed

# 3. Payee captures
rail0 payments capture "$RAIL0_ID" -a 1.00 -p 0xPayeeKey…
rail0 payments get "$RAIL0_ID"   # poll until payment=captured, capture:confirmed
```

---

### Charge (one-shot) — direct key

```sh
RAIL0_ID=$(rail0 payments create \
  -F 0xBuyer… -T 0xMerchant… -t USDC -a 1.00 -c 5042002 -m charge \
  -p 0xPayerKey… --json | jq -r .rail0_id)

rail0 payments charge "$RAIL0_ID" -p 0xPayeeKey…
rail0 payments get "$RAIL0_ID"   # poll until charged
```

---

### Refund — direct key

```sh
rail0 payments refund "$RAIL0_ID" -a 1.00 -p 0xPayeeKey…
rail0 payments get "$RAIL0_ID"   # poll until refunded
```

---

## Output modes

Every command prints the gateway response with all fields the gateway returns.

**Terminal (default):** syntax-highlighted, indented, JSON-*like* output — keys, strings, hex values (addresses/hashes), numbers, booleans, and payment status are colored for readability. Object keys are printed **unquoted** (colour already sets them apart), so this view is intentionally not strict JSON.

**Plain JSON (`--json` or piped):** strict, valid indented JSON with no ANSI colour on stdout, suitable for `jq` and shell pipelines.

**Pretty (`--pretty`):** a key-value (single record) or table (list) view with the same highlighting, as an alternative terminal layout. Terminal only — piping still emits plain JSON.

```sh
# Extract the rail0_id of a freshly created payment
RAIL0_ID=$(rail0 payments create -F 0xBuyer… -T 0xMerchant… -t USDC -a 10.50 -c 5042002 -p 0xPayerKey… --json | jq -r .rail0_id)

# Check payment status in a script
STATUS=$(rail0 payments get <rail0Id> --json | jq -r .status)
```
