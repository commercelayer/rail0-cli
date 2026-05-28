package cmd

import (
	"context"

	"github.com/rail0/cli/internal/ui"
	rail0 "github.com/rail0/go-sdk"
	"github.com/spf13/cobra"
)

var paymentsCmd = &cobra.Command{
	Use:   "payments",
	Short: "Payment lifecycle operations",
	Long: `Manage the full RAIL0 payment lifecycle.

A typical authorize-and-capture flow:
  1. payments create     — payer creates the payment and receives a signingPayload
  2. payments sign       — payer signs the signingPayload (off-chain, wallet side)
  3. payments authorize  — payee prepares the on-chain authorize transaction
  4. payments submit     — payee broadcasts the signed authorize transaction
  5. payments capture    — payee prepares the on-chain capture transaction
  6. payments submit     — payee broadcasts the signed capture transaction

A charge (one-shot) flow skips escrow:
  1. payments create --mode charge
  2. payments sign
  3. payments charge
  4. payments submit

Other operations (void, release, approve, refund) each follow the same
prepare → submit pattern.`,
}

// ── payments create ──────────────────────────────────────────────────────────

var createFlags createPaymentFlags

var paymentsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a payment and receive the EIP-712 signing payload",
	Long: `Create a new RAIL0 payment intent.

The API generates a unique paymentId, applies the server-side policy
(authorizationExpiry, refundExpiry, feeBps, feeReceiver) and returns an
EIP-712 typed-data signingPayload for the payer to sign off-chain.

After creation, the payer must sign the signingPayload with their wallet
(eth_signTypedData_v4) and submit the resulting signature via:

  rail0 payments sign <paymentId> --signature <0x…>

Required flags: --payer, --payee, --token, --amount, --chain-id
Optional flags: --mode (default: authorize)

Alternatively pass --payment-file with a JSON file containing the full
request body to skip individual flags.`,
	Example: `  # Interactive mode (TTY only — prompts for missing fields)
  rail0 payments create

  # All flags inline
  rail0 payments create \
    --payer  0xBuyer… \
    --payee  0xMerchant… \
    --token  0xcebA9300f2b948710d2De3250b7Ad3e4aFb0e50a \
    --amount 100000000 \
    --chain-id 42220

  # Charge mode (no escrow window)
  rail0 payments create --mode charge \
    --payer 0xBuyer… --payee 0xMerchant… \
    --token 0xcebA9300… --amount 50000000 --chain-id 42220

  # From a JSON file
  rail0 payments create --payment-file payment.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req, err := createFlags.build()
		if err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return newClient().Payments.CreatePayment(context.Background(), req)
		})
	},
}

// ── payments get ─────────────────────────────────────────────────────────────

var paymentsGetCmd = &cobra.Command{
	Use:   "get <paymentId>",
	Short: "Fetch the current state of a payment",
	Long: `Fetch a payment's current status from the RAIL0 API together with live
on-chain balances (capturableAmount, refundableAmount) when the payment
is in an active on-chain state.

Returned status values:
  pending_signature   — created, payer signature not yet submitted
  signed              — payer signature stored, awaiting payee action
  submitting          — signed transaction being broadcast to the network
  submitted           — transaction broadcast, waiting for on-chain confirmation
  authorized          — funds held in escrow, capture available
  charged             — one-shot charge executed, refund available
  captured            — funds fully captured by payee
  partially_captured  — partial capture executed; remaining escrow still capturable
  voided              — authorization cancelled, funds returned to payer
  released            — expired authorization released back to payer
  refunded            — captured funds fully refunded to payer
  partially_refunded  — partial refund executed; remaining balance still refundable
  failed              — terminal failure; payment cannot proceed`,
	Example: `  rail0 payments get 0xabc123…`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newClient().Payments.Get(context.Background(), args[0])
		})
	},
}

// ── payments sign ─────────────────────────────────────────────────────────────

var signSignature string

var paymentsSignCmd = &cobra.Command{
	Use:   "sign <paymentId>",
	Short: "Submit the payer's EIP-712 signature",
	Long: `Submit the payer's EIP-3009 signature to the RAIL0 API.

The signature is produced by the payer's wallet (e.g. MetaMask) when
calling eth_signTypedData_v4 on the signingPayload returned by
'payments create'. The API verifies that the recovered address matches
payment.payer and stores the signature for the payee to use.

--signature must be the raw 65-byte secp256k1 signature as a
0x-prefixed 130-character hex string (r[32] + s[32] + v[1]) exactly
as returned by eth_signTypedData_v4.`,
	Example: `  rail0 payments sign 0xpaymentId… \
    --signature 0x1a2b3c…(130 hex chars)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newClient().Payments.Sign(context.Background(), args[0],
				rail0.PayerSignatureRequest{Signature: signSignature})
		})
	},
}

// ── payments authorize ────────────────────────────────────────────────────────

var paymentsAuthorizeCmd = &cobra.Command{
	Use:   "authorize <paymentId>",
	Short: "Prepare the unsigned authorize transaction",
	Long: `Prepare an unsigned EIP-1559 transaction that calls authorize() on the
RAIL0 contract. The payer signature must have been submitted first via
'payments sign'.

The returned unsignedTransaction is an RLP-encoded type-2 transaction
ready for the payee to sign. Options:
  • Browser wallet: pass to eth_signTransaction
  • In-process key: sign with ethers.js wallet.signTransaction or any
    secp256k1 library

After signing, broadcast the result with:
  rail0 payments submit <paymentId> --signed-tx <0x…>`,
	Example: `  rail0 payments authorize 0xpaymentId…`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newClient().Payments.Authorize(context.Background(), args[0])
		})
	},
}

// ── payments charge ───────────────────────────────────────────────────────────

var paymentsChargeCmd = &cobra.Command{
	Use:   "charge <paymentId>",
	Short: "Prepare the unsigned charge transaction (one-shot, no escrow)",
	Long: `Prepare an unsigned EIP-1559 transaction that calls charge() on the
RAIL0 contract. The payment must have been created with --mode charge
and the payer signature submitted via 'payments sign'.

Charge combines authorize + capture into a single on-chain call:
funds are transferred directly from payer to payee without an escrow
window, making it suitable for immediate settlements.

After signing, broadcast the result with:
  rail0 payments submit <paymentId> --signed-tx <0x…>`,
	Example: `  rail0 payments charge 0xpaymentId…`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newClient().Payments.Charge(context.Background(), args[0])
		})
	},
}

// ── payments capture ──────────────────────────────────────────────────────────

var captureAmount string

var paymentsCaptureCmd = &cobra.Command{
	Use:   "capture <paymentId>",
	Short: "Prepare the unsigned capture transaction",
	Long: `Prepare an unsigned EIP-1559 transaction that calls capture() on the
RAIL0 contract. The payment must be in 'authorized', 'captured', or
'partially_captured' status.

Partial captures are supported: pass an --amount less than the full
capturableAmount to capture only a portion. The remaining escrowed
balance stays available for additional captures until authorizationExpiry.
Each additional capture transitions the payment back through 'submitting'
→ 'submitted' → 'partially_captured' until the full amount is captured.

After authorizationExpiry the remaining balance can only be released
(see 'payments release').

After signing, broadcast the result with:
  rail0 payments submit <paymentId> --signed-tx <0x…>`,
	Example: `  # Full capture
  rail0 payments capture 0xpaymentId… --amount 100000000

  # Partial capture (50 USDC out of 100 escrowed)
  rail0 payments capture 0xpaymentId… --amount 50000000`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newClient().Payments.PrepareCapture(context.Background(), args[0],
				rail0.CapturePaymentRequest{Amount: captureAmount})
		})
	},
}

// ── payments void ─────────────────────────────────────────────────────────────

var paymentsVoidCmd = &cobra.Command{
	Use:   "void <paymentId>",
	Short: "Prepare the unsigned void transaction",
	Long: `Prepare an unsigned EIP-1559 transaction that calls void() on the RAIL0
contract. The payment must be in 'authorized' status.

Void cancels an authorization and returns the full escrowed balance to
the payer. It can only be called before authorizationExpiry and only by
the payee.

After signing, broadcast the result with:
  rail0 payments submit <paymentId> --signed-tx <0x…>`,
	Example: `  rail0 payments void 0xpaymentId…`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newClient().Payments.PrepareVoid(context.Background(), args[0])
		})
	},
}

// ── payments release ──────────────────────────────────────────────────────────

var releaseCallerAddress string

var paymentsReleaseCmd = &cobra.Command{
	Use:   "release <paymentId>",
	Short: "Prepare the unsigned release transaction",
	Long: `Prepare an unsigned EIP-1559 transaction that calls release() on the
RAIL0 contract. Release is only available after authorizationExpiry has
passed on-chain.

Release returns the remaining escrowed balance to the payer. It is
permissionless — both payer and payee can initiate it.

Use --caller-address to build the transaction for the payer (so they
can sign and submit it themselves). Omit the flag to build for the payee.

After signing, broadcast the result with:
  rail0 payments submit <paymentId> --signed-tx <0x…>`,
	Example: `  # Build for payee (default)
  rail0 payments release 0xpaymentId…

  # Build for payer
  rail0 payments release 0xpaymentId… --caller-address 0xBuyer…`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newClient().Payments.PrepareRelease(context.Background(), args[0],
				rail0.ReleaseRequest{CallerAddress: releaseCallerAddress})
		})
	},
}

// ── payments approve ──────────────────────────────────────────────────────────

var approveAmount string

var paymentsApproveCmd = &cobra.Command{
	Use:   "approve <paymentId>",
	Short: "Prepare the unsigned ERC-20 approve transaction (needed before refund)",
	Long: `Prepare an unsigned EIP-1559 transaction that calls ERC-20 approve() on
the token contract, granting the RAIL0 contract an allowance to pull
funds from the payee for a future refund.

This step is required before 'payments refund' when the payee's wallet
has insufficient allowance. It only needs to be done once per (payee,
token, RAIL0 contract) combination — or set --amount to
115792089237316195423570985008687907853269984665640564039457584007913129639935
for an unlimited approval.

After signing, broadcast the result with:
  rail0 payments submit <paymentId> --signed-tx <0x…>`,
	Example: `  # Approve exact refund amount
  rail0 payments approve 0xpaymentId… --amount 50000000

  # Unlimited approval (type(uint256).max)
  rail0 payments approve 0xpaymentId… \
    --amount 115792089237316195423570985008687907853269984665640564039457584007913129639935`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newClient().Payments.PrepareApprove(context.Background(), args[0],
				rail0.ApproveRequest{Amount: approveAmount})
		})
	},
}

// ── payments refund ───────────────────────────────────────────────────────────

var refundAmount string

var paymentsRefundCmd = &cobra.Command{
	Use:   "refund <paymentId>",
	Short: "Prepare the unsigned refund transaction",
	Long: `Prepare an unsigned EIP-1559 transaction that calls refund() on the
RAIL0 contract. The payment must be in 'captured', 'partially_captured',
'charged', 'refunded', or 'partially_refunded' status and must still
have a positive refundableAmount.

The RAIL0 contract pulls refundAmount from the payee and returns it to
the payer. The payee must first grant sufficient allowance to the RAIL0
contract via 'payments approve'.

Partial refunds are supported: pass an --amount less than the current
refundableAmount to refund only a portion.

After signing, broadcast the result with:
  rail0 payments submit <paymentId> --signed-tx <0x…>`,
	Example: `  # Full refund
  rail0 payments refund 0xpaymentId… --amount 100000000

  # Partial refund
  rail0 payments refund 0xpaymentId… --amount 25000000`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newClient().Payments.PrepareRefund(context.Background(), args[0],
				rail0.RefundPaymentRequest{Amount: refundAmount})
		})
	},
}

// ── payments submit ───────────────────────────────────────────────────────────

var submitSignedTx string

var paymentsSubmitCmd = &cobra.Command{
	Use:   "submit <paymentId>",
	Short: "Broadcast a signed transaction (any operation)",
	Long: `Broadcast a signed transaction to the network via the RAIL0 API.

Universal submit endpoint — works after any prepare step: authorize,
charge, capture, void, release, approve, refund. The API infers the
operation from pending_operation set during the preceding prepare call.

The API enqueues the transaction and returns immediately with status
"submitting" (HTTP 202). Poll 'payments get <paymentId>' until the
status advances past "submitting" to learn the final outcome.

--signed-tx must be the RLP-encoded signed EIP-1559 transaction as
returned by the wallet's signTransaction call (0x-prefixed hex).`,
	Example: `  # After any prepare step (authorize, capture, void, release, approve, refund…)
  rail0 payments submit 0xpaymentId… --signed-tx 0x02f8…

  # Then poll until the transaction is confirmed
  rail0 payments get 0xpaymentId…`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newClient().Payments.Submit(context.Background(), args[0],
				rail0.SubmitTransactionRequest{SignedTransaction: submitSignedTx})
		})
	},
}

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	// payments create
	createFlags.addTo(paymentsCreateCmd)
	paymentsCmd.AddCommand(paymentsCreateCmd)

	// payments get
	paymentsCmd.AddCommand(paymentsGetCmd)

	// payments sign
	paymentsSignCmd.Flags().StringVar(&signSignature, "signature", "",
		"65-byte EIP-712 signature (0x-prefixed 130-char hex) from eth_signTypedData_v4")
	_ = paymentsSignCmd.MarkFlagRequired("signature")
	paymentsCmd.AddCommand(paymentsSignCmd)

	// payments authorize
	paymentsCmd.AddCommand(paymentsAuthorizeCmd)

	// payments charge
	paymentsCmd.AddCommand(paymentsChargeCmd)

	// payments capture
	paymentsCaptureCmd.Flags().StringVar(&captureAmount, "amount", "",
		"Amount to capture in token base units (must be > 0 and ≤ capturableAmount)")
	_ = paymentsCaptureCmd.MarkFlagRequired("amount")
	paymentsCmd.AddCommand(paymentsCaptureCmd)

	// payments void
	paymentsCmd.AddCommand(paymentsVoidCmd)

	// payments release
	paymentsReleaseCmd.Flags().StringVar(&releaseCallerAddress, "caller-address", "",
		"Address that will sign and submit the release tx (default: payee address on record)")
	paymentsCmd.AddCommand(paymentsReleaseCmd)

	// payments approve
	paymentsApproveCmd.Flags().StringVar(&approveAmount, "amount", "",
		"ERC-20 allowance to grant in token base units (use max uint256 for unlimited)")
	_ = paymentsApproveCmd.MarkFlagRequired("amount")
	paymentsCmd.AddCommand(paymentsApproveCmd)

	// payments refund
	paymentsRefundCmd.Flags().StringVar(&refundAmount, "amount", "",
		"Amount to refund in token base units (must be > 0 and ≤ refundableAmount)")
	_ = paymentsRefundCmd.MarkFlagRequired("amount")
	paymentsCmd.AddCommand(paymentsRefundCmd)

	// payments submit
	paymentsSubmitCmd.Flags().StringVar(&submitSignedTx, "signed-tx", "",
		"RLP-encoded signed EIP-1559 transaction (0x-prefixed hex) from eth_signTransaction")
	_ = paymentsSubmitCmd.MarkFlagRequired("signed-tx")
	paymentsCmd.AddCommand(paymentsSubmitCmd)
}
