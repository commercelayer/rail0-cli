# rail0-cli

Command-line interface for the [RAIL0](https://rail0.xyz) stablecoin payment protocol. Wraps the RAIL0 gateway's REST API and exposes every payment lifecycle step as a subcommand. Output is always JSON — syntax-highlighted on a terminal and plain (machine-readable) when piped or when `--json` is passed.

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

`bin/build` does the same without `make` (build + install `rail0` + `r0`):

```sh
bin/build                       # into /usr/local/bin
PREFIX="$HOME/go/bin" bin/build # into a dir you own (no sudo)
```

Manual build (no alias) is still just `go build -o rail0 . && mv rail0 /usr/local/bin/`.

Check the installed version with `rail0 version` (or `rail0 --version`); `bin/build`
stamps it from `git describe`.

### Reference docs

`bin/docs` regenerates the full command reference straight from the command tree —
Markdown in `docs/cli/` (for a docs site) and man pages in `docs/man/`. Run it after
changing any command, flag, or help text. The generator lives in `tools/gendocs`, so
its doc dependencies never enter the shipped binary.

---

## Shell completion

Generate a completion script for your shell (see `rail0 completion --help` for the
per-shell install path):

```sh
rail0 completion zsh  > "${fpath[1]}/_rail0"          # zsh
rail0 completion bash > /etc/bash_completion.d/rail0  # bash
rail0 completion fish > ~/.config/fish/completions/rail0.fish
```

Beyond commands and flags, `keys rm` / `keys address` complete the saved key names.

---

## Configuration

Set the gateway endpoint and credentials via flags, environment variables, or the
config file. Precedence is: **flag  >  env var  >  config file  >  built-in default**.

| Flag         | Environment variable | Description                                      |
|--------------|----------------------|--------------------------------------------------|
| `--base-url` | `RAIL0_BASE_URL`     | RAIL0 gateway base URL                               |
| —            | `RAIL0_TOKEN`        | Bearer token sent in the `Authorization` header  |
| `--debug`    | —                    | Print HTTP request/response details to stderr    |
| `--json`     | —                    | Force plain (uncolored) JSON output (useful in scripts) |
| `--pretty`   | —                    | Key-value/table view instead of JSON (terminal only) |
| `--quiet` / `-q` | —                | Print only the result's id/hash, one per line — no JSON parser needed |

### Scripting

`--quiet` prints just the primary identifier of the result (`rail0_id`, else the
transaction hash, else `id`; one per line for a list), so you can capture it
directly: `id=$(rail0 payments create … -q)`.

Exit codes distinguish the failure class: `0` success, `3` unauthorized, `4`
forbidden, `5` not found, `6` conflict, `7` rejected (422), `8` gateway error
(5xx), `9` network, `1` otherwise.

Errors carry an actionable hint where the cause is known — e.g. a `422` on
`capture` explains the amount exceeds the capturable balance, and an on-chain
`RefundExpired` revert says the refund window has closed.

The list commands (`payments list`, `payments transactions`, `payments disputes`,
`webhooks list`) take `--page` and `--per-page`; `payments list` also accepts
`--all` to walk every page and return the full set.

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

### Environments (`env`)

Switch a whole bundle of settings at once instead of setting each key by hand.
Today the only per-environment parameter is `base-url`:

| Environment | base-url |
| --- | --- |
| `development` | `http://localhost:9292` |
| `staging` | `https://rail0-gateway-staging.fly.dev` |
| `production` | `https://api.rail0.xyz` |

```sh
rail0 env                 # list environments + parameters, marking the current one
rail0 env staging         # switch to staging (writes its settings to the config file)
rail0 env staging -d      # ...and also print the resulting parameters
rail0 env current         # show the active environment
rail0 env current -d      # ...with every parameter's effective value + origin
```

`env <name>` writes the environment's settings through the same config file as
`config set`, so precedence is unchanged — `--base-url` / `RAIL0_BASE_URL` still
override for a single command. `env current` reports the environment the
*effective* configuration matches (so it's accurate even if you set a value with
`config set`), or `custom` if it matches none.

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

**Token/chain discovery from the payee.** On a terminal, if you give `--to` but
omit `--token` and/or `--chain-id`, the CLI looks up what that merchant accepts
(public `GET /payment_methods` for the payee address) and prompts you to pick
from those combinations — a single accepted method is used automatically. So a
buyer who knows only the merchant's address can run:

```sh
rail0 payments create -F @buyer -T 0xMerchant… -a 10.50 -p @payer
# → prompts "Select the token and chain the merchant accepts" (skipped if only one)
```

If you don't pass `--to` (or that merchant exposes no matching method), the
interactive form still helps: `--chain-id` and `--token` are offered as
**pickers from the public catalog** (`GET /blockchains`, then `GET /tokens` for
the chosen chain) rather than free-text — a single active chain/token is taken
automatically. All of this needs a **reachable gateway**; if none is reachable
it falls back to plain free-text prompts.

**Description prompt.** On a terminal, if `--description` (`-d`) is omitted the
CLI always asks for it — even when every required flag was supplied. It's
optional (leave it blank to omit), but it's the reconciliation reference, so
you're prompted to fill it in consciously rather than forget the flag.

**Validation happens up front.** A value you pass on the command line is checked
before any prompt is shown — a malformed `--amount`, an unknown `@alias`, or a
`--token` that isn't listed on the chain fails immediately, so you never answer
the description prompt only to have the whole command rejected afterwards.
Interactive prompts validate the same way, field by field: the form won't move to
the next field until the current one is valid (chain is asked before token, since
a token symbol is only meaningful on a chosen chain).

### Payee — action commands

Every on-chain action command (`authorize`, `charge`, `capture`, `void`, `release`, `refund`, `dispute`) is **atomic**: it prepares the unsigned transaction, signs it locally, and broadcasts it in a single step.

| Flag | Short | Description |
|------|-------|-------------|
| `--private-key` | `-p` | Caller's secp256k1 private key: 0x-prefixed hex, or `@name` for a key saved in the OS keychain (`rail0 keys add`) |
| `--wait` | `-w` | Block until the operation confirms on-chain (or fails) instead of returning at the `202` |
| `--timeout` | — | Max time to wait with `--wait` (default `2m`) |
| `--yes` | `-y` | Skip the confirmation prompt on the irreversible ops (`void`/`release`/`refund`); required to run them non-interactively |

**Waiting for confirmation.** The gateway returns `202` immediately and the indexer
confirms the on-chain event a moment later. Pass `--wait` to have the command poll
until the operation's transaction is `confirmed` (then print the settled payment)
or `failed`, instead of polling `payments get` by hand. You can also wait on an
existing payment: `rail0 payments wait <rail0Id> [--until <status>]` (default: until
no transaction is still in flight).

**Confirmations.** The irreversible actions (`void`, `release`, `refund`) prompt for
a `y/N` confirmation before broadcasting; pass `--yes` to skip it (and to run them
in a non-interactive shell, where they otherwise refuse). `keys rm` and
`wallets delete` confirm the same way.

The signing key is resolved in this order: `--private-key` (`-p` / `--pk`, where `@name` pulls the key from the OS keychain) > `RAIL0_PRIVATE_KEY` env var. If none is set, the CLI prompts for the key **hidden** on a terminal (or reads one line from a piped stdin), so it never lands in the shell history or process arguments — the same masked input as `keys add`.

#### Addressing a payment: UUID or rail0_id

Commands that take a payment argument (`get`, `transactions`, and the action commands) accept either form, passed straight through to the gateway, which resolves both:

- the payment **UUID** `id`;
- the logical **`rail0_id`** (`0x…`, 64 hex).

Either works with or without a session (subject to the endpoint's own auth — the action commands are still payee/payer-gated). The CLI no longer pre-resolves the `rail0_id` client-side; a value that is neither a UUID nor a `rail0_id` is a clean 404.

---

## Commands

### Authentication

#### `auth login`

Obtain a JWT via SIWE (Sign-In With Ethereum) and cache it in `~/.cache/rail0/token`. **Any wallet can sign in — no gateway account is required** (a buyer signs in with just its key); an account only unlocks merchant features.

```sh
rail0 auth login -p 0xYourPrivateKey…
rail0 auth login -p @payee -t          # also print the token + its expiry
```

On success the command prints the session (address, expiry) but **not** the raw token. Pass `--token` / `-t` to also print the token itself along with its duration and expiry timestamp, or fetch it later with [`rail0 token`](#token).

> Note: `auth login -p` accepts the same key forms as the payment action commands — a raw **0x-prefixed hex key**, an **`@name`** OS-keychain reference (`rail0 keys add`), or, with `-p` omitted, the **`RAIL0_PRIVATE_KEY`** environment variable.

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

#### `token`

Print the cached session JWT and its expiry, so it can be copied or piped. If you're not logged in (or the token has expired), it suggests logging in instead. This is the **singular** session token — distinct from [`rail0 tokens`](#tokens), which lists the gateway's accepted payment tokens.

```sh
rail0 token
export RAIL0_TOKEN="$(rail0 token --json | jq -r .token)"
```

---

### Wallets

`wallets` manages **your own** account's wallets and requires a login (`auth login`) — the gateway now protects the whole account section with SIWE. When **listing**, `--account` defaults to the logged-in session's account; if given it must be your own account (another account's id is rejected with 403). Run bare to list; the verbs act on one wallet, named by its id or **0x address** (the wallet's own identity, unique per account). Address-book `@aliases` are for external addresses in `payments`, not for your own wallets. To discover **another** merchant's accepted wallets/tokens without logging in, use [`payment-methods`](#payment-methods).

#### `wallets` — list

Lists the account's wallets, each with its token holdings nested under `tokens`. A wallet with no tokens still appears (with an empty `tokens`), so it is never hidden just because nothing is associated.

```sh
rail0 wallets                                   # logged-in session account
rail0 wallets --account 019e748b-…              # your own account, stated explicitly
rail0 wallets --chain-id 5042002 --symbol USDC --active true
rail0 wallets --default true --sort -created_at
```

| Flag | Short | Description |
|------|-------|-------------|
| `--account` | — | Your account id to list; defaults to the logged-in session account (must be your own — requires login) |
| `--chain-id` | — | Restrict the nested tokens to this chain id (wallets still listed) |
| `--symbol` | `-s` | Restrict the nested tokens to this symbol, e.g. `USDC` (wallets still listed) |
| `--active` | — | Filter the wallets by active flag (`true` or `false`) |
| `--default` | — | Restrict the nested token holdings to the default one (`true` or `false`) |
| `--sort` | — | Sort wallets, e.g. `-created_at` |

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

### Payment methods

`payment-methods` is the **public**, buyer-facing counterpart to `wallets`: it discovers which wallets and tokens a merchant accepts, **without a login**. Where `wallets` lists your own account (behind SIWE), this identifies **another** merchant by exactly one of `--account` (all its active wallets) or `--address` (just that one wallet). It returns the merchant's active wallets, each with the active tokens (and chains) it accepts — the values a payer needs to build a `payments create`.

```sh
rail0 payment-methods --account 019e748b-…       # all the merchant's active wallets
rail0 payment-methods --address 0xABCD…          # just that wallet
rail0 payment-methods --address @payee           # …or an address-book alias
```

| Flag | Short | Description |
|------|-------|-------------|
| `--account` | — | Merchant account id — list all its active wallets |
| `--address` | `-A` | A merchant wallet address (`0x…` or `@alias`) — list just that wallet |

Exactly one of `--account` / `--address` is required (mirrors the gateway's `GET /payment_methods`).

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

List the token catalog. Filter by chain with `--chain-id` and/or by token symbol with `--symbol` / `-s` (case-insensitive).

```sh
rail0 tokens
rail0 tokens --chain-id 5042002
rail0 tokens --symbol USDC
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
| `event-callbacks` | List delivery attempts (`--status`, `--topic`, `--payment-id`, `--since`, `--until`, `--sort`) |
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
| `--charge` | `-C` | no | (off) | Create with mode `charge` (immediate, no escrow); default is `authorize` (escrow + capture) |
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
| `voided` | Authorization cancelled (only before any capture); funds returned to payer |
| `released` | Full authorization released back to payer (total release, nothing captured) |
| `refunded` | Payment fully settled by refund (both escrow and refundable drained) |
| `partially_refunded` | Legacy — no longer produced; a partial refund now leaves the status unchanged |
| `failed` | Terminal failure; payment cannot proceed |

---

#### `payments list`

List payments for the authenticated wallet (requires login). Filter with
`--chain-id`, `--disputed`, `--min-amount` / `--max-amount` (token base units),
and `--created-from` / `--created-to` (ISO-8601); sort with `--sort`.

```sh
rail0 payments list
rail0 payments list --sort -created_at
rail0 payments list --chain-id 5042002 --disputed
rail0 payments list --min-amount 1000000 --created-from 2026-01-01T00:00:00Z
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

Works with or without a session — `payments history` is a public read; the `rail0_id` (or UUID) argument is resolved by the gateway in-path.

```sh
rail0 payments history 0xabc123…
```

---

> **Interactive prompts (operations).** On a terminal, the action commands below
> (`authorize`/`charge`/`capture`/`void`/`release`/`refund`/`dispute`/`dispute close`)
> prompt for anything you didn't pass: omit `<rail0Id>` to pick from a list of your
> payments the operation can act on; omit `-a`/`--amount` (capture/refund) or
> `--reason` (dispute) to be asked for it. The signing key (`-p`) is never
> prompted — pass it, set `RAIL0_PRIVATE_KEY`, or use an `@keychain-name`. In a
> non-interactive shell a missing required value is a clear error instead.

#### `payments authorize <rail0Id>`

Prepare the `authorize()` transaction pulling funds into escrow. Payer signature must be submitted first.

```sh
rail0 payments authorize <rail0Id> -p 0xPayeeKey…
```

---

#### `payments charge <rail0Id>`

Prepare the `charge()` transaction — authorize + capture in one call (no escrow). Payment must have been created with `-C` / `--charge`.

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
  -F 0xBuyer… -T 0xMerchant… -t USDC -a 1.00 -c 5042002 -C \
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
