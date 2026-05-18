# rail0-cli

Command-line interface for the [RAIL0](https://rail0.xyz) stablecoin payment protocol. Wraps the RAIL0 REST API and exposes every endpoint as a subcommand with human-friendly styled output on a terminal and machine-readable JSON when piped.

## Requirements

- Go 1.22 or later
- Access to the RAIL0 API (`https://api.rail0.xyz` or a self-hosted instance)

## Installation

### Build from source

The CLI depends on the [rail0-go](../rail0-go) SDK. Both repositories must be present as siblings:

```
Documents/GitHub/
├── rail0-cli/   ← this repo
└── rail0-go/    ← Go SDK
```

```sh
git clone https://github.com/rail0/cli rail0-cli
git clone https://github.com/DrPierlu/rail0-go rail0-go

cd rail0-cli
go build -o rail0 .
```

Move the binary to a directory on your `$PATH`:

```sh
mv rail0 /usr/local/bin/
```

## Configuration

Every flag can be set via an environment variable. The flag always takes priority.

| Flag          | Environment variable | Description                                 |
|---------------|----------------------|---------------------------------------------|
| `--base-url`  | `RAIL0_BASE_URL`     | RAIL0 API base URL (default: no base URL)   |
| `--api-key`   | `RAIL0_API_KEY`      | Bearer token sent in the `Authorization` header |
| `--debug`     | —                    | Print raw HTTP request/response to stderr   |
| `--json`      | —                    | Force plain JSON output (useful in scripts) |

Minimal setup:

```sh
export RAIL0_BASE_URL=https://api.rail0.xyz
export RAIL0_API_KEY=your_api_key
```

## Usage

```
rail0 <command> <subcommand> [flags]
```

### Passing a Payment object

Most commands require the eight fields of the `Payment` struct. There are three ways to supply them:

**1. Individual flags** (good for one-off commands):

```sh
rail0 payments hash \
  --payer 0xBuyer... \
  --payee 0xMerchant... \
  --token 0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913 \
  --max-amount 100000000 \
  --auth-expiry 1999999999 \
  --refund-expiry 2000000000
```

**2. A JSON file** via `--payment-file` (good for automation):

```json
{
  "payer":               "0xBuyerAddress",
  "payee":               "0xMerchantAddress",
  "token":               "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
  "maxAmount":           "100000000",
  "authorizationExpiry": 1999999999,
  "refundExpiry":        2000000000,
  "feeBps":              0,
  "feeReceiver":         "0x0000000000000000000000000000000000000000"
}
```

```sh
rail0 payments hash --payment-file payment.json
```

**3. Interactive form** (when running in a terminal with no flags set):

If required fields are missing and stdin is a TTY, the CLI launches a two-page [huh](https://github.com/charmbracelet/huh) form. Navigate with Tab / Shift-Tab, confirm with Enter, and cancel with Esc.

---

## Commands

### `payments get <paymentId>`

Fetch the on-chain state of a payment.

```sh
rail0 payments get 0xdeadbeef...
```

### `payments hash`

Compute the EIP-712 digest of a Payment configuration. The returned hash is the `configHash` used for nonce derivation.

```sh
rail0 payments hash --payment-file payment.json
```

### `payments authorize-nonce <paymentId>`

Return the EIP-3009 nonce the buyer must use when signing for `authorize`. Requires the `configHash` from `payments hash`.

```sh
CONFIG_HASH=$(rail0 payments hash --payment-file payment.json --json | jq -r .hash)

rail0 payments authorize-nonce 0xdeadbeef... --config-hash "$CONFIG_HASH"
```

### `payments charge-nonce <paymentId>`

Same as `authorize-nonce` but for the `charge` flow.

```sh
rail0 payments charge-nonce 0xdeadbeef... --config-hash "$CONFIG_HASH"
```

### `payments authorize <paymentId>`

Pull funds into escrow. Requires the EIP-3009 signature (`--v`, `--r`, `--s`) produced off-chain by the buyer.

```sh
rail0 payments authorize 0xdeadbeef... \
  --payment-file payment.json \
  --amount 50000000 \
  --v 27 \
  --r 0xabc... \
  --s 0xdef...
```

### `payments charge <paymentId>`

Authorize and capture in a single transaction (no escrow window).

```sh
rail0 payments charge 0xdeadbeef... \
  --payment-file payment.json \
  --amount 25000000 \
  --v 27 --r 0xabc... --s 0xdef...
```

### `payments capture <paymentId>`

Move escrowed funds to the payee.

```sh
rail0 payments capture 0xdeadbeef... \
  --payment-file payment.json \
  --amount 50000000
```

### `payments void <paymentId>`

Cancel an authorization and return escrowed funds to the payer.

```sh
rail0 payments void 0xdeadbeef... --payment-file payment.json
```

### `payments release <paymentId>`

Return escrowed funds to the payer after `authorizationExpiry` (permissionless).

```sh
rail0 payments release 0xdeadbeef... --payment-file payment.json
```

### `payments refund <paymentId>`

Refund a captured amount from the payee back to the payer.

```sh
rail0 payments refund 0xdeadbeef... \
  --payment-file payment.json \
  --amount 25000000
```

### `tokens check <address>`

Check whether a token is in the RAIL0 allowlist.

```sh
rail0 tokens check 0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913
```

### `utils domain-separator`

Print the current EIP-712 domain separator of the deployed RAIL0 contract.

```sh
rail0 utils domain-separator
```

### `utils version`

Print the RAIL0 contract version.

```sh
rail0 utils version
```

---

## Output modes

**Terminal (default):** lipgloss-styled key-value layout. Hex values are highlighted in blue, numbers in amber, booleans in green/red.

**JSON (`--json` or piped):** plain indented JSON on stdout — suitable for `jq` and shell pipelines.

```sh
# Extract the transaction hash from an authorize call
rail0 payments authorize 0xdeadbeef... --payment-file p.json \
  --amount 50000000 --v 27 --r 0x... --s 0x... \
  --json | jq -r .transactionHash
```

---

## Authorize-and-capture example (full flow)

```sh
export RAIL0_BASE_URL=https://api.rail0.xyz
PAYMENT_ID=0xdeadbeef000000000000000000000000000000000000000000000000000000001

# 1. Compute configHash
CONFIG_HASH=$(rail0 payments hash --payment-file payment.json --json | jq -r .hash)

# 2. Fetch the authorize nonce
NONCE=$(rail0 payments authorize-nonce "$PAYMENT_ID" --config-hash "$CONFIG_HASH" --json | jq -r .nonce)

# 3. Sign off-chain with the buyer's key (use the Go/Rust/TS SDK or cast sign-typed-data)
# V=27 R=0x... S=0x...

# 4. Submit the authorization
rail0 payments authorize "$PAYMENT_ID" \
  --payment-file payment.json \
  --amount 50000000 \
  --v "$V" --r "$R" --s "$S"

# 5. Capture when ready
rail0 payments capture "$PAYMENT_ID" \
  --payment-file payment.json \
  --amount 50000000
```
