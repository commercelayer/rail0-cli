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

## Commands

### Merchants

#### `merchants payment-methods <merchantId>`

List the accepted payment methods for a merchant. The returned `walletAddress`, `tokenAddress`, and `chainId` fields map directly to the `--payee`, `--token`, and `--chain-id` flags of `payments create`.

```sh
rail0 merchants payment-methods 42
rail0 merchants payment-methods 42 --json
```

---

### Payments

Every on-chain operation follows the same **prepare → sign offline → submit** pattern. The prepare commands (`authorize`, `charge`, `capture`, `void`, `release`, `approve`, `refund`) return an `unsignedTransaction` (RLP-encoded EIP-1559). Sign it with any secp256k1 wallet, then broadcast the result with `payments submit`.

#### `payments create`

Create a payment intent. Returns a `paymentId` and an EIP-712 `signingPayload` for the payer to sign off-chain.

Flags:

| Flag              | Required | Default      | Description                                      |
|-------------------|----------|--------------|--------------------------------------------------|
| `--payer`         | yes      | —            | Buyer wallet address (0x-prefixed)               |
| `--payee`         | yes      | —            | Merchant wallet address (0x-prefixed)            |
| `--token`         | yes      | —            | ERC-20 token address (0x-prefixed)               |
| `--amount`        | yes      | —            | Amount in token base units (e.g. `100000000`)    |
| `--chain-id`      | yes      | —            | EVM chain ID (e.g. `42220` for Celo)             |
| `--mode`          | no       | `authorize`  | `authorize` (escrow + capture) or `charge` (immediate) |
| `--payment-file`  | no       | —            | Path to a JSON file with the full request body   |

```sh
# All flags inline
rail0 payments create \
  --payer  0xBuyer… \
  --payee  0xMerchant… \
  --token  0xcebA9300f2b948710d2De3250b7Ad3e4aFb0e50a \
  --amount 100000000 \
  --chain-id 42220

# Charge mode (no escrow)
rail0 payments create --mode charge \
  --payer 0xBuyer… --payee 0xMerchant… \
  --token 0xcebA9300… --amount 50000000 --chain-id 42220

# From a JSON file
rail0 payments create --payment-file payment.json
```

If required flags are missing and stdin is a TTY, an interactive form launches automatically.

---

#### `payments get <paymentId>`

Fetch the current status of a payment, including live on-chain balances (`capturableAmount`, `refundableAmount`) when relevant.

```sh
rail0 payments get 0xabc123…
```

Payment status values:

| Status                 | Description                                                    |
|------------------------|----------------------------------------------------------------|
| `pending_signature`    | Created; payer signature not yet submitted                     |
| `signed`               | Payer signature stored; awaiting payee action                  |
| `submitting`           | Signed transaction being broadcast                             |
| `submitted`            | Transaction broadcast; awaiting on-chain confirmation          |
| `authorized`           | Funds held in escrow; capture available                        |
| `charged`              | One-shot charge executed; refund available                     |
| `captured`             | Funds fully captured by payee                                  |
| `partially_captured`   | Partial capture executed; remaining escrow still capturable    |
| `voided`               | Authorization cancelled; funds returned to payer               |
| `released`             | Expired authorization released back to payer                   |
| `refunded`             | Captured funds fully refunded to payer                         |
| `partially_refunded`   | Partial refund executed; remaining balance still refundable    |
| `failed`               | Terminal failure; payment cannot proceed                       |

---

#### `payments sign <paymentId> --signature <0x…>`

Submit the payer's EIP-712 signature to the API. The signature is produced by calling `eth_signTypedData_v4` on the `signingPayload` returned by `payments create`.

`--signature` must be a 0x-prefixed 130-character hex string (65 bytes: r[32] + s[32] + v[1]).

```sh
rail0 payments sign 0xpaymentId… \
  --signature 0x1a2b3c…
```

---

#### `payments authorize <paymentId>`

Prepare the unsigned EIP-1559 transaction that calls `authorize()` on the RAIL0 contract, pulling funds into escrow. The payer signature must have been submitted first.

```sh
rail0 payments authorize 0xpaymentId…
```

---

#### `payments charge <paymentId>`

Prepare the unsigned transaction for a one-shot `charge()` (authorize + capture in one call, no escrow window). The payment must have been created with `--mode charge`.

```sh
rail0 payments charge 0xpaymentId…
```

---

#### `payments capture <paymentId> --amount <n>`

Prepare the unsigned transaction that calls `capture()`. Partial captures are supported: pass an `--amount` less than `capturableAmount` to capture a portion; the remaining balance stays available until `authorizationExpiry`.

```sh
# Full capture
rail0 payments capture 0xpaymentId… --amount 100000000

# Partial capture
rail0 payments capture 0xpaymentId… --amount 50000000
```

---

#### `payments void <paymentId>`

Prepare the unsigned transaction that cancels an authorization and returns the full escrowed balance to the payer. Only available before `authorizationExpiry`, by the payee.

```sh
rail0 payments void 0xpaymentId…
```

---

#### `payments release <paymentId>`

Prepare the unsigned transaction that returns remaining escrowed funds to the payer after `authorizationExpiry`. Permissionless — both payer and payee can initiate it.

Use `--caller-address` to build the transaction for the payer (so they sign and submit it themselves). Omit to build for the payee.

```sh
# Build for payee (default)
rail0 payments release 0xpaymentId…

# Build for payer
rail0 payments release 0xpaymentId… --caller-address 0xBuyer…
```

---

#### `payments approve <paymentId> --amount <n>`

Prepare the unsigned ERC-20 `approve()` transaction granting the RAIL0 contract an allowance to pull funds from the payee for a future refund. Required before `payments refund` if the payee has insufficient allowance.

```sh
# Approve exact amount
rail0 payments approve 0xpaymentId… --amount 100000000

# Unlimited approval
rail0 payments approve 0xpaymentId… \
  --amount 115792089237316195423570985008687907853269984665640564039457584007913129639935
```

---

#### `payments refund <paymentId> --amount <n>`

Prepare the unsigned transaction that calls `refund()`, pulling `amount` from the payee and returning it to the payer. Partial refunds are supported. Requires prior `approve` if allowance is insufficient.

```sh
# Full refund
rail0 payments refund 0xpaymentId… --amount 100000000

# Partial refund
rail0 payments refund 0xpaymentId… --amount 25000000
```

---

#### `payments submit <paymentId> --signed-tx <0x…>`

Broadcast a signed transaction to the network via the RAIL0 API. Works after any prepare step (`authorize`, `charge`, `capture`, `void`, `release`, `approve`, `refund`) — the API infers the operation from the pending state set during the preceding prepare call.

Returns immediately with `{ "status": "submitting" }` (HTTP 202). Poll `payments get <paymentId>` until the status advances past `submitting`.

`--signed-tx` must be the RLP-encoded signed EIP-1559 transaction as a 0x-prefixed hex string.

```sh
rail0 payments submit 0xpaymentId… --signed-tx 0x02f8…

# Poll for outcome
rail0 payments get 0xpaymentId…
```

---

## Full flow examples

### Authorize-and-capture

The standard escrow flow: payer creates and signs off-chain; payee authorizes, captures, and optionally refunds.

```sh
export RAIL0_BASE_URL=https://api.rail0.xyz
export RAIL0_API_KEY=your_api_key

# 1. Payer creates the payment and receives signingPayload
PAYMENT=$(rail0 payments create \
  --payer  0xBuyer… \
  --payee  0xMerchant… \
  --token  0xcebA9300f2b948710d2De3250b7Ad3e4aFb0e50a \
  --amount 100000000 \
  --chain-id 42220 \
  --json)
PAYMENT_ID=$(echo "$PAYMENT" | jq -r .paymentId)

# 2. Payer signs signingPayload with their wallet (eth_signTypedData_v4)
#    and submits the 65-byte signature
rail0 payments sign "$PAYMENT_ID" --signature 0x1a2b3c…

# 3. Payee prepares the authorize transaction
UNSIGNED=$(rail0 payments authorize "$PAYMENT_ID" --json | jq -r .unsignedTransaction)

# 4. Payee signs the transaction offline (ethers.js, cast, etc.) → SIGNED_TX

# 5. Payee broadcasts it
rail0 payments submit "$PAYMENT_ID" --signed-tx "$SIGNED_TX"

# 6. Poll until authorized
rail0 payments get "$PAYMENT_ID"

# 7. Payee prepares and submits the capture transaction
UNSIGNED=$(rail0 payments capture "$PAYMENT_ID" --amount 100000000 --json | jq -r .unsignedTransaction)
# sign → SIGNED_TX
rail0 payments submit "$PAYMENT_ID" --signed-tx "$SIGNED_TX"
rail0 payments get "$PAYMENT_ID"   # → captured
```

---

### Charge (one-shot, no escrow)

```sh
# 1. Create with charge mode
PAYMENT_ID=$(rail0 payments create --mode charge \
  --payer 0xBuyer… --payee 0xMerchant… \
  --token 0xcebA9300… --amount 50000000 --chain-id 42220 \
  --json | jq -r .paymentId)

# 2. Payer signs and submits signature
rail0 payments sign "$PAYMENT_ID" --signature 0x…

# 3. Payee prepares, signs, and submits the charge transaction
UNSIGNED=$(rail0 payments charge "$PAYMENT_ID" --json | jq -r .unsignedTransaction)
# sign → SIGNED_TX
rail0 payments submit "$PAYMENT_ID" --signed-tx "$SIGNED_TX"
rail0 payments get "$PAYMENT_ID"   # → charged
```

---

### Refund

```sh
# Ensure the payee has granted sufficient allowance to the RAIL0 contract
UNSIGNED=$(rail0 payments approve "$PAYMENT_ID" --amount 25000000 --json | jq -r .unsignedTransaction)
# sign → SIGNED_TX
rail0 payments submit "$PAYMENT_ID" --signed-tx "$SIGNED_TX"

# Prepare and submit the refund
UNSIGNED=$(rail0 payments refund "$PAYMENT_ID" --amount 25000000 --json | jq -r .unsignedTransaction)
# sign → SIGNED_TX
rail0 payments submit "$PAYMENT_ID" --signed-tx "$SIGNED_TX"
rail0 payments get "$PAYMENT_ID"   # → refunded or partially_refunded
```

---

## Output modes

**Terminal (default):** lipgloss-styled output with highlighted hex values, amounts, and status fields.

**JSON (`--json` or piped):** plain indented JSON on stdout, suitable for `jq` and shell pipelines.

```sh
# Extract unsignedTransaction from any prepare step
rail0 payments authorize 0xpaymentId… --json | jq -r .unsignedTransaction

# Check payment status in a script
STATUS=$(rail0 payments get 0xpaymentId… --json | jq -r .status)
```
