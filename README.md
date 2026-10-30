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
the first attempt never got as far as broadcasting, the re-run finishes it; if it
**failed** on-chain, the re-run exits non-zero with the decoded reason, and retrying
needs a **new** key — the old one can only bring the failed attempt back. The same key with
a different amount is refused `422 idempotency_key_reused`, so a key you forgot to
rotate is reported rather than silently honoured. Keys are scoped to the payment.

## Contents

- [Requirements](#requirements)
- [Installation](#installation) — [install script](#install-script-macos-and-linux--recommended) · [manual download](#manual-download-any-platform-windows-included) · [from source](#build-from-source-contributors) · [`rail0 update`](#updating-rail0-update) · [`rail0 init`](#guided-setup-rail0-init)
- [**Your first payment**](#your-first-payment) — the guided path
- [Shell completion](#shell-completion)
- [Configuration](#configuration) — [defaults](#persisted-defaults-config) · [environments](#environments-env) · [`--chain`](#chains---chain-slugid) · [address book](#address-book-aliases) · [signing keys](#signing-keys-os-keychain) · [new wallet](#new-wallet-keys-generate) · [`keys balance`](#keys-balance-addressnamename--on-chain-balance-like-cast-balance)
- [Signing modes](#signing-modes) — who signs what, and when
- [Commands](#commands) — the full reference, including [repairing a webhook](#repairing-a-webhook-that-stopped-delivering) and [payouts](#payouts)
- [Full flow examples](#full-flow-examples) — authorize/capture, charge, refund
- [Output modes](#output-modes) — including [agent mode (`--agent`)](#agent-mode---agent)
- [Releasing](#releasing) — for maintainers

---

## Requirements

- Go ≥ 1.26 (matches the `go` directive in `go.mod`)
- Nothing else to clone: the [rail0-go](https://github.com/commercelayer/rail0-go) SDK comes from the Go module proxy at the version `go.mod` requires
- Access to a RAIL0 gateway instance (for the commands that call it — building/`rail0 --help` need no gateway)

---

## Installation

Three ways to install a release, all the same static binary with nothing else to set
up. Every release (see [Releasing](#releasing)) publishes them for macOS and Linux
(amd64 and arm64) and Windows (amd64).

### Install script (macOS and Linux) — recommended

```sh
curl -fsSL https://raw.githubusercontent.com/commercelayer/rail0-cli/main/install.sh | sh
```

[`install.sh`](install.sh) does the following:

1. Picks the archive for your OS and CPU.
2. Verifies it against the release's `checksums.txt`. It refuses to install anything
   that does not match.
3. Installs `rail0` and `r0` into `~/.local/bin`, a directory you own, so neither the
   install nor a later `rail0 update` needs `sudo`.
4. If that directory is not on your `PATH`, tells you the line to add.

It runs in any POSIX `sh`, including minimal containers. Optional variables:

- `RAIL0_VERSION=1.2.0` pins a release.
- `RAIL0_INSTALL_DIR=/somewhere/bin` changes the destination.
- `RAIL0_RELEASES_URL` points at a mirror.

### Manual download (any platform, Windows included)

Grab the archive for your platform from
[Releases](https://github.com/commercelayer/rail0-cli/releases). On Windows it is a
`.zip`. Verify it against `checksums.txt` from the same release, then put `rail0` on
your `PATH`.

Each archive also carries the **shell completions** (`completions/`), the **man pages**
(`man/man1/`) and the command reference (`docs/`), so an installed binary is
self-documenting:

```sh
sudo cp man/man1/*.1 /usr/local/share/man/man1/   # then: man rail0
cp completions/_rail0 "${fpath[1]}/"              # zsh; see `rail0 completion --help`
```

Whichever way you install, continue with [`rail0 init`](#guided-setup-rail0-init).

### Build from source (contributors)

Clone this repo and build. The `rail0-go` SDK is an ordinary module dependency, fetched
at the version `go.mod` requires.

```sh
git clone https://github.com/commercelayer/rail0-cli
cd rail0-cli
make install            # installs `rail0` + the short `r0` alias into /usr/local/bin
```

`make install` puts the binary on your `PATH` as **`rail0`** and a short alias
**`r0`** (a symlink), so the two are interchangeable — `r0 wallets` is the same
as `rail0 wallets`. Override the location with `make install PREFIX=$HOME/go/bin`.

To build against a local `rail0-go` checkout (an SDK change not released yet), use a Go
workspace instead of editing `go.mod`; `go.work` is git-ignored:

```sh
go work init . ../rail0-go
```

`bin/build` does the same as `make install` without `make` (build + install `rail0` + `r0`):

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

### Updating (`rail0 update`)

A release binary updates itself:

```sh
rail0 update           # download the latest release and install it in place
rail0 update --check   # only report whether a newer release exists
```

It downloads this platform's archive from the latest GitHub release, with a progress
bar on a terminal. It verifies the archive's SHA-256 against the release's
`checksums.txt` and then swaps the binary. The swap is a rename, so an interrupted
update leaves the old binary intact, and nothing changes unless the download verifies.
The `r0` alias follows, since it is a link to the same file. If rail0 sits in a
directory you cannot write to (`/usr/local/bin` after `make install`), run
`sudo rail0 update`.

Interactive sessions also **check once a day** and, when a newer release exists, ask
`Update now? [Y/n]` before running your command. The command then runs normally
whatever you answer. The check never runs in CI (`CI` set), with `--json` or
`--quiet` (or a stored `output` default of `json` or `quiet`, from `rail0 config set
output …` or `RAIL0_OUTPUT`, unless another output flag is passed), when stdin or
stderr is not a terminal, or with `RAIL0_NO_UPDATE_CHECK=1`.
It is skipped for local builds (`dev` or a `git describe` version), and a failed lookup
is silent. The last check time lives in the user cache directory, next to the session
tokens (`update-check.json`).

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

No wallet yet? [`rail0 keys generate`](#new-wallet-keys-generate) creates one
locally and prints its key once.

### 4. Sign in as the merchant

```sh
rail0 auth login -p @merchant-key
rail0 auth status                  # address, account, expiry
```

**The buyer does not need this step.** `payments create` signs in on its own with
the payer key, so a buyer with a wallet and nothing else can pay. The signing
commands (`authorize`, `capture`, …) do the same with their own `-p` key, so the
merchant does not log in for those either: the login stores the merchant's session
for the commands that take no key — the reads (`payments get`, `list`, `wait`) and
the account views (`account`, `wallets`, `webhooks`, `analytics`).

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
  --token USDC --amount 1.00 --chain arc \
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

Beyond commands and flags: `keys rm` / `keys address` / `keys balance` complete the
saved key names, the address flags (`--from`, `--to`, `--address`, `--payer`,
`--payee`) complete the address book as `@name`, `--chain` completes the current
gateway's chain slugs, `rail0 env <TAB>` completes the environment names, and the
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
| `--agent`    | `RAIL0_AGENT=1`      | Agent mode: one JSON envelope on stdout, JSON events on stderr, never a prompt — see [agent mode](#agent-mode---agent) |
| — | `RAIL0_HTTP_TIMEOUT` | Per-request HTTP timeout in seconds (default 30). Distinct from the wait commands' `--timeout`, which is the polling deadline |

`--json`, `--pretty` and `--quiet` select one output mode each. Combined, they do not
error: one wins silently in the order **`--quiet`  >  `--pretty`  >  `--json`** — so
`rail0 aliases --json --pretty` shows the pretty view, and `-q` beats either.
`--agent` is the exception: it overrides any stored `output` default, and combined with
`--quiet` or `--pretty` it is refused (exit `2`).

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

The scale is the payment's own `decimals` field, which the gateway sets on every
payment — so these views cost no `GET /tokens`. Only a payment whose `decimals` is null
(a token the gateway cannot resolve) falls back to the token catalogue.

Every payment also carries `in_flight`, the gateway's own answer to "is a transaction
on its way to the chain" (submitting, submitted, or signed and pending broadcast) — the
field to read before deciding whether a balance is settled enough to act on.

The raw fields are untouched, so a script reading `amount` keeps working — the
companions are additive. They are **omitted** when the token's decimals are unknown
(null on the payment and missing from `rail0 tokens`), because a base-unit integer in a field named
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

Exit codes distinguish the failure class:

| Code | Meaning |
|---|---|
| `0` | success |
| `1` | any other failure |
| `2` | usage: unknown command or flag, wrong arguments, a required value or `--yes` missing where nobody can be asked, a command not offered in agent mode |
| `3` | unauthorized (401) |
| `4` | forbidden (403) |
| `5` | not found (404) |
| `6` | conflict (409) |
| `7` | rejected by the gateway (400, 422) |
| `8` | gateway error (5xx) |
| `9` | network: no response |
| `10` | rate-limited (429) |
| `11` | in flight: a `--wait` ran out of time — even when its last poll hit a network error. The operation may still settle: check it, never repeat it |
| `130` | cancelled by the user |

`2` and `11` are new in this release: before, both exited `1`, and a 400 exited `1`
instead of `7`.

`10` is the one class where re-running the identical command later is the correct
response, which is why it is not folded into the generic failure.

`130` is what you get by pressing Ctrl-C (or ESC) at any prompt, and by answering `N`
to a destructive confirmation — the POSIX convention for SIGINT, so `&&` chains stop
there. Nothing was executed; it is a cancellation, not a failure.

Errors are printed as the gateway's own sentence plus, where the CLI can add
something, the command to run next: a `422` on `capture` explains that the amount
exceeds the capturable balance and points at `rail0 payments get <id>`, and a wallet
with no gas is named as such rather than as an opaque RPC failure.

A `404` never says which of its two causes it is — the gateway answers the same for
an id that does not exist and one this session may not see. For a `payments` command
the CLI adds that you may not be the payment's payer or payee (check `-p`); for
webhooks, wallets, wallet tokens and accounts, which belong to the account rather
than to a party, it says only that the id may be wrong or not visible to this session.
A `409` on [`account update`](#account-update) adds that the name or email is already
used by another account — the gateway's conflict body cannot say which field collided.

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
rail0 auth login -p @payee     # caches a JWT via SIWE; the commands that take no key read it from there
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
every command. Run bare to list the settings that are **set**, with each value and
where it comes from (flag, env var or config file); a key set nowhere is not listed,
because it has no value. (Secrets like private keys are never stored here.)

```sh
rail0 config                                  # list the settings that are set + origin (flag/env/file)
rail0 config set base-url http://localhost:9292
rail0 config set chain arc                     # fills --chain/-c when omitted (slug or id)
rail0 config set token USDC                    # fills --token/-t when omitted
rail0 config get chain                         # value + origin; an unset key is an error
rail0 config unset chain
rail0 config path                              # config file location
```

| Key | Fills | Env var | When not set |
| --- | --- | --- | --- |
| `base-url` | `--base-url` | `RAIL0_BASE_URL` | `https://api.rail0.xyz` |
| `chain` | `--chain` / `-c` | `RAIL0_CHAIN` | the matching flag must be passed |
| `chain-id` | legacy numeric form of `chain` | `RAIL0_CHAIN_ID` | — |
| `token` | `--token` / `-t` | `RAIL0_TOKEN_SYMBOL` | the matching flag must be passed |
| `admin-url` | — (the page `rail0 open` opens) | `RAIL0_ADMIN_URL` | the current environment's (development, staging) |
| `output` | `--json` / `--pretty` / `--quiet` | `RAIL0_OUTPUT` | `auto` |
| `http-timeout` | — (per-request timeout, seconds) | `RAIL0_HTTP_TIMEOUT` | `30` |

A stored value is the lowest-priority source, so the matching flag or env var still
overrides it for a single command. The last column is what the CLI does when the key
is set nowhere — the CLI's behaviour, not a value of the key, which is why the listing
does not show it. When `payments create` fills the chain or the token from a stored
setting, it says so on stderr ("Using Arc Testnet (5042002) (config file) — pass -c/-t
to override…"), so a refusal never names a chain you did not type without telling you
where it came from.

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

`chain` takes a slug (`arc`) or a chain id, resolved against the current gateway
when a command uses it. Configs written before slugs hold the numeric `chain-id`; it is
still read (`chain` wins when both are set in the same place, and an env var outranks
the config file across the two, so an exported `RAIL0_CHAIN_ID` still overrides a
stored `chain`). The next `rail0 env development|staging` rewrites it as `chain: arc`.

Note that `token`'s env var is `RAIL0_TOKEN_SYMBOL`, not `RAIL0_TOKEN` — that name was
a session-token source until it was removed, and reusing it would make a stale
exported JWT arrive as a payment token symbol.

### Environments (`env`)

Switch a whole bundle of settings at once instead of setting each key by hand:

| Environment | base-url | admin-url | chain | token |
| --- | --- | --- | --- | --- |
| `development` | `http://localhost:9292` | `http://localhost:3000` | `arc` | `USDC` |
| `staging` | `https://rail0-gateway-staging.fly.dev` | `https://rail0-admin-staging.fly.dev` | `arc` | `USDC` |
| `production` | `https://api.rail0.xyz` | — | — | — |

Production carries no admin URL, chain or token, and that is not an omission: the two other
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
  chain = arc
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

### Chains (`--chain <slug|id>`)

Every command that takes a chain takes `--chain` (`-c` where it had one): a **slug**
(`arc`, `base`, `optimism`, `arbitrum`, `polygon`, `avalanche`, `sonic`, `ethereum`, …)
or a numeric **chain id**. A slug is shared by a chain's testnet and mainnet, and a
gateway serves one network class — staging and development testnets, production
mainnets — so the slug is resolved against the current environment's gateway
(`GET /blockchains`, once per command) and picks the right network on its own:
`--chain arc` is Arc Testnet on staging and Arc on production.

```sh
rail0 tokens --chain arc
rail0 payments create … -c base
rail0 wallets balances @merchant --chain 84532     # an id still works, no lookup
```

- An unknown slug lists the available ones: `no chain with slug "solana" (available: arc (5042002), base (84532))`.
- A numeric id is used as is, without asking the gateway — exactly as before.
- An unreachable gateway is reported as such; pass the id to skip the lookup.
- `--chain-id <id>` is still accepted as a hidden alias. Passing both with different
  chains is an error.
- `rail0 chains` lists each chain's `slug`; shell completion offers the slugs on `--chain`.

Commands with `--chain`: `payments create`, `payments list`,
`analytics summary|timeseries|breakdown`, `tokens`, `wallets`, `wallets balances`,
`wallets tokens add|enable|disable`, `payouts destinations add`, `keys balance`.

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
rail0 keys balance payer --chain arc   # on-chain balance of that address
rail0 keys rm payer             # remove

rail0 payments create -F @buyer -T @payee -t USDC -a 1.00 -c arc -p @payer
```

#### `keys balance <address|@name|name>` — on-chain balance (like `cast balance`)

Reads a balance straight from the chain's RPC nodes: no session, nothing signed. The
argument is a `0x` address or a saved key name (`@` optional — the address is derived,
the key is never printed); a name that is not a saved key is tried in the address book.

```sh
rail0 keys balance 0xABCD… --chain arc               # native balance
rail0 keys balance @payer --chain arc --token USDC   # one ERC-20 (symbol or 0x address)
rail0 keys balance payer --tokens                    # native + every active rail0 token (default chain)
rail0 keys balance 0xABCD… --rpc-url https://rpc.example.org -q
```

| Flag | Short | Description |
| --- | --- | --- |
| `--chain` | `-c` | Chain slug or id; defaults to the configured `chain` |
| `--rpc-url` | | Read from this node instead (its `eth_chainId` names the chain; `--chain`, if passed, must agree) |
| `--token` | `-t` | One ERC-20: a symbol from `rail0 tokens` (USDC) or a `0x` address (symbol/decimals read on-chain) |
| `--tokens` | | Native balance plus every active rail0 token of the chain |

The RPC endpoints are the chain's keyless `public_rpc_urls` from the gateway, tried in
order with a per-endpoint timeout until one answers. Output: `balance` (wei / base
units) and `balance_human` per asset; `-q` prints the raw value (one
`<asset> <symbol> <value>` line each when there are several), `--json` / `--pretty` /
`--agent` as everywhere. **On Arc the native gas token is USDC with 18 decimals**, while
the USDC ERC-20 (`--token USDC`) uses 6 — the same money on two scales.

```json
{
  "address": "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
  "chain_id": 5042002,
  "chain": "arc",
  "chain_name": "Arc Testnet",
  "balances": [
    { "asset": "native", "symbol": "USDC", "decimals": 18, "balance": "1500000000000000000", "balance_human": "1.5 USDC" },
    { "asset": "0x3600000000000000000000000000000000000000", "symbol": "USDC", "decimals": 6, "balance": "2500000", "balance_human": "2.5 USDC" }
  ]
}
```

Backends: macOS Keychain, Windows Credential Manager, Linux Secret Service
(gnome-keyring/KWallet). On a system without one (e.g. **headless Linux**) the
keychain `keys` commands return an error — there, pass `--private-key` or set
`RAIL0_PRIVATE_KEY`.

### New wallet (`keys generate`)

No wallet yet? `rail0 keys generate` creates a new key pair **on your machine** and
prints its EIP-55 address and its private key (0x-prefixed hex). It is fully
offline — no gateway call, no update check — and writes nothing: not the config,
not the keychain, not a file.

```sh
rail0 keys generate           # address + private key on stdout, warning on stderr
rail0 keys generate --json    # {"address": "0x…", "private_key": "0x…"} — stdout stays clean JSON
```

> **The private key is shown once and cannot be recovered** — not by RAIL0, not by
> anyone. Store it right away in a password manager. Whoever has it controls the
> funds: never paste it into a website or a chat, never share it with RAIL0
> operators (they will never ask), and mind shell history, terminal scrollback,
> screen sharing and wherever you redirect stdout.

Next steps:

1. `rail0 keys add <name>` saves it in the OS keychain (prompted hidden), so you sign
   with `-p @<name>` instead of pasting it.
2. Register the **address** — never the key — with RAIL0: sign up as a merchant
   (`/signup` in the admin) or ask an operator for an invite, then claim the account
   from the emailed link by signing with this wallet (import the key into a wallet
   you control to sign there). After that, `rail0 auth login -p @<name>`.

Adding the address to an account you already have (`wallets create`) is currently
unavailable (`403 wallet_add_unavailable`).

---

## Signing modes

### Payer — `payments create` / `payments sign`

The payer creates the payment and signs the EIP-712 payload with their key
(`--private-key` / `-p`), submitting the signature in one step:

```sh
rail0 payments create -F @buyer -T @merchant -t USDC -a 10.50 -c arc -p @buyer-key
```

> **Auto sign-in.** The gateway requires a SIWE session for every `/payments`
> write and binds the payment to the payer (the `payer` must equal the signed-in
> address). `payments create` handles this: it signs in with the payer's key (the
> same `-p` key it signs the payload with) when there's no valid session for that
> address — so no separate `rail0 auth login` is needed, and the session's address
> matches `--from`. A stored session for the same address that would expire before the
> command finishes (a minute, plus the `--timeout` budget with `--wait`) is renewed and
> saved. If a session for a *different* address is already stored, that one is left
> alone: the sign-in serves this command only, and the CLI says so.

> **Replay safety.** Each `payments create` sends a fresh `Idempotency-Key`, so a
> command retried after a lost response (a timeout, a 429) cannot mint a second
> payment — the gateway returns the first one. Pass `--idempotency-key` to reuse a
> key of your own across processes. Re-running with a key whose create already went
> through prints a note and returns that payment (exit 0) — it is already signed, so
> nothing is signed again; one replayed still `unsigned` is finished by signing it.

> **Checked before signing.** Before the payer's key signs anything, the CLI checks
> the payment the gateway created against the request you built: `payer`, `payee`,
> `token` (addresses compared case-insensitively), `mode`, `chain_id`, and `amount` in
> base units (your `--amount` scaled locally). The EIP-3009 authorization must be from
> the signing key, on that chain and token, addressed to the payment's RAIL0 contract,
> for exactly that amount. Any mismatch refuses to sign, and so does anything the check
> cannot compare: a created payment without a `rail0_contract` is refused, never signed
> unchecked. `payments sign` and `payments refund` run the same check against the
> fetched payment (its stored amount, or the refund `--amount` scaled by its decimals).
> Not yet checked: the
> authorization's `nonce`, which binds it to the payment terms on-chain — recomputing it
> needs the contract version, which the SDK cannot read from `GET /blockchains` yet.

**If the signature step fails** (a dropped connection, a rate limit, the wrong
key), the payment exists at `unsigned` with no signature. Don't create it again —
deposit the signature on the existing one:

```sh
rail0 payments sign 0xRail0Id… -p @buyer-key
```

If `payments sign` says the payment is unsigned but the gateway could not build its
signing payload, the gateway's chain read failed (usually a transient RPC issue):
retry the same command in a moment.

**Token/chain discovery from the payee.** On a terminal, if you give `--to` but
omit `--token` and/or `--chain`, the CLI looks up what that merchant accepts
(public `GET /payment_methods` for the payee address) and prompts you to pick
from those combinations — a single accepted method is used automatically. So a
buyer who knows only the merchant's address can run:

```sh
rail0 payments create -F @buyer -T @merchant -a 10.50 -p @payer
# → prompts "Select the token and chain the merchant accepts" (skipped if only one)
```

If you don't pass `--to` (or that merchant exposes no matching method), the
interactive form still helps: `--chain` and `--token` are offered as
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
| `--timeout` | — | Max time to wait with `--wait`. Unset, it follows the payment's chain: **10× the chain's measured p90 settlement** (`settlement.p90_seconds` on `rail0 chains`), kept between `5m` and `15m` — never longer than the old fixed default, never short enough to report one slow-but-healthy operation as a failure. It stays `15m` when the chain has too few confirmed transactions to be measured, and on `payments redrive`, which holds no payment to read the chain from. Settlement is end to end — the chain's `safe`/`finalized` lag plus the gateway's pipeline — so it is minutes on some chains (~11 min on Arbitrum Sepolia) and seconds on others. A `--wait` that gives up exits non-zero, which a script reads as a failed capture, so raise it rather than lower it when in doubt; an explicit value always wins |
| `--idempotency-key` | — | Make a re-run safe: the same key returns the first transaction instead of opening a second — see [Safe re-runs](#safe-re-runs---idempotency-key) |
| `--yes` | `-y` | Skip the confirmation prompt (every action except `authorize`); required to run non-interactively |

**Waiting for confirmation.** The gateway returns `202` immediately and the indexer
confirms the on-chain event a moment later. Pass `--wait` to have the command poll
until the operation's transaction is `confirmed` (then print the settled payment)
or `failed`, instead of polling `payments get` by hand. You can also wait on an
existing payment: `rail0 payments wait <rail0Id> [--until <status>]` (default: until
no transaction is still in flight). Polling backs off: a read every 2s for the first
30s, then each pause half again as long, up to one every 10s — and a `429` is waited
out for its `Retry-After` rather than polled through.

**Confirmations.** Every irreversible action prompts for a `y/N` confirmation first;
pass `--yes` to skip it (and to run it in a non-interactive shell, where it otherwise
refuses rather than blocking on a hidden prompt). That covers every fund-moving
on-chain op — `capture`, `charge`, `void`, `release`, `refund`, `dispute`,
`dispute close` — and, identically, `auth revoke-all`, `keys rm`, `wallets delete`
(and `wallets update --active false`, the same operation), `wallets tokens disable`,
`webhooks delete`, `webhooks rotate-secret`, and an `env` switch that would clear
settings. A sweep test asserts the whole
surface, so a new fund-moving command cannot ship ungated.

`webhooks rotate-secret` is gated for a reason that is easy to miss: it is not
undoable by rotating back. The previous secret is gone, and from that moment every
consumer still verifying `X-Rail0-Signature` against it rejects deliveries — so a
mistyped webhook id silently breaks production verification.

The signing key is resolved in this order: `--private-key` (`-p` / `--pk`, where `@name` pulls the key from the OS keychain) > `RAIL0_PRIVATE_KEY` env var. If none is set, the CLI prompts for the key **hidden** on a terminal (or reads one line from a piped stdin), so it never lands in the shell history or process arguments — the same masked input as `keys add`.

**Checked before signing.** The gateway prepares the transaction, but the CLI checks it
locally before your key touches it, and refuses to sign on any mismatch: it must be
value-0, on the payment's chain, from the signing key, addressed to the payment's RAIL0
contract (required: a transaction without `to`, or a payment without a
`rail0_contract`, is refused), call **the RAIL0 function you asked for** (its 4-byte selector — so a `void`
answered with `capture` calldata, or a `dispute` with `closeDispute`, is refused) on
**this payment's id**, and — for `capture` and `refund` — move **exactly the `--amount`
you typed**, scaled to base units locally. Like the refund authorization's check, the
amount check fails closed: if the token's decimals cannot be resolved, the command
refuses rather than signing unchecked.

#### Addressing a payment: UUID or rail0_id

Commands that take a payment argument (`get`, `transactions`, and the action commands) accept either form, passed straight through to the gateway, which resolves both:

- the payment **UUID** `id`;
- the logical **`rail0_id`** (`0x…`, 64 hex).

Either form needs the same authorization: the gateway serves a payment only to its payer or payee. The action commands sign in as their `-p` key; the reads (`get`, `transactions`, `wait`, …) use the stored session, which must be one of the two parties'. The CLI no longer pre-resolves the `rail0_id` client-side; a value that is neither a UUID nor a `rail0_id` is a clean 404.

---

## Commands

### Authentication

Every `/payments` operation requires a SIWE session (the gateway rejects anonymous writes and binds a created payment to the payer). **Every signing command signs in automatically as its own key** — `payments create`/`sign` with the payer key, the action commands with their `-p` key — so none of them needs a prior login (see [Payer — `payments create`](#payer--payments-create)). The commands that take no key (the payment reads, `payments redrive`, and the account views) use the session stored by `auth login`; for a payment read it must be the payer's or the payee's.

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

### Account

`account` shows and edits the **profile** of the signed-in account — its id, name, email, `priority_tip` setting and timestamps — as opposed to [`wallets`](#wallets), which manages the addresses under it. It needs a login (`auth login`) by a wallet that belongs to an account; the account is the session's own, the id `auth status` prints. `--account <id>` (on the bare command and both verbs) names another account, which the gateway answers only for an **admin** — anyone else gets `403 not_your_account`. The account's `active` flag is operator-only and is not exposed.

#### `account` / `account get`

Show the account. Run bare or as `account get` — the same read.

```sh
rail0 account                          # the signed-in account
rail0 account get --pretty
rail0 account get --account 019e748b-… # another account (admin only)
```

| Flag | Short | Description |
|------|-------|-------------|
| `--account` | — | Account id; defaults to the logged-in session account (another account's needs an admin session) |

#### `account update`

Change the display name, contact email, priority-tip setting and/or payment windows. At least one of `--name` / `--email` / `--priority-tip` / `--auth-ttl` / `--refund-ttl` is required, and a name or email may not be blank — both are refused locally, before any request.

```sh
rail0 account update --name "Acme Store"
rail0 account update --name "Acme Store" --email ops@acme.example
rail0 account update --priority-tip=false   # minimum tip on the account's own transactions
rail0 account update --priority-tip         # back to the network's suggested priority fee
rail0 account update --auth-ttl 2d --refund-ttl 60d
rail0 account update --refund-ttl default # back to the gateway default
```

| Flag | Short | Description |
|------|-------|-------------|
| `--name` | — | New display name (unique across accounts) |
| `--email` | — | New contact email (unique across accounts) |
| `--priority-tip` | — | Tip the network's suggested priority fee on the account's own transactions (the default); `--priority-tip=false` uses the chain's minimum tip |
| `--auth-ttl` | — | Capture window of new payments: days or hours (`2d`, `36h`), or `default` |
| `--refund-ttl` | — | Refund and dispute window of new payments: days or hours (`60d`), or `default` |
| `--account` | — | Account id; defaults to the logged-in session account (another account's needs an admin session) |

Name and email are **unique across accounts**: one another account already holds is refused with `409`, and the CLI says it is the name or email. A deactivated account, or a session signed in with a deactivated wallet, cannot change them (`403 account_deactivated` / `wallet_deactivated`) — a revoked key must not be able to redirect the contact email. An admin may update another account with `--account`; the gateway audits an operator's change with the values it replaced. No confirmation is asked: the change is not destructive, and another `account update` reverses it.

`--priority-tip` is the fee setting of the account's **own** transactions — authorize, capture, void, release, refund. On, the default, they tip the network's suggested priority fee; off, they pay the chain's minimum tip (0 on most chains, 25 gwei on Polygon PoS/Amoy): cheaper, and slower to confirm when the chain is busy. The buyer's dispute and close-dispute never read it. The flag is sent only when given, so `account update --name X` leaves the setting alone.

`--auth-ttl` and `--refund-ttl` set the deadlines the account's **new** payments are created with: how long an authorization can be captured, and how long a payment can be refunded or disputed. Each takes days or hours (`2d`, `36h`), or `default` to go back to the gateway default; a malformed or zero value is refused locally. The bounds are on the profile (`payment_windows` in `rail0 account`): outside them, or with the refund window less than a day past the authorization, the gateway answers `422 invalid_payment_windows`. Existing payments keep their signed deadlines.

---

### Wallets

`wallets` manages **your own** account's wallets and requires a login (`auth login`) — the gateway now protects the whole account section with SIWE. When **listing**, `--account` defaults to the logged-in session's account; if given it must be your own account (another account's id is rejected with 403). Run bare to list; the verbs act on one wallet, named by its id or **0x address** (the wallet's own identity, unique per account). Address-book `@aliases` are for external addresses in `payments`, not for your own wallets. To discover **another** merchant's accepted wallets/tokens without logging in, use [`payment-methods`](#payment-methods).

A session is bound to the wallet it signed in with: reads cover every wallet of the account, but `update`, `delete` and `tokens` act only on that wallet (`403 not_your_wallet` otherwise). Adding a wallet (`wallets create`) is currently unavailable (`403 wallet_add_unavailable`).

#### `wallets` — list

Lists the account's wallets, each with its token holdings nested under `tokens`. A wallet with no tokens still appears (with an empty `tokens`), so it is never hidden just because nothing is associated.

```sh
rail0 wallets                                   # logged-in session account
rail0 wallets --account 019e748b-…              # your own account, stated explicitly
rail0 wallets --chain arc --symbol USDC --active true
rail0 wallets --default true --sort -created_at
```

| Flag | Short | Description |
|------|-------|-------------|
| `--account` | — | Your account id to list; defaults to the logged-in session account (must be your own — requires login) |
| `--chain` | — | Restrict the nested tokens to this chain id (wallets still listed) |
| `--symbol` | `-s` | Restrict the nested tokens to this symbol, e.g. `USDC` (wallets still listed) |
| `--active` | — | Filter the wallets by active flag (`true` or `false`) |
| `--default` | — | Restrict the nested token holdings to the default one (`true` or `false`) |
| `--token-active` | — | Restrict the nested token holdings by active flag (`true` or `false`) |
| `--sort` | — | Sort wallets, e.g. `-created_at` |
| `--page` / `--per-page` | — | Page through the wallets (the gateway's default page is 25) |

#### `wallets get <wallet>`

Every `wallets` verb that takes a `<wallet>` (`get`, `update`, `delete`, `balances`,
`tokens add/enable/disable`) accepts the wallet's UUID, its `0x` address, or an
[address-book](#address-book-aliases) `@alias`.

```sh
rail0 wallets get 0xABCD…
rail0 wallets get <wallet_uuid>
rail0 wallets get @merchant
```

#### `wallets create`

> **Currently unavailable.** The gateway refuses `wallets create` for every session with `403 wallet_add_unavailable`, and the CLI prints that adding a wallet is not available yet. A session is bound to the one wallet it signed in with. Adding another wallet returns with the merchant wallet-management section, which keeps the proof below, so the command stays.

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

A new wallet **accepts no token**, so it cannot receive payments until you add one:
payment creation refuses a payee with no active holding. After a successful create the
command prints that next step on stderr (`rail0 wallets tokens add <address> …`), except
under `--quiet`. The result on stdout is unchanged.

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

Existing payments are unaffected. They carry their own payee address and settle whatever the flag says now.

**Switching it back on.** Every switch-off records who did it in the wallet's `deactivated_by` (`owner` or `operator`, shown by `wallets get`). A deactivated wallet's session is read-only (`403 wallet_deactivated`) except for one write: `wallets update <wallet> --active true` on its **own** wallet, which switches it back on when you switched it off yourself. A wallet an operator switched off — or one switched off before the gateway recorded who did it (`deactivated_by` null) — is refused with `403 wallet_deactivated_by_operator`, and the CLI tells you to ask an operator to reactivate it. Another wallet's session cannot change it at all (`403 not_your_wallet`). The flip side: a wallet you switched off can be switched back on by whoever holds its key, so a definitive revocation (a leaked key) must come from an operator.

The account's **only active wallet cannot be switched off** — the gateway refuses it with `422 last_active_wallet`. Login resolves the account from the signing wallet, so an account with no active wallet could neither act nor add a wallet back. Another wallet must be active first; adding one is currently unavailable, so sign in with a wallet you switched off and switch it back on (`--active true`), or ask an operator to reactivate one.

**Only the wallet you signed in with.** `wallets update`, `wallets delete` and `wallets tokens` act on the session's own wallet: another wallet of the same account is refused with `403 not_your_wallet`, and the CLI prints the hint to `rail0 auth login` with that wallet's key. Reads (`wallets`, `wallets get`, `wallets balances`) still cover every wallet of the account.

#### `wallets delete <wallet>`

**A soft delete, despite the verb.** The gateway sets `active=false` and keeps the row — it is the *same operation* as `wallets update <wallet> --active false`, with the same consequences and the same way back. Nothing is removed, nothing on-chain is touched, no funds move.

Both spellings exist because the API exposes a `DELETE` verb as well as the flag. Use whichever reads better where you are writing it. The same last-active-wallet guard (`422 last_active_wallet`) and own-wallet rule (`403 not_your_wallet`) apply.

```sh
rail0 wallets delete 0xABCD…                      # same as…
rail0 wallets update 0xABCD… --active false       # …this
rail0 wallets update 0xABCD… --active true        # back on (its own session, if you switched it off; else an operator)
```

> Contrast [`webhooks delete`](#webhooks), which really does destroy the record.

#### `wallets balances <wallet>`

Read a wallet's live on-chain balances across the configured chains. Each chain reports the native gas-token balance and the active token balances, or an error if that chain's RPC was unreachable (reported per chain, so one dead RPC never hides the others).

```sh
rail0 wallets balances 0xABCD… --chain base --symbol USDC
rail0 wallets balances @merchant --chain arc
```

| Flag | Short | Description |
|------|-------|-------------|
| `--chain` | | Restrict to one chain id |
| `--symbol` | `-s` | Restrict tokens to this symbol (e.g. USDC) |

#### `wallets tokens` — what a wallet accepts

A wallet's **token holdings** — the (chain, token) pairs it accepts — are what
[`payment-methods`](#payment-methods) offers buyers and what payment creation checks: a
wallet with no active holding cannot be paid. So registering a wallet is only half the
setup; this is the other half.

```sh
rail0 wallets tokens add 0xABCD… --chain base --token USDC --default
rail0 wallets tokens disable 0xABCD… USDC --chain base    # alias: remove
rail0 wallets tokens enable 0xABCD… USDC --chain base
```

- `add` is an upsert: a disabled holding is re-enabled, never duplicated. `--token` is a
  symbol resolved on `--chain`, or the token's `0x` address; both fall back to the
  configured `chain` / `token`.
- `enable` / `disable` take `<token>` as a symbol, an address, or the `token_id` shown by
  `rail0 wallets`, matched against the holdings the wallet actually has. When the wallet
  holds one symbol on several chains, `--chain` picks the chain; the CLI never guesses.
- A wallet with active holdings has exactly **one default**, the method buyers see first.
  The first holding takes it; `add … --default` moves it. The default cannot be disabled
  (`422 default_payment_method`), so move it first.
- Disabling is soft and reversible, and asks first (`--yes` skips the prompt). Payments
  already created with that token are unaffected.
- Only on the wallet you signed in with: another wallet of the account is refused with
  `403 not_your_wallet`.

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

List the supported blockchains. Every command's `--chain` takes a chain's `slug` or
its `chain_id` (see [Chains](#chains---chain-slugid)); `public_rpc_urls` are keyless RPC
endpoints for reading the chain directly (`rail0 keys balance`).

Filter with `--symbol` / `-s` (the chain's native symbol) and/or `--network-type` / `-n`
(`testnet` | `mainnet`); both match case-insensitively.

```sh
rail0 chains
rail0 blockchains                 # alias
rail0 chains --symbol USDC        # by native symbol
rail0 chains --network-type testnet
rail0 chains -s ETH -n mainnet    # combined
```

Each chain carries `settlement` — how long an operation has taken to settle **on this
gateway**, broadcast to confirmation, over a trailing window (`p50_seconds`,
`p90_seconds`, `sample_size`, `window_days`) — and a one-line `settlement_human` beside
it: `p50 11s · p90 19s (20 samples, 7d)`, or `n/a (3 samples)` while the chain has too
few confirmed transactions for the percentiles. A measurement, not a guarantee; the p90
is what sizes the default [`--timeout`](#payments-wait-rail0id) of a wait.

---

### Tokens

#### `tokens`

List the token catalog. Filter by chain with `--chain` / `-c`, by token symbol with `--symbol` / `-s` (case-insensitive), and by `--active` (`--active=false` for retired tokens only; omit it for every token).

```sh
rail0 tokens
rail0 tokens --chain arc
rail0 tokens --symbol USDC
```

---

### Webhooks

Manage webhook subscriptions for the authenticated account. A webhook subscribes to one or more payment-lifecycle topics and delivers a signed callback to your endpoint — one subscription means **one** shared secret and **one** circuit breaker however many topics it carries, and each delivery names the event that fired in `X-Rail0-Topic`. Two subscriptions for the same callback URL must not overlap, since the same event arriving twice under two different secrets is indistinguishable from a duplicate at the receiving end. All webhook commands require authentication.

#### Topics

`-t/--topic` completes these in the shell; the gateway is what validates them.

| Topic | Fires when |
| --- | --- |
| `payments.created` | a payment is created |
| `payments.signed` | the payer's signature is stored |
| `payments.authorized` | the authorization is confirmed on-chain — funds in escrow |
| `payments.charged` | a one-shot charge is confirmed |
| `payments.captured` | a capture (full or partial) is confirmed |
| `payments.voided` | the authorization is voided |
| `payments.released` | escrow is released back to the payer |
| `payments.refunded` | a refund is confirmed |
| `payments.authorization_expiring` | **a warning, not a transition**: an authorization with capturable funds (`authorized`, `partially_captured`) is about to lapse — its `authorization_expiry` falls inside the gateway's `AUTHORIZATION_EXPIRING_NOTICE` window (24h by default). Sent **once** per payment, while there is still time to capture; the status does not change and the payload's `transaction` is null. Seconds left = `payment.authorization_expiry` − `emitted_at` |
| `payments.expired` | a never-captured authorization's window lapsed (a window that already closed gets this, never `authorization_expiring`) |
| `payments.failed` | an on-chain attempt failed |
| `payments.disputed` | a dispute is opened |
| `payments.dispute_closed` | a dispute is closed |
| `payouts.confirmed` | a payout's transfer was mined with the expected ERC-20 `Transfer` |
| `payouts.failed` | a payout failed — its `error_code` says how (`reverted`, `dropped`, `cancelled`, …) |
| `payout_destinations.verified` | a payout destination was confirmed (`payouts destinations confirm`) |

Every payment payload carries `capturable_amount` and `authorization_expiry`.

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
rail0 webhooks redeliver <webhook_id> <callback_id>           # 4. replay what was lost
```

Step 3 is not optional after a tripped breaker — `update` changes the URL but not the active state, so repointing a broken webhook at a working endpoint leaves it exactly as dead as it was. And doing 3 before 2 just spends another 10 events reopening the breaker.

Step 4 replays one failed attempt per call, taking the callback id from step 1's rows. It sends the recorded payload **verbatim** — the same event id, so a receiver that already processed it can deduplicate — under a fresh signature. The webhook must be active: a disabled webhook or an open circuit drops the replay silently, which is why it comes after step 3 (`enable` is enough when the webhook was only disabled by hand). Delivery is asynchronous — the command returns `{"status": "queued"}` — so check `rail0 webhooks event-callbacks <webhook_id>` for the new attempt.

#### The shared secret

Returned **once**, on `create` and `rotate-secret`. The gateway stores a hash, not the value, so it cannot be read back — capture it when it appears. `rotate-secret` is the recovery, not a lookup.

Rotation has a **grace window**: for `WEBHOOK_SECRET_GRACE` after `rotate-secret` (one hour by default, a gateway setting) every delivery is signed twice — `X-Rail0-Signature` under the new secret and `X-Rail0-Signature-Previous` under the replaced one — so a receiver still holding the old secret keeps verifying (against the previous header) and nothing in flight at rotation is rejected. Deploy the new secret within the window; past it the old key is gone, and a receiver not updated by then rejects every delivery, which counts as a failure and feeds the breaker. A second rotation inside the window drops the oldest key.

```sh
rail0 webhooks list --topic payments.captured --active true
rail0 webhooks get <webhook_id>
rail0 webhooks create --name "order lifecycle" --callback-url https://example.com/hook \
  -t payments.authorized -t payments.captured -t payments.voided -t payments.refunded
rail0 webhooks create --name "capture reminder" --callback-url https://example.com/hook \
  -t payments.authorization_expiring
rail0 webhooks update <webhook_id> --callback-url https://example.com/new
rail0 webhooks enable <webhook_id>
rail0 webhooks disable <webhook_id>
rail0 webhooks rotate-secret <webhook_id>
rail0 webhooks reset-circuit <webhook_id>
rail0 webhooks test <webhook_id>          # signed webhooks.test event now; shows what the endpoint answered
rail0 webhooks event-callbacks <webhook_id> --status failed
rail0 webhooks redeliver <webhook_id> <callback_id>
rail0 webhooks delete <webhook_id>
```

| Command | Description |
|---------|-------------|
| `list` | List webhooks (`--topic` shows subscriptions that *include* that event, `--active`, `--circuit-state`, `--sort`) |
| `get` | Fetch a single webhook |
| `create` | Register a webhook (`--name`, `--callback-url`, `-t/--topic` repeated or comma-separated; returns shared secret). At most **20 per account** (the gateway's `WEBHOOKS_PER_ACCOUNT` default): past it, 422 `webhook_limit_reached`. Disabled webhooks count toward the limit, deleted ones do not — delete one or add topics to an existing one |
| `update` | Patch `--name`, `--callback-url`, or `--topic` — the topic flag **replaces** the set, which is also how one is removed |
| `enable` | Clear the **manual** disable only — see [the two switches](#two-ways-to-be-off--and-only-one-enable) |
| `disable` | Stop deliveries reversibly, keeping the id and secret (`delete` is the permanent form) |
| `rotate-secret` | Issue a new shared secret (returned once) |
| `reset-circuit` | Close the breaker **and** clear the manual disable — the only way back from a trip |
| `event-callbacks` | Delivery attempts — the log to read when a callback did not arrive (`--status delivered\|failed`, `--response-code`, `--topic`, `--payment-id` as the payment's UUID or its 0x rail0_id — passed to the gateway as is, so any payment of the account matches whichever of its wallets it belongs to, `--since`, `--until`, `--sort`, `--page` / `--per-page`) |
| `redeliver` | Replay one recorded attempt's exact payload (same event id, fresh signature); async, returns `queued` — needs an active webhook, so `enable` / `reset-circuit` first. If the gateway's job queue is down nothing is queued: retry later |
| `delete` | Destroy the webhook and its secret, permanently (unlike `wallets delete`, this one is real) |

---

### Health

#### `health`

Report gateway liveness/readiness, including database connectivity and active chain/contract counts.

```sh
rail0 health
```

---

### Open in the admin (`open`)

Opens the rail0 admin app in the browser: its home page, or — given a payment's
`rail0_id` (`0x…`) or UUID — that payment's page. `--console` opens the operator
console's page for it instead (it needs an admin session in the browser); `--print`
only prints the URL, as the command also does with no terminal or with `--agent`.

The address is the `admin-url` setting (`RAIL0_ADMIN_URL`), or else the one the current
environment declares — development and staging have one, production does not: there,
set it with `rail0 config set admin-url https://…`.

```sh
rail0 open                        # the admin's home page
rail0 open 0xabc123…              # that payment's page
rail0 open 0xabc123… --console    # the console's page for it (operators)
rail0 open 0xabc123… --print      # just the URL
```

### Disputes

> **Operations go through the gateway.** The gateway mirrors only the operations it prepared. A dispute, close-dispute, capture, void, refund or release sent to the RAIL0 contract directly — not with these commands — happens on-chain, but `rail0 payments get` keeps showing the previous status and balances and no webhook is sent; a merchant is not told about a dispute its buyer opened that way. The gateway records it for its operators as a sync error (`external_operation`).

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

Every dispute reads its reasons back next to the raw on-chain bytes32 (`reason`,
`close_reason`): `reason_code` / `reason_description` and `close_reason_code` /
`close_reason_description`. A dispute opened or closed with no reason (the zero bytes32)
has a null code and the description `No reason given`; a non-zero bytes32 outside the
dictionary — a direct contract call — has a null code and the description
`Unrecognised reason`.

#### `disputes reasons`

The dispute-reason dictionary: the codes `--reason` accepts on `payments dispute` and
`payments dispute close`, with what each means. Local — no login, no request.

| List | Codes |
| --- | --- |
| `open` | `not_received`, `not_as_described`, `damaged_or_defective`, `duplicate`, `incorrect_amount`, `cancelled`, `refund_not_received`, `unauthorized`, `other` |
| `close` | `resolved_with_merchant`, `withdrawn`, `item_received`, `other` |
| `system` | `full_refund` — recorded by the protocol when a full refund closes the dispute; never chosen |

On-chain each reason is the bytes32 `keccak256("rail0.dispute.<code>")`; `--json` (or
piped output) adds it to every row.

```sh
rail0 disputes reasons
rail0 disputes reasons --kind close
rail0 disputes reasons --json
```

---

### Analytics

#### `analytics summary` / `timeseries` / `breakdown`

The account's sales analytics. Requires an **account**: a wallet-only session gets
`403 account_required`, because these are the merchant's figures rather than an
address's.

All three take the same filters — `--mode`, `--status`, `--token`, `--chain`,
`--from`, `--to`, `--payee`.

By default the figures cover the **whole account** — every wallet it owns. `--payee`
restricts them to **one** of the account's wallets: a `0x` address or an `@alias` from the
address book (resolved locally, like the other address flags). A wallet the account does
not own is refused with `403`, a malformed address with `400`.

```sh
rail0 analytics summary                          # orders, disputes, refund/dispute rates, volume
rail0 analytics summary --from 2026-01-01T00:00:00Z
rail0 analytics timeseries --interval week       # day (default) | week | month
rail0 analytics breakdown --by token             # token | chain | mode | status | operation
rail0 analytics summary --payee @shop-eu         # just one of the account's wallets
```

Volume figures carry a `*_human` companion (`gross_human`, `settled_human`,
`escrowed_human`, `captured_human`, `refunded_human`, `volume_human`) beside the raw base units, using the `decimals` each
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
| `--chain` | `-c` | yes | — | Chain slug (`arc`) or id as in `chains` |
| `--charge` | `-C` | no | (off) | Create with mode `charge` (immediate, no escrow); default is `authorize` (escrow + capture) |
| `--description` | `-d` | no | — | Reconciliation note (e.g. order id); prompted on a terminal when omitted |
| `--payment-file` | `-f` | no | — | JSON file with the full request body: `chain_id`, `mode` (`authorize` or `charge`), `amount`, `token` (the contract address), `payer`, `payee` required, `description` optional. The file is the whole request: `-F -T -t -a -c -C -d` and the stored chain/token defaults are ignored with it, so put `"mode": "charge"` in the file instead of passing `-C` |
| `--private-key` | `-p` | yes | — | Payer's key — creates and signs locally in one step |
| `--idempotency-key` | — | no | fresh per run | Reuse to retry the same create without minting a second payment |

```sh
rail0 payments create \
  -F @buyer -T @merchant -t USDC -a 10.50 -c arc \
  -p @buyer-key
```

If required flags are missing and stdin is a TTY, an interactive form launches automatically.

---

#### `payments get <rail0Id>`

Fetch the current status of a payment, including its last **confirmed** on-chain balances (mirrors the indexer reports, not live reads — a broadcast but unconfirmed capture or refund is not in them yet) and the transaction list.

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
| `refunded` | Fully settled (both balances zero), by refund or by a final release |
| `partially_refunded` | Legacy — no longer produced; a partial refund now leaves the status unchanged |
| `expired` | Authorization window closed with nothing captured — the escrow is still on-chain and `release` (or `void`) recovers it |

---

#### `payments update <rail0Id>`

Change a payment's off-chain fields, in any status, closed ones included. They live only on the gateway (in nothing signed on-chain), so no signing key is needed, no money moves and no webhook is sent. Requires login.

- `--description`/`-d` (either party, max 255 characters), or `--clear` to remove it — not both.
- `--note` — the **payee's** internal note (max 2000 characters), never shown to the payer.
- `--reference` / `--reference-origin` — the payment's identifier in your own system and which system (a lowercase slug, e.g. `commercelayer`). Clearing the reference clears its origin.

The last three are the payee's (the payer gets `403 not_the_payee`); pass one as `""` to clear it. Only the flags given are sent, and at least one is required.

```sh
rail0 payments update 0xabc123… -d "spring collection"   # set the description
rail0 payments update 0xabc123… --clear                  # remove it
rail0 payments update 0xabc123… --note "refund agreed by email"
rail0 payments update 0xabc123… --reference ORD-1042 --reference-origin commercelayer
```

There is no `failed` payment **status**: `failed` belongs to a **transaction**. A
payment whose transaction reverts keeps the status it already held — the gateway
records the revert on the payment as `last_error_code` / `last_error_message` and
changes nothing else — so the operation can simply be attempted again. A reverted
attempt shows up in `rail0 payments transactions <rail0Id>`, under the transaction's
own status.

---

#### `payments list`

List payments for the authenticated wallet (requires login). Filter with `--status`,
`--mode`, `--payer` / `--payee`, `--token`, `--chain`, `--disputed`, `--operation`, `--reference` / `--reference-origin`,
`--rail0-id`, `--min-amount` / `--max-amount`, and `--created-from` / `--created-to`
(ISO-8601); sort with `--sort`.

`--status` takes **several values**, repeated (`--status authorized --status
partially_captured`) or comma-separated (`--status authorized,partially_captured`), and
matches payments in **any** of them — that example is every payment with escrow still
capturable. Each value is checked against the payment-status vocabulary before any
request, and an unknown one is refused by name.

`--operation` keeps payments carrying at least one transaction of that operation, so
`--operation dispute` finds every payment that was **ever** disputed, which `--disputed`
(an **open** dispute) does not.

`--min-amount` / `--max-amount` are **decimals in token units** (`1.00`), like every
other amount flag, and require `--chain` and `--token`: converting them to the base
units the gateway filters on needs the token's decimals, and an amount range across
tokens with different scales doesn't describe anything. They used to take base units —
the only flags in the CLI that did — so `--min-amount 1.00` quietly meant one
millionth of a USDC and matched everything.

```sh
rail0 payments list
rail0 payments list --sort -created_at
rail0 payments list --status authorized,partially_captured
rail0 payments list --chain arc --disputed
rail0 payments list --chain arc --token 0x3600… --min-amount 1.00
rail0 payments list --operation refund --created-from 2026-09-01T00:00:00Z
```

**`--csv`.** Writes the result as CSV instead of JSON — one page, or every page with
`--all` — with the admin export's columns plus the two deadlines: amounts in base units
and as decimals (`amount_human`, empty when the scale is unknown), `reference` /
`reference_origin`, the payee's `merchant_note` (empty on rows where you are the payer),
`authorization_expiry` / `refund_expiry` as RFC 3339. Free text that a spreadsheet would
run as a formula (`=`, `+`, `-`, `@`) is prefixed with `'`. It cannot be combined with
`--agent`, `--quiet` or `--pretty`.

```sh
rail0 payments list --all --csv > payments.csv
rail0 payments list --created-from 2026-10-01T00:00:00Z --all --csv > october.csv
```

**`--scope all` (admin).** The operator's list: every payment on the gateway, with the
same filters, on the same command. It needs an admin session — anyone else gets `403`,
never a silent narrowing to their own payments. Three filters exist only there, and the
CLI refuses them without it (the gateway would ignore them and hand back your own list,
which reads as an answer): `--account <id>` keeps payments whose payee is one of that
account's wallets, `--has-error` those carrying a `last_error_code` (`--has-error=false`
for the clean ones), and `--expiring-within <duration>` the `authorized` payments whose
hold lapses within that time (`24h`, `90m`; sent as whole seconds, up to 90 days).

```sh
rail0 payments list --scope all --has-error                 # every failing payment, any account
rail0 payments list --scope all --expiring-within 24h       # holds lapsing within a day
rail0 payments list --scope all --account 019e748b-… --all  # one merchant's incoming payments
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
rail0 payments transactions 0xabc123… --status submitting,submitted   # on its way to the chain
```

`--sort` takes `created_at`, `updated_at`, `block_number` or `confirmed_at`;
`--operation` and `--status` use the transaction vocabularies (`close_dispute`,
`failed` and the rest), which `--status` on `payments list` does not. Like there,
`--status` takes several values (repeated or comma-separated) and matches any of them;
each is checked locally first.

#### `payments transaction <rail0Id> <transaction_id>`

One transaction, by the id the lifecycle commands print when the gateway accepts an
operation: its status, the decoded reason of a failed one, and whether a stuck one is
`redrivable`.

#### `payments redrive <rail0Id> <transaction_id>`

Re-broadcast a transaction that was prepared and **signed** but never reached the chain
(a worker that died in between, a drained queue). The same signed bytes go out again;
no new signature is asked for and nothing about the payment changes. Only a row reading
`redrivable: true` qualifies. A `pending` row without a signature is unfinished, not
stuck, so re-run the operation instead. Payee only, and it takes no key: it runs on the
stored session, so `auth login` as the payee first. `-w/--wait` blocks until it
confirms. The gateway also redrives on its own recovery cadence; this asks for it now.
When the gateway's job queue is unreachable it answers 503 `queue_unavailable`: nothing
was enqueued and the transaction is unchanged, so retry later.

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
> `--reason` (dispute, dispute close) to pick it from the list — its first entry, "No
> reason", goes on without one. The signing key (`-p`) is prompted
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

Prepare the `void()` transaction — cancels authorization and returns escrowed funds to payer. Callable on an `authorized` or `expired` payment with nothing captured yet, by the payee; the contract's `void` has no expiry check, so an expired, untouched authorization can still be voided.

```sh
rail0 payments void <rail0Id> -p @merchant-key
```

---

#### `payments release <rail0Id>`

Prepare the `release()` transaction — returns remaining escrowed funds to payer after `authorizationExpiry`. Only the payer or the payee may release — the contract accepts no other sender, and the gateway refuses anyone else with 422 `release_submitter_not_a_party`. The caller is whoever the signing key belongs to (derived from the key), so pass the payer's key to release as the payer.

The status becomes `released` only on a total release (nothing captured). After a capture it stays unchanged while a refundable residual remains — unless the captured part was already refunded, in which case the release leaves both balances at zero and the payment closes as `refunded`.

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

#### `payments dispute <rail0Id>` / `payments dispute close <rail0Id>`

Open or withdraw a dispute. **Payer only**, so these sign with the buyer's key; signal-only
(no money moves), but still on-chain transactions, so `--wait` applies. A merchant cannot
dismiss a dispute — a full `refund` closes it. `payments disputes <rail0Id>` lists one
payment's open/close history.

`--reason` is **optional**: a code from the dispute-reason dictionary — the **open** list
for `dispute`, the **close** list for `dispute close` (see [`disputes reasons`](#disputes-reasons))
— or exactly that code's bytes32. It is checked locally, so a typo fails with the valid
codes before anything is signed; the gateway refuses anything else with 422
`unknown_dispute_reason`. Leave it out (or pass the all-zero bytes32) for no reason: the
dispute records the zero bytes32 and reads back as `No reason given`. On a terminal a
missing `--reason` opens the picker with **No reason** as the first entry; in a script it
simply means no reason.

```sh
rail0 payments dispute <rail0Id> -p @buyer-key --reason not_received --wait
rail0 payments dispute close <rail0Id> -p @buyer-key --reason withdrawn --wait
rail0 payments dispute <rail0Id> -p @buyer-key --wait   # no reason (picker on a terminal)
```

---

#### `payments wait <rail0Id>`

Block until a payment settles — no transaction still in flight — or, with `--until
<status>`, until it reaches that status; it stops early with an error once the status
can no longer be reached. Runs on the stored session.

Without `--timeout` the deadline is sized from the payment's chain: 10× its measured p90
settlement (see `rail0 chains`), kept between `5m` and `15m`, and `15m` when the chain
is not measured yet. The first poll is what names the chain, so this costs no extra
read of the payment (at most one `GET /blockchains` per process).

```sh
rail0 payments wait <rail0Id> --until captured
```

---

### Payouts

Move a merchant wallet's tokens to a deposit address at a venue of your own — an exchange account, Circle Mint, another wallet you control. The gateway builds an unsigned ERC-20 `transfer` from the wallet to a registered **destination**; the CLI checks it is exactly that transfer (the destination's token and chain, its address, the amount, value 0, from your key's wallet) and signs it locally. The gateway never holds funds or keys and never talks to the venue. Every payout command needs a login; reads cover the whole account, writes act for the wallet you sign in with, which must be the destination's. A **deactivated account is read-only**: it cannot prepare, send or change payouts or destinations (`403 account_deactivated`).

#### First payout to a new venue

```sh
# 1. register the deposit address — you confirm you checked the details at the venue
rail0 payouts destinations add -c arc -t USDC -A 0xVenueDeposit… \
  -l "Exchange USDC (Arc)" --min-amount 10

# 2. send the proof: 1 token unit, raised to the minimum (--wait: until confirmed)
rail0 payouts send --destination <destination_id> --proof -p @merchant-key --wait

# 3. once the venue shows the proof CREDITED (not just mined)
rail0 payouts destinations confirm <destination_id>

# 4. pay out — a decimal, or all: the balance minus the refund reserve
rail0 payouts send --destination <destination_id> --amount 250.00 -p @merchant-key
```

A destination is bound to **one chain and one token**: an EVM address is the same string on every chain, but a venue credits only the network it issued the address for. Generate the deposit address at the venue with the network set to `--chain`.

#### Confirming the details (`--confirm-details`)

`destinations add`, and an `update` that **changes** the minimum, show what will be registered followed by the gateway's details statement (`details_v1`):

> I have checked the address, network, token and minimum deposit on the venue's deposit page and confirm they are correct. Funds sent to a wrong address or network, or below the venue's minimum, may be permanently lost and cannot be recovered by rail0.

and need your explicit consent: a y/N prompt (default **no**) on a terminal, or `--confirm-details` without one or with `--agent`. Without it nothing is sent. `--yes` does **not** count — a script that passes it everywhere must not attest, by accident, to having checked a page nobody looked at. The gateway records the attestation (version, who, when) in the destination's audit trail.

#### Minimum and refund reserve

`--min-amount` is the venue's minimum deposit. A venue may ignore a deposit below it, and that deposit can be lost for good — so payouts below it are refused (`payout_below_minimum`) and the proof is raised to it. Raising it above the destination's latest proof takes the verification away: the destination needs a new proof.

`--reserve` is what `send --amount all` leaves in the wallet, so refunds still inside their window can be paid. Unset (or `--reserve ""`), the gateway keeps its suggestion: the refundable balance of the wallet's payments in that token (`suggested_reserve` on the destination).

Both are **decimals in token units** (`10.00`), converted with the token's decimals, like every other amount flag.

#### `payouts destinations` — list (also `destinations list`)

```sh
rail0 payouts destinations --status verified
rail0 payouts destinations get <destination_id>
rail0 payouts destinations update <destination_id> --label "Exchange (main)" --reserve 50
rail0 payouts destinations update <destination_id> --min-amount 25 --confirm-details
rail0 payouts destinations disable <destination_id>     # reversible; any wallet of the account
rail0 payouts destinations enable <destination_id>      # keeps the verification it had
rail0 payouts destinations events <destination_id>      # its audit trail and its payouts'
```

`disable` settles the destination's pending payout first: an unsigned one fails `cancelled`; a **signed** one refuses the switch-off (`409 payout_already_signed`) because its transaction may still be mined. `confirm` asks y/N (`--yes` to skip) — confirm only after you see the deposit at the venue: a transfer can be mined and never credited.

#### `payouts send`

Prepare → verify → sign → submit in one step, with the destination wallet's key (`-p`, `@name`, `RAIL0_PRIVATE_KEY`, or a hidden prompt). Before signing it shows the amount, token, deposit address, network and sending wallet and asks to confirm (y/N); `--yes` skips it and is required without a terminal. With `--amount all` or `--proof` the gateway computes the amount; to see the exact figure first, use `prepare` + `submit`. A payout to a disabled or not-yet-verified destination is refused before the prompt.

The result is the payout, `submitted` with its `transaction_hash` (`-q` prints the hash); it turns `confirmed` once its block is final. A node refusal (502) leaves it `pending` and signed: `payouts resend <id>`.

**`-w/--wait`** (also on `submit` and `resend`) keeps polling the payout instead, on the same loop as the payment waits: the result is the payout once `confirmed`, and a `failed` one exits `1` with its `error_code`/`error_detail` (for `submission_timeout` and `dropped`, which are not final, the message names `report-hash`). A payout is `confirmed` only at the chain's finality, so the wait can run past the first receipt. `--timeout` follows the [payment actions' convention](#payee--action-commands): unset, 10× the chain's measured p90 settlement kept within `5m`–`15m`. A wait that runs out exits `11` — the transfer may still be in flight: check it with `payouts get <id>`, never send it again. Progress goes to stderr (every ~10s off a terminal); `--json`, `-q` and `--agent` behave as on the payment waits.

```sh
rail0 payouts send --destination <destination_id> --amount 250.00 -p @merchant-key --yes --wait
rail0 payouts submit <payout_id> -p @merchant-key --wait --timeout 10m
```

#### `payouts prepare` / `payouts submit <payout_id>`

`prepare` builds the payout without signing and prints it — the exact `amount`/`amount_formatted` and `unsigned_transaction`. Then either `submit <id> -p …` (verify against the destination read again, confirm, sign, submit), or sign it with a wallet of your own and report the hash with `report-hash`. Only a pending, **unsigned** payout can be signed; a signed one is `resend`'s.

#### `payouts` — list (also `payouts list`), `get`, `events`

```sh
rail0 payouts --status submitted,confirmed --purpose payout
rail0 payouts list --destination <destination_id> --created-from 2026-10-01T00:00:00Z
rail0 payouts get <payout_id>
rail0 payouts events <payout_id>
```

A payout goes `pending` → `submitted` → `confirmed`, or `failed` with an `error_code`: `reverted`, `foreign_transfer`, `dropped`, `submission_timeout`, `cancelled`. One pending payout per destination: a new prepare replaces an unsigned one.

#### `payouts resend` / `cancel` / `report-hash`

```sh
rail0 payouts resend <payout_id>                  # same signed bytes again: mined once at most (--wait to follow it)
rail0 payouts cancel <payout_id>                  # an UNSIGNED pending payout; frees the destination
rail0 payouts report-hash <payout_id> 0x5c50…     # a transfer you broadcast yourself
```

`report-hash` also brings back a payout that failed `submission_timeout` or `dropped` because the transaction that went out was sped up or replaced.

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

# The MERCHANT logs in, so the stored session (used by 'payments get') is theirs.
# The signing commands below sign in as their own -p key either way.
rail0 auth login -p @merchant-key

# 1. Buyer creates the payment and signs the EIP-3009 authorization locally
RAIL0_ID=$(rail0 payments create \
  -F @buyer -T @merchant -t USDC -a 1.00 -c arc \
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
  -F @buyer -T @merchant -t USDC -a 1.00 -c arc -C \
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
RAIL0_ID=$(rail0 payments create -F @buyer -T @merchant -t USDC -a 10.50 -c arc -p @buyer-key --json | jq -r .rail0_id)

# Check payment status in a script
STATUS=$(rail0 payments get <rail0Id> --json | jq -r .status)
```

### Agent mode (`--agent`)

For a program driving the CLI — an AI agent above all. Turn it on with `--agent` on each
call, or once with `RAIL0_AGENT=1` in the agent's environment.

- **stdout: exactly one line of JSON per command.** On success
  `{"ok":true,"data":…}`, where `data` is what `--json` prints (`null` for a command
  with no body). On failure:

  ```json
  {"ok":false,
   "error":{"code":"not_capturable","message":"request rejected (HTTP 422): …",
            "exit_code":7,"http_status":422,"hint":"the payment must be 'authorized' …",
            "retryable":false,"in_flight":false},
   "next_steps":["…"]}
  ```

  `code` is the gateway's machine code when the gateway answered, else one of the CLI's:
  `usage_error`, `flag_conflict`, `confirmation_required`, `unavailable_in_agent_mode`,
  `interactive_only`, `aborted`, `wait_timeout`, `network_error`, `cli_error`.
  `retryable` is true only when the identical command is safe to run again (a 429 — see
  `retry_after_s` — or a gateway code saying nothing was submitted). `in_flight` true
  means an on-chain operation may still be running: run `next_steps`, do not repeat it.
  `reason` carries the gateway's sub-cause when there is one.
- **stderr: JSON Lines events only**, e.g. `{"event":"signed_in","address":"0x…","stored":true,"message":"Signed in as 0x…"}`,
  `{"event":"progress","message":"waiting… 30s elapsed of 10m0s"}`,
  `{"event":"dispute_reason","kind":"open","reason":null}`. `--debug` traces are JSON too.
- **Never a prompt, a spinner, colour or an update offer.** A value a terminal would
  ask for is a usage error naming the flag. Commands that move funds or delete things
  still need `--yes`: agent mode is not consent.
- **Not available:** `keys generate` (the private key would land in the agent's
  context — generate it in a terminal and give the agent only an alias) and `init`.
- Cannot be combined with `--quiet` or `--pretty` (exit `2`); `--json` is implied.

```sh
export RAIL0_AGENT=1
rail0 payments get "$ID" | jq -e '.ok' >/dev/null || echo "failed"
rail0 payments capture "$ID" -a 10 -p @merchant-key --yes --wait   # exit 11 = still in flight
```

## Releasing

A release is an approved pull request: merging it is what builds and publishes the
binaries. Never push a `v*` tag by hand.

1. `bin/release-prepare`, on an up-to-date, clean `main`. It computes the version from the titles of the pull requests merged since the last tag
   (Conventional Commits): a breaking change — `!` after the type, as in `feat(api)!:`, or a
   `BREAKING CHANGE:` footer — makes it a major, any `feat` a minor, anything else a patch,
   and it refuses when nothing merged. `bin/release-prepare minor` (or `patch`, `major`,
   `X.Y.Z`) sets it by hand instead. The release PR says which pull requests decided it.
   It then bumps `VERSION`, writes the release notes from the pull
   requests merged since the last tag, and opens `chore(release): vX.Y.Z`. If `go.mod`
   requires an older rail0-go than the latest tag (or a pseudo-version) it says so and
   offers the bump. `--dry-run`
   prints all of it and writes nothing.

   **Or on a feature branch**, to release a change in the same review: run it on a clean
   branch that is pushed, contains `origin/main` and has an open pull request into `main`.
   The bump is committed and pushed on that branch (`chore(release): vX.Y.Z`), the pull
   request's own title counts toward the version, gets a line in the notes and gains a
   ` (release vX.Y.Z)` suffix, and the
   notes go into its description between `<!-- release:start -->` markers — the text above
   them is kept. Re-run it after new commits or a merge of `main`: it refreshes the version
   and the notes instead of bumping twice. One approval then covers the change and its
   release.
2. Review the pull request (edit the notes in its body if needed), approve and merge it.
   The approval must come from a reviewer **other than the pull request's author**, on its
   final commit: release.yml checks it before publishing anything and stops otherwise —
   an admin merge that bypassed the review, a direct push to `main` or a resume of an
   unapproved release publishes nothing.
3. `.github/workflows/release.yml` runs on that push to `main`: CI again on the release
   commit (with the rail0-go tag `go.mod` requires), the tag `vX.Y.Z` = `VERSION`,
   GoReleaser (binaries, archives, `checksums.txt`, notes from the pull request) into a
   draft release, published once complete, then a Slack message.

A merge that does not change `VERSION` releases nothing. If a run stops halfway, Actions →
release → Run workflow (on `main`) resumes it: the tag and GoReleaser's draft are reused.

One-time settings (repository admins):

- environment `release` (Settings → Environments), deployment limited to `main`;
- a tag ruleset on `v*`: block updates and deletions, restrict creation to GitHub Actions
  and admins;
- branch protection on `main`: required `ci` check, "Include administrators";
- the Slack channel: `release.yml` posts through the `PIPELINE_SLACK_CHANNEL_WEBHOOK_URL`
  secret, named in one place (the `announce` job).

## License

MIT — see [LICENSE](LICENSE).
