# rail0-cli

Command-line interface for the [RAIL0](https://rail0.xyz) stablecoin payment protocol. Wraps the RAIL0 gateway's REST API and exposes every payment lifecycle step as a subcommand. Output is always JSON — syntax-highlighted on a terminal and plain (machine-readable) when piped or when `--json` is passed.

**New here? [Your first payment](#your-first-payment) is a start-to-finish walkthrough** — install, save the buyer and merchant addresses, sign in, and settle one payment on a testnet. The rest of this file is reference material you can come back to.

---

## Safe re-runs (`--idempotency-key`)

Every on-chain operation takes `--idempotency-key`. The hazard it closes is not this
process retrying — it does not. It is a **script re-running the command**: a `--wait`
that timed out on a slow chain exits non-zero, and the natural response to a non-zero
capture is to capture again. Without a key the gateway cannot tell that re-run from a
deliberate second partial capture, because by then the first transaction has left
`pending` — so it opens a second one and the money moves twice.

```bash
rail0 payments capture <id> --amount 50.00 --idempotency-key "$ORDER_ID"
```

With a key, a re-run returns the first transaction and nothing moves: the CLI does not
sign or broadcast it again, and with `--wait` it waits on that original transaction. If
the first attempt never got as far as broadcasting, the re-run finishes it. The same key with
a different amount is refused `422 idempotency_key_reused`, so a key you forgot to
rotate is reported rather than silently honoured. Keys are scoped to the payment.

## Contents

- [Requirements](#requirements)
- [Installation](#installation) — [release binary](#download-a-release-binary-recommended) · [from source](#build-from-source-contributors) · [`rail0 init`](#guided-setup-rail0-init)
- [**Your first payment**](#your-first-payment) — the guided path
- [Shell completion](#shell-completion)
- [Configuration](#configuration) — [defaults](#persisted-defaults-config) · [environments](#environments-env) · [address book](#address-book-aliases) · [signing keys](#signing-keys-os-keychain)
- [Signing modes](#signing-modes) — who signs what, and when
- [Commands](#commands) — the full reference, including [repairing a webhook](#repairing-a-webhook-that-stopped-delivering)
- [Full flow examples](#full-flow-examples) — authorize/capture, charge, refund
- [Output modes](#output-modes)

---

## Requirements

- Go ≥ 1.25 (matches the `go` directive in `go.mod`)
- The [rail0-go](https://github.com/commercelayer/rail0-go) SDK cloned as a sibling directory (used via a `replace` directive in `go.mod`)
- Access to a RAIL0 gateway instance (for the commands that call it — building/`rail0 --help` need no gateway)

---

## Installation

### Download a release binary (recommended)

Every `v*` tag publishes static, cross-platform binaries — no Go toolchain, no
sibling checkout, nothing to compile. Grab the archive for your platform from
[Releases](https://github.com/commercelayer/rail0-cli/releases), or in one line:

```sh
# macOS arm64 — swap darwin/arm64 for your platform (darwin|linux|windows, amd64|arm64)
VERSION=$(curl -fsSL https://api.github.com/repos/commercelayer/rail0-cli/releases/latest | sed -n 's/.*"tag_name": *"v\([^"]*\)".*/\1/p')
curl -fsSL "https://github.com/commercelayer/rail0-cli/releases/download/v${VERSION}/rail0_${VERSION}_darwin_arm64.tar.gz" | tar xz
sudo mv rail0 /usr/local/bin/
rail0 init          # guided setup
```

Each archive also carries the **shell completions** (`completions/`), the **man
pages** (`man/man1/`) and the command reference (`docs/`), so an installed binary is
self-documenting:

```sh
sudo cp man/man1/*.1 /usr/local/share/man/man1/   # then: man rail0
cp completions/_rail0 "${fpath[1]}/"              # zsh; see `rail0 completion --help`
```

Verify the download against `checksums.txt` from the same release.

> **Building from source is not an option for external users**, and that is why the
> release binaries matter: the build needs the `rail0-go` SDK as a sibling checkout
> (`replace` directive in `go.mod`) and that repository is private.

### Build from source (contributors)

Clone this repo and the `rail0-go` SDK **as siblings**, then build. rail0-cli imports the SDK through `replace github.com/commercelayer/rail0-go =>
../rail0-go` (see `go.mod`), so `rail0-go` must sit next to `rail0-cli`.

Both repositories must be present as siblings:

```
parent-directory/
├── rail0-cli/   ← this repo
└── rail0-go/    ← Go SDK (replace directive points here)
```

```sh
git clone https://github.com/commercelayer/rail0-cli
git clone https://github.com/commercelayer/rail0-go
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

Manual build (no alias) is `make build && mv rail0 /usr/local/bin/` — **not** a bare
`go build`, which leaves the version at its `dev` default.

Check the installed version with `rail0 version` (or `rail0 --version`). Every build
path stamps it from `git describe`: `make build` does the compile, `bin/build` and
`make install` delegate to it, and goreleaser passes the release tag as `VERSION`.
That matters more than it looks — a binary that reports `dev` cannot be told apart
from one built weeks earlier, and a stale CLI fails in ways that point at the server
rather than at itself.

### Guided setup (`rail0 init`)

```sh
rail0 init
```

Walks the four steps a first payment needs, each skippable: pick a gateway (writes
`base-url`), check it answers, optionally save a signing key in the OS keychain
(referenced later as `@name`), optionally sign in with it. Safe to re-run on a
half-configured machine.

It composes the existing commands rather than reimplementing them, so `rail0 env`,
`rail0 health`, `rail0 keys add` and `rail0 auth login` are exactly what it runs — and
`keys add`'s refusal to overwrite an orphaned keychain entry still applies. In a
script, run those four directly; `init` is interactive and says so instead of hanging
on a prompt.

The gateway is checked **before** the key and the sign-in on purpose: both are wasted
effort against a gateway that is not there, and a connection error right after pasting
a private key reads as if the key were the problem.

### Reference docs

`bin/docs` regenerates the full command reference straight from the command tree —
Markdown in `docs/cli/` (for a docs site) and man pages in `docs/man/`. Run it after
changing any command, flag, or help text. The generator lives in `tools/gendocs`, so
its doc dependencies never enter the shipped binary.

---

## Your first payment

From an empty machine to a settled payment. Nothing below is demo-only
scaffolding — every command here is one you keep using.

### First, the two sides

A rail0 payment always has two, and keeping them straight from the start saves
most of the confusion later:

| | signs | pays gas | needs a gateway account |
| --- | --- | --- | --- |
| **buyer** (`payer`) | the EIP-3009 authorization, **off-chain** | no | **no** — any wallet can pay |
| **merchant** (`payee`) | the **on-chain** transactions | yes | yes, for wallets and analytics |

The buyer never sends a transaction. They sign a message; the merchant submits it
and pays the gas. That is why the buyer needs no account and no separate login,
and why the merchant does.

This walkthrough plays **both** roles from one machine, which is the honest way to
try it. In production they are two people on two machines.

### 1. Install and point at a gateway

```sh
rail0 init
```

Four questions — gateway, reachability check, an optional key, an optional
sign-in — each skippable, safe to re-run. If you'd rather not be asked:

```sh
rail0 env development     # or staging / production — writes base-url once
rail0 health              # confirm it answers before going further
```

### 2. Save the two addresses

Give the buyer and merchant wallets names, so no 42-character hex ever appears
in a command again. `@name` then works on **any** address flag (`--from`,
`--to`, `--address`):

```sh
rail0 aliases add buyer    0xBuyerAddress…
rail0 aliases add merchant 0xMerchantAddress…

rail0 aliases             # bare: lists the address book
```

Names are unique — to repoint one, `rail0 aliases rm buyer` first. One address
may carry several names.

### 3. Save the two keys

Same `@name` sigil, different store: keys live in the **OS keychain**, never in
the config file and never in your shell history. `keys add` prompts for the
secret hidden.

```sh
rail0 keys add buyer-key         # prompts (hidden) for the buyer's private key
rail0 keys add merchant-key      # and the merchant's

rail0 keys address buyer-key     # the derived 0x address, to check you saved the right one
```

> **Which store does `@name` hit?** The flag decides, not the name: `@x` on
> `--private-key`/`-p` is a key, on `--from`/`--to`/`--address` it's an address.
> That is why `buyer` and `buyer-key` above are named apart — nothing forces it,
> but you will thank yourself when reading a script six months from now.

On headless Linux there may be no keychain; there, pass `--private-key` or set
`RAIL0_PRIVATE_KEY`.

### 4. Sign in as the merchant

```sh
rail0 auth login -p @merchant-key
rail0 auth status                  # address, account, expiry
```

**The buyer does not need this step.** `payments create` signs in on its own with
the payer key, so a buyer with a wallet and nothing else can pay. The merchant
logs in because the action commands and the account views require it.

### 5. Check what the merchant accepts

Public, no login — this is what a buyer's client would call first:

```sh
rail0 payment-methods -A @merchant
```

It returns the chains and tokens that merchant can be paid on. Pick one for the
next step; if you have no preference, **Arc Testnet (`5042002`)** settles in about
four confirmations, where Base Sepolia wants sixty — a much shorter wait for a
first run.

### 6. Buyer creates and signs the payment

```sh
RAIL0_ID=$(rail0 payments create \
  --from @buyer --to @merchant \
  --token USDC --amount 1.00 --chain-id 5042002 \
  --private-key @buyer-key --quiet)

echo "$RAIL0_ID"
```

One step, not two: it creates the payment **and** signs the EIP-3009
authorization locally. The key never leaves your machine — the gateway stores the
signature, not the secret. Omit `--private-key` and it prompts, hidden.

`--quiet`/`-q` prints just the id, which is what you want in a variable. Drop it
to see the whole payment.

> `--amount` is a **human decimal** — `1.00` means one USDC, not one base unit.

### 7. Merchant authorizes, then captures

```sh
# funds move into escrow — and --wait blocks until the chain confirms it
rail0 payments authorize "$RAIL0_ID" -p @merchant-key --wait

# and out of escrow to the merchant
rail0 payments capture "$RAIL0_ID" --amount 1.00 -p @merchant-key --wait
```

**`--wait` is the flag that makes this pleasant.** Without it the commands return
the moment the transaction is broadcast, which is not the same as settled: the
payment sits at `signed` until rail0-indexer sees the event. With it, the command
blocks until the state actually changes, prints progress while it does, and exits
non-zero if the operation failed — so a script reads top to bottom instead of
growing a polling loop.

A payment that reaches `expired` stops the wait rather than running to the deadline: no
capture can land on a lapsed authorization, so waiting is over. The message says the
escrow is still recoverable and names the `release` command for it — `expired` is
terminal for the *wait*, not for the money.

Check the result any time:

```sh
rail0 payments get "$RAIL0_ID" --pretty
```

### That's the whole loop

Authorize-then-capture is the escrow flow. Two variants worth knowing now that
the shape is familiar:

- **One-shot**: add `-C`/`--charge` at step 6 and run `rail0 payments charge`
  instead of authorize+capture. Funds go straight from buyer to merchant, no
  escrow window.
- **Undo**: `rail0 payments refund <id> -a 1.00 -p @merchant-key --wait` sends
  captured funds back. Before anything is captured, `void` releases the hold.

Full worked versions of all three are in [Full flow examples](#full-flow-examples).

---

## Shell completion

Generate a completion script for your shell (see `rail0 completion --help` for the
per-shell install path):

```sh
rail0 completion zsh  > "${fpath[1]}/_rail0"          # zsh
rail0 completion bash > /etc/bash_completion.d/rail0  # bash
rail0 completion fish > ~/.config/fish/completions/rail0.fish
```

Beyond commands and flags: `keys rm` / `keys address` complete the saved key names,
the address flags (`--from`, `--to`, `--address`, `--payer`, `--payee`) complete the
address book as `@name`, `rail0 env <TAB>` completes the environment names, and the
finite enums (`--status`, `--mode`, `--by`, `--interval`, `--until`) complete their
accepted values.

---

## Configuration

Set the gateway endpoint and credentials via flags, environment variables, or the
config file. Precedence is: **flag  >  env var  >  config file  >  built-in default**.

| Flag         | Environment variable | Description                                      |
|--------------|----------------------|--------------------------------------------------|
| `--base-url` | `RAIL0_BASE_URL`     | RAIL0 gateway base URL                               |
| `--debug`    | —                    | Print HTTP request/response details to stderr    |
| `--json`     | —                    | Force plain (uncolored) JSON output (useful in scripts) |
| `--pretty`   | —                    | Key-value/table view instead of JSON (terminal only) |
| `--quiet` / `-q` | —                | Print only the result's id/hash, one per line — no JSON parser needed |
| — | `RAIL0_HTTP_TIMEOUT` | Per-request HTTP timeout in seconds (default 30). Distinct from the wait commands' `--timeout`, which is the polling deadline |

#### If the config file goes bad

A file the CLI cannot parse is **ignored, with a warning naming it** — commands keep working on flags, env vars and defaults, so nothing is blocked. Reads refuse rather than act on settings they could not read.

Any command that *writes* the config replaces it: `config set`, `config unset`, `env <name>`, `aliases add`/`rm`, `keys add`/`rm`. They say on stderr that the old contents are gone — which they are, since keys inside unparseable YAML cannot be recovered by any means. So the fastest repair is simply to pick an environment again:

```sh
rail0 config path      # where it lives, if you'd rather look first
rail0 env staging      # rewrites the file, valid
```

### Amounts and expiries in the output

Every payment the CLI prints — `payments get`, `payments list` (one page or `--all`),
the lifecycle commands — carries a **human companion** next to each raw amount:
`amount_human`, `capturable_amount_human`, `refundable_amount_human`. Transaction rows
(`payments transactions`, `payments transaction`) carry `amount_human` too, on their
payment's scale; their gas figures stay in the chain's native wei. The two expiries
get `authorization_expiry_at` / `refund_expiry_at` (RFC3339) and
`authorization_expires_in` / `refund_expires_in` ("2h 5m", or "expired") beside the raw
epoch integers.

The raw fields are untouched, so a script reading `amount` keeps working — the
companions are additive. They are **omitted** when the token's decimals are unknown
(a token missing from `rail0 tokens`), because a base-unit integer in a field named
`_human` would be worse than no field: `4050000` reads as four million rather than
4.05, and that is a six-order-of-magnitude error someone can act on. The interactive
payment picker, which has no script to keep compatible, instead labels it
`4050000 (base units)`.

### Scripting

`--quiet` prints just the primary identifier of the result (`rail0_id`, else the
transaction hash, else `id`; one per line for a list), so you can capture it
directly: `id=$(rail0 payments create … -q)`.

The one exception is a response carrying a **one-time secret**: `webhooks create` and
`webhooks rotate-secret` print the `shared_secret` under `-q`, because it is shown only
once and the id can be looked up again with `webhooks list`:
`secret=$(rail0 webhooks rotate-secret <id> -q)`.

Exit codes distinguish the failure class: `0` success, `3` unauthorized, `4`
forbidden, `5` not found, `6` conflict, `7` rejected (422), `8` gateway error
(5xx), `9` network, `10` rate-limited (429), `130` cancelled by the user, `1`
otherwise — including a `--wait` that ran out of time, even when its last poll hit a
network error: the operation may still be in flight, so it never exits `9`.

`10` is the one class where re-running the identical command later is the correct
response, which is why it is not folded into the generic failure.

`130` is what you get by pressing Ctrl-C (or ESC) at any prompt, and by answering `N`
to a destructive confirmation — the POSIX convention for SIGINT, so `&&` chains stop
there. Nothing was executed; it is a cancellation, not a failure.

Errors are printed as the gateway's own sentence plus, where the CLI can add
something, the command to run next: a `422` on `capture` explains that the amount
exceeds the capturable balance and points at `rail0 payments get <id>`, and a wallet
with no gas is named as such rather than as an opaque RPC failure.

The failures you actually hit are mostly not raised by RAIL0: the token reverts when
the buyer is short of balance (`insufficient_token_balance`), signed with the wrong
key (`invalid_token_signature`) or reuses an authorization
(`authorization_already_used`), and the node refuses a transaction that never reaches
the chain (`insufficient_gas_funds`, `nonce_too_low`). All of these arrive with a
title and a sentence instead of raw revert bytes or `rpc_error: -32000`.

A failed transaction shows the same code and sentence in `rail0 payments
transactions <id>`, whether it reverted on-chain or was refused before broadcast.

The list commands (`payments list`, `payments transactions`, `payments disputes`,
`disputes`, `wallets`, `webhooks list`, `webhooks event-callbacks`) take `--page` and
`--per-page`; `payments list` also accepts `--all` to walk every page and return the
full set.

Minimal setup:

```sh
export RAIL0_BASE_URL=https://api.rail0.xyz
rail0 auth login -p @payee     # caches a JWT via SIWE; every command reads it from there
```

The session lives only in the OS cache dir, keyed **per gateway**
(`~/.cache/rail0/tokens/<digest of the base URL>`), so each environment keeps its own
session across switches. There is no environment variable to supply it: the token is a
bearer credential, and the environment is the one place it would leak into every child
process and `env` dump.

`rail0 auth logout` **revokes** the token on the gateway (its `jti` goes into the
denylist, so the JWT stops being accepted before it expires) and then removes the local
copy. Removing the local copy alone is not a logout — the JWT stays valid for the rest
of its lifetime, so anyone holding another copy keeps the session. If the gateway cannot
be reached, the local file is removed anyway (an unreachable gateway is a normal reason
to log out) and the command warns that the token was NOT revoked.

### Persisted defaults (`config`)

Persist non-sensitive defaults in the config file so you don't repeat them on
every command. Run bare to see each setting's effective value and where it comes
from. (Secrets like private keys are never stored here.)

```sh
rail0 config                                  # list settings + origin (flag/env/file/default)
rail0 config set base-url http://localhost:9292
rail0 config set chain-id 5042002              # fills --chain-id/-c when omitted
rail0 config set token USDC                    # fills --token/-t when omitted
rail0 config get chain-id                      # effective value + origin
rail0 config unset chain-id
rail0 config path                              # config file location
```

| Key | Fills | Env var | Built-in default |
| --- | --- | --- | --- |
| `base-url` | `--base-url` | `RAIL0_BASE_URL` | `https://api.rail0.xyz` |
| `chain-id` | `--chain-id` / `-c` | `RAIL0_CHAIN_ID` | — |
| `token` | `--token` / `-t` | `RAIL0_TOKEN_SYMBOL` | — |
| `output` | `--json` / `--pretty` / `--quiet` | `RAIL0_OUTPUT` | `auto` |
| `http-timeout` | — (per-request timeout, seconds) | `RAIL0_HTTP_TIMEOUT` | `30` |

A stored value is the lowest-priority source, so the matching flag or env var still
overrides it for a single command.

**Addresses and signing keys are not settable here**, and that is deliberate. Each
already has a mechanism for not retyping the value — the [address book](#address-book-aliases)
for payer/payee (`--from @buyer`), the OS keychain for keys (`-p @payer`) — so a
second home in the config file would duplicate them and, worse, make the two parties
and the signer of a fund-moving command resolve from a file rather than from the
command line. `RAIL0_PRIVATE_KEY` stays the only name that carries a raw key.

`--amount` has no stored default for a related reason: it is the one field that
differs every time, and a default there would silently price a payment.
`--description` is left out as the mirror image — it is the reconciliation reference,
and a constant one is worse than none.

Note that `token`'s env var is `RAIL0_TOKEN_SYMBOL`, not `RAIL0_TOKEN` — that name was
a session-token source until it was removed, and reusing it would make a stale
exported JWT arrive as a payment token symbol.

### Environments (`env`)

Switch a whole bundle of settings at once instead of setting each key by hand:

| Environment | base-url | chain-id | token |
| --- | --- | --- | --- |
| `development` | `http://localhost:9292` | `5042002` | `USDC` |
| `staging` | `https://rail0-gateway-staging.fly.dev` | `5042002` | `USDC` |
| `production` | `https://api.rail0.xyz` | — | — |

Production carries no chain or token, and that is not an omission: the two other
environments serve testnets, so Arc Testnet (4 confirmations, the quickest active one)
is a safe default there, while the production chain set is a deployment decision —
inventing one here would send a real payment to a chain nobody chose.

```sh
rail0 env                 # list the environments, marking the current one
rail0 env -d              # ...with the parameters each one declares
rail0 env staging         # switch to staging (writes its settings to the config file)
rail0 env staging -d      # ...and also print the resulting parameters
rail0 env current         # show the active environment
rail0 env current -d      # ...with every parameter's effective value + origin
```

A switch writes the config file, so it is **refused** while `--base-url` or
`RAIL0_BASE_URL` points somewhere else: either one outranks the file, and the switch
would change nothing the commands use. Unset the override, then switch. Base URLs are
compared the way sessions are stored, so a trailing `/` or a different case does not
make the current gateway look like a different one.

After a switch the environment-managed settings are **exactly** the target's: a key
the target does not declare is **removed**, not left behind. Applying an environment
used to only write its own keys, so moving from `development` to `production` — which
declares base-url alone — kept `chain-id: 5042002` and `token: USDC` armed: the
production gateway with an Arc Testnet default still loaded, inherited in silence
because nothing wrote it at that moment.

Because that discards stored values, a switch that would clear something **names them
and asks first**:

```
Switching to "production" clears settings it does not define:
  chain-id = 5042002
  token = USDC
Clear them and switch? [y/N]:
```

The prompt appears **only** when there is something to lose — one that appeared every
time is one you would clear by reflex. Declining changes nothing at all, including the
session on the gateway you were leaving: the question is asked before any of it. Pass
`--yes` in a script (without it, and with no terminal, the switch refuses rather than
hanging), and `cleared` is reported in the result so `-q`/`--json` callers see it too.

Settings outside the bundle — `output`, `http-timeout` — are your own preferences and
a switch never touches them.

Switching **ends the session on the gateway you are leaving**: its token is revoked
(denylist) and the local copy removed, so the file left behind is worthless. Sessions are
per gateway, so a switch used to simply orphan the old token — a bearer credential on
disk, valid for up to 24h, for a session nobody is watching. The result says which of the
three happened: `previous_session: none | revoked | abandoned` (`abandoned` carries a
detail, e.g. the old gateway was unreachable — the switch still proceeds, because moving
away from a down environment is a normal reason to switch).

The result also reports **`logged_in`**, and prints the login hint when it is false.
Sessions are per gateway, so arriving unauthenticated is the normal outcome of a
switch — it used to be silent, and surfaced later as a `401` that looked like the
switch had broken something.

`env <name>` writes the environment's settings through the same config file as
`config set`, so precedence is unchanged — `--base-url` / `RAIL0_BASE_URL` still
override for a single command. `env current` reports the environment the
*effective* configuration matches (so it's accurate even if you set a value with
`config set`), or `custom` if it matches none. The match is on `base-url` alone: the
gateway is what identifies an environment, and the other parameters are conveniences
the switch also writes — comparing those too would report `custom` for someone who
merely changed chain after `env staging`.

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

rail0 payments create --from @buyer --to @merchant -t USDC -a 10.50 -c 84532 -p @buyer-key
```

The JWT session token is **not** stored in the config file — it stays in the OS cache dir (written by `rail0 auth login`).

### Signing keys (OS keychain)

Save signing keys in the **OS keychain** and reference them as `@name` on the
`--private-key` flag, instead of pasting a raw key or exporting `RAIL0_PRIVATE_KEY`.
The same `@name` sigil as the address book — the **flag decides** which it means:
`@name` on `--private-key`/`-p` is a key, on `--from`/`--to`/`--address` it's an
address alias. When a signing command has no key and prompts for one interactively,
that hidden prompt also accepts `@name` (not just a raw 0x key).

The secret never touches the config file or your shell history (`keys add`
prompts for it, hidden); it lives in the keychain.

**There is no `rail0 keys` listing, on purpose.** A keychain cannot be enumerated,
so a listing could only replay a copy of the names kept next to the config — and
that copy drifts from what the keychain actually holds. Delete `config.yaml` to
start clean, or copy one between machines, and such a list names keys that are gone
and omits keys that are there, with nothing in its output to say which you are
looking at. A list that can be wrong is worse than no list. Ask about one key at a
time instead: `keys address <name>` reads the keychain itself and either derives the
address or tells you the key is not there, so it cannot be stale.

The names still sit in `config.yaml`, but only as a **hint for shell completion** —
a wrong suggestion at the TAB key is the whole cost of a stale entry.

That drift is also why `keys add` **refuses** to write over an entry the keychain
holds but the names do not list (a private key cannot be recovered, so it errors
rather than asking a yes/no you'd answer by reflex), and why `keys rm` clears one
anyway — otherwise the name would be neither addable nor removable. `keys address
<name>` is how you inspect one before deciding.

```sh
rail0 keys add payer            # prompts for the key (hidden), saves it as "payer"
rail0 keys address payer        # shows the derived 0x address (never the key)
rail0 keys rm payer             # remove

rail0 payments create -F @buyer -T @payee -t USDC -a 1.00 -c 5042002 -p @payer
```

Backends: macOS Keychain, Windows Credential Manager, Linux Secret Service
(gnome-keyring/KWallet). On a system without one (e.g. **headless Linux**) the
`keys` commands return an error — there, pass `--private-key` or set
`RAIL0_PRIVATE_KEY`.

---

## Signing modes

### Payer — `payments create` / `payments sign`

The payer creates the payment and signs the EIP-712 payload with their key
(`--private-key` / `-p`), submitting the signature in one step:

```sh
rail0 payments create -F @buyer -T @merchant -t USDC -a 10.50 -c 5042002 -p @buyer-key
```

> **Auto sign-in.** The gateway requires a SIWE session for every `/payments`
> write and binds the payment to the payer (the `payer` must equal the signed-in
> address). `payments create` handles this: it signs in with the payer's key (the
> same `-p` key it signs the payload with) when there's no valid session for that
> address — so no separate `rail0 auth login` is needed, and the session's address
> matches `--from`. If a session for a *different* address is already stored, that
> one is left alone: the sign-in serves this command only, and the CLI says so.

> **Replay safety.** Each `payments create` sends a fresh `Idempotency-Key`, so a
> command retried after a lost response (a timeout, a 429) cannot mint a second
> payment — the gateway returns the first one. Pass `--idempotency-key` to reuse a
> key of your own across processes.

**If the signature step fails** (a dropped connection, a rate limit, the wrong
key), the payment exists at `unsigned` with no signature. Don't create it again —
deposit the signature on the existing one:

```sh
rail0 payments sign 0xRail0Id… -p @buyer-key
```

**Token/chain discovery from the payee.** On a terminal, if you give `--to` but
omit `--token` and/or `--chain-id`, the CLI looks up what that merchant accepts
(public `GET /payment_methods` for the payee address) and prompts you to pick
from those combinations — a single accepted method is used automatically. So a
buyer who knows only the merchant's address can run:

```sh
rail0 payments create -F @buyer -T @merchant -a 10.50 -p @payer
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
| `--timeout` | — | Max time to wait with `--wait` (default `15m`). The gateway is notified only once the chain calls the block settled, so settlement is minutes, not seconds: ~2 min on Base/OP Sepolia and ~11 min on Arbitrum Sepolia (its `safe` block sits ~2640 blocks back, on L1 batch cadence), ~0 on Arc and Amoy. The old `2m` default could not be reached on Arbitrum at all — and a `--wait` that gives up exits non-zero, which a script reads as a failed capture. Lower it on a fast chain if you want a stuck operation reported sooner |
| `--yes` | `-y` | Skip the confirmation prompt on any irreversible op (`void`/`release`/`refund`, `keys rm`, `wallets delete`, `webhooks delete`/`rotate-secret`); required to run them non-interactively |

**Waiting for confirmation.** The gateway returns `202` immediately and the indexer
confirms the on-chain event a moment later. Pass `--wait` to have the command poll
until the operation's transaction is `confirmed` (then print the settled payment)
or `failed`, instead of polling `payments get` by hand. You can also wait on an
existing payment: `rail0 payments wait <rail0Id> [--until <status>]` (default: until
no transaction is still in flight).

**Confirmations.** Every irreversible action prompts for a `y/N` confirmation first;
pass `--yes` to skip it (and to run it in a non-interactive shell, where it otherwise
refuses rather than blocking on a hidden prompt). That covers every fund-moving
on-chain op — `capture`, `charge`, `void`, `release`, `refund`, `dispute`,
`dispute close` — and, identically, `keys rm`, `wallets delete`,
`webhooks delete` and `webhooks rotate-secret`. A sweep test asserts the whole
surface, so a new fund-moving command cannot ship ungated.

`webhooks rotate-secret` is gated for a reason that is easy to miss: it is not
undoable by rotating back. The previous secret is gone, and from that moment every
consumer still verifying `X-Rail0-Signature` against it rejects deliveries — so a
mistyped webhook id silently breaks production verification.

The signing key is resolved in this order: `--private-key` (`-p` / `--pk`, where `@name` pulls the key from the OS keychain) > `RAIL0_PRIVATE_KEY` env var. If none is set, the CLI prompts for the key **hidden** on a terminal (or reads one line from a piped stdin), so it never lands in the shell history or process arguments — the same masked input as `keys add`.

#### Addressing a payment: UUID or rail0_id

Commands that take a payment argument (`get`, `transactions`, and the action commands) accept either form, passed straight through to the gateway, which resolves both:

- the payment **UUID** `id`;
- the logical **`rail0_id`** (`0x…`, 64 hex).

Either works with or without a session (subject to the endpoint's own auth — the action commands are still payee/payer-gated). The CLI no longer pre-resolves the `rail0_id` client-side; a value that is neither a UUID nor a `rail0_id` is a clean 404.

---

## Commands

### Authentication

Every `/payments` operation requires a SIWE session (the gateway rejects anonymous writes and binds a created payment to the payer). Reads and merchant action commands need a prior `auth login`; **`payments create` signs in automatically** with the payer key, so a buyer needs no separate login step (see [Payer — `payments create`](#payer--payments-create)).

#### `auth login`

Obtain a JWT via SIWE (Sign-In With Ethereum) and cache it under `~/.cache/rail0/tokens/<digest of the base URL>` — one file per gateway; the exact path is printed on success and by `auth status`. **Any wallet can sign in — no gateway account is required** (a buyer signs in with just its key); an account only unlocks merchant features.

```sh
rail0 auth login -p 0xYourPrivateKey…
rail0 auth login -p @merchant-key -t   # also print the token + its expiry
```

The cache is **per gateway**, so switching with [`rail0 env`](#environments-env) switches session along with it rather than logging you out — come back and the earlier session is still there, if it has not expired.

On success the command prints the session (address, expiry) but **not** the raw token. Pass `--token` / `-t` to also print the token itself along with its duration and expiry timestamp, or fetch it later with [`rail0 token`](#token).

> Note: `auth login -p` accepts the same key forms as the payment action commands — a raw **0x-prefixed hex key**, an **`@name`** OS-keychain reference (`rail0 keys add`), or, with `-p` omitted, the **`RAIL0_PRIVATE_KEY`** environment variable.

#### `auth status`

Print the wallet address, account id, and expiry encoded in the cached JWT.

```sh
rail0 auth status
```

#### `auth logout`

Revoke the session on the gateway and remove the cached JWT. The result says what
happened: `session` is `none` (nothing was stored), `revoked`, or `abandoned` (removed
locally but not revoked — `session_detail` says why), the same vocabulary `env` uses
for `previous_session`.

```sh
rail0 auth logout
rail0 auth logout --json | jq -r .session
```

#### `auth revoke-all`

End **every** session of an address: the ones on this machine, on other machines,
and any a thief is holding. `auth logout` revokes one token, the one stored here;
this is per **address**. The gateway records a cutoff instant and refuses every
earlier session of the address, including this machine's.

It is authorized by a fresh signature of the address's **key**, not by the session,
because whoever reacts to a leaked key holds the wallet, not the stolen token. The
signed message carries its own statement ("Sign out of RAIL0 everywhere"), so a
login signature cannot be replayed to trigger it. It asks first (`--yes` skips).

```sh
rail0 auth revoke-all -p @merchant-key
```

The result carries `cutoff_at`, and `local_session` says what happened to this
machine's stored session: `removed` (it was this address's, and is now dead),
`kept` (another address's, still valid) or `none`. Sign in again afterwards. If the
**key** itself leaked, ending its sessions does not make it safe: move the funds.

#### `token`

Print the cached session JWT and its expiry, so it can be copied or piped. If you're not logged in (or the token has expired), it suggests logging in instead. This is the **singular** session token — distinct from [`rail0 tokens`](#tokens), which lists the gateway's accepted payment tokens.

```sh
rail0 token
TOKEN="$(rail0 token --json | jq -r .token)"   # e.g. to call the gateway with curl
```

Exporting it does **not** configure the CLI — it reads the session from its cache file
and nowhere else.

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
| `--token-active` | — | Restrict the nested token holdings by active flag (`true` or `false`) |
| `--sort` | — | Sort wallets, e.g. `-created_at` |
| `--page` / `--per-page` | — | Page through the wallets (the gateway's default page is 25) |

#### `wallets get <wallet>`

```sh
rail0 wallets get 0xABCD…
rail0 wallets get <wallet_uuid>
```

#### `wallets create`

Register a new EVM wallet address on the account. The gateway requires a **SIWE
proof-of-ownership** of the address being added, so this command needs the
private key **of that address**: it fetches a nonce, builds and signs the same
EIP-4361 message as `auth login` (for the added address), and submits the
message + signature. The added address need not be your session address (a
merchant may register several payee wallets), but the key must match `--address`
— the command derives the signer from the key and rejects a mismatch. Addresses
are globally unique; one already registered to any account is refused.

The key is resolved like every signing command: `-p` as a raw `0x` hex key, as
`@name` for an OS-keychain key (`rail0 keys add`), or `RAIL0_PRIVATE_KEY` (you
are prompted if none is set).

```sh
rail0 wallets create --address 0xABCD… -p @treasury --label "Treasury"
rail0 wallets create --address @merchant -p 0xYourKey…
```

| Flag | Short | Required | Description |
|------|-------|----------|-------------|
| `--address` | `-A` | yes | EVM wallet address to add (0x-prefixed, or `@alias`) |
| `--private-key` | `-p` | yes¹ | Signing key **of the address being added**: `0x` hex, `@name` (keychain), or `RAIL0_PRIVATE_KEY` |
| `--label` | `-l` | no | Human-readable label |

¹ Falls back to `RAIL0_PRIVATE_KEY`, then an interactive hidden prompt.

#### `wallets update <wallet>`

```sh
rail0 wallets update 0xABCD… --label "New label" --active false
```

| Flag | Short | Description |
|------|-------|-------------|
| `--label` | `-l` | New wallet label |
| `--active` | — | Set active flag (`true` or `false`) |

`--active false` is the merchant's **revocation** switch, and it does more than hide a row:

- the address stops being offered by [`payment-methods`](#payment-methods), so buyers can no longer discover it and no new payment can name it;
- webhooks stop being dispatched for payments to it — deliberately: deactivation says "stop trusting this address", and an outbound push about its payments would contradict that.

Existing payments are unaffected. They carry their own payee address and settle whatever the flag says now. Reversible with `--active true`.

#### `wallets delete <wallet>`

**A soft delete, despite the verb.** The gateway sets `active=false` and keeps the row — it is the *same operation* as `wallets update <wallet> --active false`, with the same consequences and the same way back. Nothing is removed, nothing on-chain is touched, no funds move.

Both spellings exist because the API exposes a `DELETE` verb as well as the flag. Use whichever reads better where you are writing it.

```sh
rail0 wallets delete 0xABCD…                      # same as…
rail0 wallets update 0xABCD… --active false       # …this
rail0 wallets update 0xABCD… --active true        # back on
```

> Contrast [`webhooks delete`](#webhooks), which really does destroy the record.

#### `wallets balances <wallet>`

Read a wallet's live on-chain balances across the configured chains. Each chain reports the native gas-token balance and the active token balances, or an error if that chain's RPC was unreachable (reported per chain, so one dead RPC never hides the others).

```sh
rail0 wallets balances 0xABCD… --chain-id 84532 --symbol USDC
```

| Flag | Short | Description |
|------|-------|-------------|
| `--chain-id` | | Restrict to one chain id |
| `--symbol` | `-s` | Restrict tokens to this symbol (e.g. USDC) |

#### `wallets tokens` — what a wallet accepts

A wallet's **token holdings** — the (chain, token) pairs it accepts — are what
[`payment-methods`](#payment-methods) offers buyers and what payment creation checks: a
wallet with no active holding cannot be paid. So registering a wallet is only half the
setup; this is the other half.

```sh
rail0 wallets tokens add 0xABCD… --chain-id 84532 --token USDC --default
rail0 wallets tokens disable 0xABCD… USDC --chain-id 84532    # alias: remove
rail0 wallets tokens enable 0xABCD… USDC --chain-id 84532
```

- `add` is an upsert: a disabled holding is re-enabled, never duplicated. `--token` is a
  symbol resolved on `--chain-id`, or the token's `0x` address; both fall back to the
  configured `chain-id` / `token`.
- `enable` / `disable` take `<token>` as a symbol, an address, or the `token_id` shown by
  `rail0 wallets`, matched against the holdings the wallet actually has. When the wallet
  holds one symbol on several chains, `--chain-id` picks the chain; the CLI never guesses.
- A wallet with active holdings has exactly **one default**, the method buyers see first.
  The first holding takes it; `add … --default` moves it. The default cannot be disabled
  (`422 default_payment_method`), so move it first.
- Disabling is soft and reversible, and asks first (`--yes` skips the prompt). Payments
  already created with that token are unaffected.

#### `wallets alias <address> <name>`

Save an address-book alias, in address-first order — handy right after copying an address out of `wallets`. It's the same as [`aliases add <name> <address>`](#address-book-aliases) (arguments reversed) and obeys the same **unique-name** rule: an existing name is rejected, `rm` it first.

```sh
rail0 wallets alias 0xABCD… treasury
rail0 payments create --to @treasury -t USDC -a 10.50 -c 84532 -p @buyer-key
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

Manage webhook subscriptions for the authenticated account. A webhook subscribes to one or more payment-lifecycle topics and delivers a signed callback to your endpoint — one subscription means **one** shared secret and **one** circuit breaker however many topics it carries, and each delivery names the event that fired in `X-Rail0-Topic`. Two subscriptions for the same callback URL must not overlap, since the same event arriving twice under two different secrets is indistinguishable from a duplicate at the receiving end. All webhook commands require authentication.

#### Two ways to be off — and only one `enable`

A webhook delivers only while it is **active**, and there are two independent switches:

| | what turned it off | what turns it back on |
| --- | --- | --- |
| **disabled** | you ran `disable` | `enable` |
| **circuit open** | 10 consecutive failed *events* tripped the breaker | `reset-circuit`, and nothing else |

They are not interchangeable, and this is the trap worth knowing before you need it: **a tripped breaker also marks the webhook disabled**, so `enable` looks like it worked — it returns 200 with the webhook in the body — while the webhook stays silent, because the breaker is still open.

Read `active` in the response rather than trusting the exit code. It is false whenever either switch is off, and `circuit_state` says which.

The breaker never closes on its own: nothing is delivered while it is open, so no later success can reset it.

#### Repairing a webhook that stopped delivering

```sh
rail0 webhooks event-callbacks <webhook_id> --status failed   # 1. what was it failing on?
rail0 webhooks update <webhook_id> --callback-url https://…   # 2. fix it
rail0 webhooks reset-circuit <webhook_id>                     # 3. let it deliver again
```

Step 3 is not optional after a tripped breaker — `update` changes the URL but not the active state, so repointing a broken webhook at a working endpoint leaves it exactly as dead as it was. And doing 3 before 2 just spends another 10 events reopening the breaker.

#### The shared secret

Returned **once**, on `create` and `rotate-secret`. The gateway stores a hash, not the value, so it cannot be read back — capture it when it appears. `rotate-secret` is the recovery, not a lookup.

Rotation has **no overlap window**: the old secret stops verifying immediately, so any delivery already in flight was signed with the old one and a receiver that has not been updated will reject it — which counts as a failure and feeds the breaker. Update the receiver first where you can.

```sh
rail0 webhooks list --topic payments.captured --active true
rail0 webhooks get <webhook_id>
rail0 webhooks create --name "order lifecycle" --callback-url https://example.com/hook \
  -t payments.authorized -t payments.captured -t payments.voided -t payments.refunded
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
| `list` | List webhooks (`--topic` shows subscriptions that *include* that event, `--active`, `--circuit-state`, `--sort`) |
| `get` | Fetch a single webhook |
| `create` | Register a webhook (`--name`, `--callback-url`, `-t/--topic` repeated or comma-separated; returns shared secret) |
| `update` | Patch `--name`, `--callback-url`, or `--topic` — the topic flag **replaces** the set, which is also how one is removed |
| `enable` | Clear the **manual** disable only — see [the two switches](#two-ways-to-be-off--and-only-one-enable) |
| `disable` | Stop deliveries reversibly, keeping the id and secret (`delete` is the permanent form) |
| `rotate-secret` | Issue a new shared secret (returned once) |
| `reset-circuit` | Close the breaker **and** clear the manual disable — the only way back from a trip |
| `event-callbacks` | Delivery attempts — the log to read when a callback did not arrive (`--status delivered\|failed`, `--response-code`, `--topic`, `--payment-id` as the payment's id or 0x rail0 id, `--since`, `--until`, `--sort`, `--page` / `--per-page`) |
| `delete` | Destroy the webhook and its secret, permanently (unlike `wallets delete`, this one is real) |

---

### Health

#### `health`

Report gateway liveness/readiness, including database connectivity and active chain/contract counts.

```sh
rail0 health
```

---

### Disputes

#### `disputes`

List **every** dispute on the payments your wallet is party to (payer or payee), each
with its parent payment embedded. Requires login.

Open **and** closed: a closed dispute stops setting the `disputed` flag on
`payments list` — which reports current state — but stays listed here, so the history
remains visible. Filter with `--status open|closed`, sort with `--sort`, page with
`--page` / `--per-page`.

```sh
rail0 disputes
rail0 disputes --status open
rail0 disputes --sort -opened_at
```

Distinct from `payments disputes <rail0Id>`, which is the open/close history of **one**
payment.

---

### Analytics

#### `analytics summary` / `timeseries` / `breakdown`

The account's sales analytics. Requires an **account**: a wallet-only session gets
`403 account_required`, because these are the merchant's figures rather than an
address's.

All three take the same filters — `--mode`, `--status`, `--token`, `--chain-id`,
`--from`, `--to`.

```sh
rail0 analytics summary                          # orders, disputes, refund/dispute rates, volume
rail0 analytics summary --from 2026-01-01T00:00:00Z
rail0 analytics timeseries --interval week       # day (default) | week | month
rail0 analytics breakdown --by token             # token | chain | mode | status | operation
```

Volume figures carry a `*_human` companion (`gross_human`, `captured_human`,
`refunded_human`, `volume_human`) beside the raw base units, using the `decimals` each
row already carries — so no token lookup is involved, and a row whose token has left
the catalog simply has no companion rather than a figure at a guessed scale.

Two nulls that are deliberate, not missing data: a **timeseries** row carries a volume
only when a single token *and* chain are filtered, and a **breakdown** by `mode` or
`status` carries none at all — a group spanning several tokens has no single unit to
add up in.

**`summary` is exempt**, and it is worth knowing which rule applies where: its volume is
broken out per (token, chain) already, so every figure has one unit behind it and can be
read directly. It never returns a null for this reason.

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
  -F @buyer -T @merchant -t USDC -a 10.50 -c 5042002 \
  -p @buyer-key
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
| `expired` | Authorization window closed with nothing captured — the escrow is still on-chain and `release` recovers it |

There is no `failed` payment **status**: `failed` belongs to a **transaction**. A
payment whose transaction reverts keeps the status it already held — the gateway
records the revert on the payment as `last_error_code` / `last_error_message` and
changes nothing else — so the operation can simply be attempted again. A reverted
attempt shows up in `rail0 payments transactions <rail0Id>`, under the transaction's
own status.

---

#### `payments list`

List payments for the authenticated wallet (requires login). Filter with `--status`,
`--mode`, `--payer` / `--payee`, `--token`, `--chain-id`, `--disputed`, `--operation`,
`--rail0-id`, `--min-amount` / `--max-amount`, and `--created-from` / `--created-to`
(ISO-8601); sort with `--sort`.

`--operation` keeps payments carrying at least one transaction of that operation, so
`--operation dispute` finds every payment that was **ever** disputed, which `--disputed`
(an **open** dispute) does not.

`--min-amount` / `--max-amount` are **decimals in token units** (`1.00`), like every
other amount flag, and require `--chain-id` and `--token`: converting them to the base
units the gateway filters on needs the token's decimals, and an amount range across
tokens with different scales doesn't describe anything. They used to take base units —
the only flags in the CLI that did — so `--min-amount 1.00` quietly meant one
millionth of a USDC and matched everything.

```sh
rail0 payments list
rail0 payments list --sort -created_at
rail0 payments list --chain-id 5042002 --disputed
rail0 payments list --chain-id 5042002 --token 0x3600… --min-amount 1.00
rail0 payments list --operation refund --created-from 2026-09-01T00:00:00Z
```

---

#### `payments transactions <rail0Id>`

One row per on-chain attempt the payment has produced, newest first by default.

**This is the view `--wait` watches.** It polls this table for the attempt it created and returns when that row reads `confirmed` or `failed` — so it is also where to look when a `--wait` timed out, or when an operation was submitted without one. The row's status says whether the chain has spoken yet, and a failed row carries the decoded revert reason.

A payment can hold several rows for the same operation (a retried broadcast, or successive partial captures), which is why the balances a payment reports are the last **confirmed** ones and not the last submitted.

```sh
rail0 payments transactions 0xabc123…
rail0 payments transactions 0xabc123… --sort -confirmed_at
rail0 payments transactions 0xabc123… --operation capture --status failed
```

`--sort` takes `created_at`, `updated_at`, `block_number` or `confirmed_at`;
`--operation` and `--status` use the transaction vocabularies (`close_dispute`,
`failed` and the rest), which `--status` on `payments list` does not.

#### `payments transaction <rail0Id> <transaction_id>`

One transaction, by the id the lifecycle commands print when the gateway accepts an
operation: its status, the decoded reason of a failed one, and whether a stuck one is
`redrivable`.

#### `payments redrive <rail0Id> <transaction_id>`

Re-broadcast a transaction that was prepared and **signed** but never reached the chain
(a worker that died in between, a drained queue). The same signed bytes go out again;
no new signature is asked for and nothing about the payment changes. Only a row reading
`redrivable: true` qualifies. A `pending` row without a signature is unfinished, not
stuck, so re-run the operation instead. Payee only; `-w/--wait` blocks until it
confirms. The gateway also redrives on its own recovery cadence; this asks for it now.

```sh
rail0 payments transaction 0xabc123… 7f1c0c8e-…
rail0 payments redrive 0xabc123… 7f1c0c8e-… --wait
```

---

#### `payments history <rail0Id>`

Print a payment's chronological timeline: the `created` and `signed` milestones plus one entry per on-chain transaction (`authorize`, `capture`, `void`, `release`, `refund`, `dispute`, `close_dispute`), oldest first. Only the significant fields are shown per entry (time, event, status, amount, tx hash, block). It is assembled client-side from a single `payments get` — there is no dedicated gateway endpoint.

Requires login: the timeline is assembled from `payments get`, which the gateway serves only to the payment's payer or payee. The `rail0_id` (or UUID) argument is resolved by the gateway in-path.

```sh
rail0 payments history 0xabc123…
```

---

> **Interactive prompts (operations).** On a terminal, the action commands below
> (`authorize`/`charge`/`capture`/`void`/`release`/`refund`/`dispute`/`dispute close`)
> prompt for anything you didn't pass: omit `<rail0Id>` to pick from a list of your
> payments the operation can act on; omit `-a`/`--amount` (capture/refund) or
> `--reason` (dispute) to be asked for it. The signing key (`-p`) is prompted
> **hidden** if you pass neither `-p` nor `RAIL0_PRIVATE_KEY`, so it never reaches
> the shell history or the process arguments. In a non-interactive shell a missing
> required value is a clear error instead.

#### `payments authorize <rail0Id>`

Prepare the `authorize()` transaction pulling funds into escrow. Payer signature must be submitted first.

```sh
rail0 payments authorize <rail0Id> -p @merchant-key
```

---

#### `payments charge <rail0Id>`

Prepare the `charge()` transaction — authorize + capture in one call (no escrow). Payment must have been created with `-C` / `--charge`.

```sh
rail0 payments charge <rail0Id> -p @merchant-key
```

---

#### `payments capture <rail0Id> -a <n>`

Prepare the `capture()` transaction. Partial captures supported: pass `-a` less than `capturableAmount`.

```sh
# Full capture
rail0 payments capture <rail0Id> -a 1.00 -p @merchant-key

# Partial capture
rail0 payments capture <rail0Id> -a 0.50 -p @merchant-key
```

---

#### `payments void <rail0Id>`

Prepare the `void()` transaction — cancels authorization and returns escrowed funds to payer. Only callable before `authorizationExpiry`, by the payee.

```sh
rail0 payments void <rail0Id> -p @merchant-key
```

---

#### `payments release <rail0Id>`

Prepare the `release()` transaction — returns remaining escrowed funds to payer after `authorizationExpiry`. Permissionless: both payer and payee can call it. The caller is whoever the signing key belongs to (derived from the key), so pass the payer's key to release as the payer.

```sh
# Payee releases
rail0 payments release <rail0Id> -p @merchant-key

# Payer releases (just use the payer's key)
rail0 payments release <rail0Id> -p @buyer-key
```

---

#### `payments refund <rail0Id> -a <n>`

Prepare the `refund()` transaction. Partial refunds supported.

```sh
rail0 payments refund <rail0Id> -a 1.00 -p @merchant-key
```

---

## Full flow examples

Worked versions of the three shapes, using the address book and keychain names
from [Your first payment](#your-first-payment) — `@buyer`/`@merchant` are
addresses, `@buyer-key`/`@merchant-key` are keys. Substitute raw `0x…` values
anywhere if you have not saved them.

All three use `--wait`, so each command returns only once the chain has confirmed
it and exits non-zero if it did not. That is what makes these safe to paste into a
script: without it a command succeeds the moment the transaction is *broadcast*,
and the next line runs against a payment that has not moved yet.

### Authorize-and-capture

The escrow flow: funds are held, then released to the merchant — possibly in
parts, possibly never.

```sh
export RAIL0_BASE_URL=https://api.rail0.xyz

# The MERCHANT logs in: the action commands need a session.
# The buyer does not — 'payments create' signs in with the payer key on its own.
rail0 auth login -p @merchant-key

# 1. Buyer creates the payment and signs the EIP-3009 authorization locally
RAIL0_ID=$(rail0 payments create \
  -F @buyer -T @merchant -t USDC -a 1.00 -c 5042002 \
  -p @buyer-key -q)

# 2. Merchant authorizes — funds move into escrow
rail0 payments authorize "$RAIL0_ID" -p @merchant-key --wait

# 3. Merchant captures — funds move to the merchant
rail0 payments capture "$RAIL0_ID" -a 1.00 -p @merchant-key --wait

rail0 payments get "$RAIL0_ID" --pretty
```

To capture in parts, call `capture` more than once with smaller amounts; the rest
stays available until `authorizationExpiry`. `--wait` matters more here than
anywhere else — `capturableAmount` is the last *confirmed* balance, so without it
you would pick the second amount from a stale number.

---

### Charge (one-shot)

No escrow: funds go straight from buyer to merchant. Note the `-C` at create —
the mode is fixed when the payment is created, not when it is submitted.

```sh
RAIL0_ID=$(rail0 payments create \
  -F @buyer -T @merchant -t USDC -a 1.00 -c 5042002 -C \
  -p @buyer-key -q)

rail0 payments charge "$RAIL0_ID" -p @merchant-key --wait
```

---

### Refund

Sends captured funds back to the buyer. The **merchant** signs, because the money
is now in the merchant's wallet — this is the mirror of the buyer's signature at
create, and needs no allowance either.

```sh
rail0 payments refund "$RAIL0_ID" -a 1.00 -p @merchant-key --wait
```

Before anything is captured there is nothing to refund — use `void` to drop the
hold instead, or `release` once the authorization window has closed.

---

## Output modes

Every command prints the gateway response with all fields the gateway returns.

**Terminal (default):** syntax-highlighted, indented, JSON-*like* output — keys, strings, hex values (addresses/hashes), numbers, booleans, and payment status are colored for readability. Object keys are printed **unquoted** (colour already sets them apart), so this view is intentionally not strict JSON.

**Plain JSON (`--json` or piped):** strict, valid indented JSON with no ANSI colour on stdout, suitable for `jq` and shell pipelines.

**Pretty (`--pretty`):** a key-value (single record) or table (list) view with the same highlighting, as an alternative terminal layout. Terminal only — piping still emits plain JSON.

```sh
# Extract the rail0_id of a freshly created payment
RAIL0_ID=$(rail0 payments create -F @buyer -T @merchant -t USDC -a 10.50 -c 5042002 -p @buyer-key --json | jq -r .rail0_id)

# Check payment status in a script
STATUS=$(rail0 payments get <rail0Id> --json | jq -r .status)
```
