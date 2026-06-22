package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/rail0/cli/internal/ui"
	rail0 "github.com/rail0/go-sdk"
	"github.com/spf13/cobra"
)

var paymentsCmd = &cobra.Command{
	Use:   "payments",
	Short: "Payment lifecycle operations",
	Long: `Manage the full RAIL0 payment lifecycle.

Every on-chain operation supports two signing modes:

  Direct key (automated)
    Pass --private-key (-p) to any action command. The CLI prepares the
    unsigned transaction, signs it locally, and broadcasts it in one step.

      rail0 payments authorize <rail0Id> -p 0xYourPrivateKey…

  External wallet (MetaMask, Ledger, hardware wallet)
    Pass --prepare to obtain the unsigned transaction, sign it in your
    wallet, then broadcast the result with 'submit'.

      rail0 payments authorize <rail0Id> --prepare
      # sign unsignedTransaction in MetaMask / cast / ethers.js
      rail0 payments submit <rail0Id> --signed-tx 0x02f8…

  --prepare and --private-key are mutually exclusive.

Authorize-and-capture flow:
  1. payments create -F 0xBuyer… -p 0xPayerKey…  — payer creates + signs
  2. payments authorize <rail0Id> -p 0xPayeeKey…  — payee authorizes
  3. payments capture   <rail0Id> -a <n> -p 0xPayeeKey…  — payee captures

Charge (one-shot) flow:
  1. payments create -m charge -F 0xBuyer… -p 0xPayerKey…
  2. payments charge <rail0Id> -p 0xPayeeKey…`,
}

// ── payments get ─────────────────────────────────────────────────────────────

var paymentsGetCmd = &cobra.Command{
	Use:   "get <rail0Id>",
	Short: "Fetch the current state of a payment",
	Long: `Fetch a payment's current status from the RAIL0 API together with live
on-chain balances (capturableAmount, refundableAmount) when the payment
is in an active on-chain state.

Status values:
  unsigned   — created, payer signature not yet submitted
  signed              — payer signature stored, awaiting payee action
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
			ctx := context.Background()
			client := newClient()
			id, err := resolvePaymentID(ctx, client, args[0])
			if err != nil {
				return nil, err
			}
			return client.Payments.Get(ctx, id)
		})
	},
}

// ── payments list ────────────────────────────────────────────────────────────

var paymentsListSort string

var paymentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List payments for the authenticated wallet (requires login)",
	Long: `Returns all payments where your wallet is payer or payee.
Requires authentication — run 'rail0 auth login -p 0xKey…' first.`,
	Example: `  rail0 payments list
  rail0 payments list --sort -created_at
  rail0 payments list --sort -amount,created_at`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			p := rail0.ListPaymentsParams{Sort: paymentsListSort}
			return newClient().Payments.List(context.Background(), p)
		})
	},
}

// ── payments transactions ─────────────────────────────────────────────────────

var paymentsTransactionsSort string

var paymentsTransactionsCmd = &cobra.Command{
	Use:   "transactions <rail0Id>",
	Short: "List on-chain transactions for a payment",
	Args:  cobra.ExactArgs(1),
	Example: `  rail0 payments transactions 0xabc123…
  rail0 payments transactions 0xabc123… --sort -submitted_at`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			ctx := context.Background()
			client := newClient()
			id, err := resolvePaymentID(ctx, client, args[0])
			if err != nil {
				return nil, err
			}
			p := rail0.ListTransactionsParams{Sort: paymentsTransactionsSort}
			return client.Payments.Transactions(ctx, id, p)
		})
	},
}

// ── payments sign ─────────────────────────────────────────────────────────────

var signPrivateKey string
var signSignature string
var signCreateFlags createPaymentFlags

var paymentsSignCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a payment and sign it with the payer's key (or submit an external signature)",
	Long: `Create a RAIL0 payment and sign the EIP-3009 payload with the payer's private
key in one step. The API stores the signature; the payee can then authorize.

  --private-key (-p)   Create the payment, sign the signingPayload locally,
                       and submit the signature in one step.

For external wallets (MetaMask): use 'payments get' to fetch the signingPayload,
sign it with eth_signTypedData_v4, then submit the result:

  rail0 payments create <rail0Id> --signature 0x1a2b…`,
	Example: `  # Payer with direct key access — create + sign in one step
  rail0 payments create \
    -F 0xBuyer… -T 0xMerchant… -t 0x3600… -a 1000000 -c 5042002 \
    -p 0xPayerKey…

  # Payer using MetaMask — submit externally-produced signature
  rail0 payments create <rail0Id> --signature 0x1a2b…(130 hex chars)`,
	Args: cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		client := newClient()

		if len(args) == 1 {
			// Submit-signature mode: payer signed externally (MetaMask).
			if signSignature == "" {
				return errors.New("provide --signature when passing a rail0Id")
			}
			return ui.RunAPI(func() (any, error) {
				return client.Payments.Sign(ctx, args[0],
					rail0.PayerSignatureRequest{Signature: signSignature})
			})
		}

		// Create + sign mode: payer has direct key access.
		if signPrivateKey == "" {
			return errors.New("provide --private-key (-p) to sign locally, or pass <rail0Id> --signature for an externally-produced signature")
		}
		req, err := signCreateFlags.build()
		if err != nil {
			return err
		}
		created, err := client.Payments.Create(ctx, req)
		if err != nil {
			return err
		}
		sig, err := signEIP712FromCreate(created, req.Mode, signPrivateKey)
		if err != nil {
			return fmt.Errorf("sign: %w", err)
		}
		return ui.RunAPI(func() (any, error) {
			return client.Payments.Sign(ctx, created.ID,
				rail0.PayerSignatureRequest{Signature: sig})
		})
	},
}


// ── action command helpers ────────────────────────────────────────────────────

// addSigningFlags attaches --prepare, --private-key / -p, and the hidden
// alias --pk to cmd, and marks --prepare and --private-key mutually exclusive.
func addSigningFlags(cmd *cobra.Command, privateKey *string, prepare *bool) {
	cmd.Flags().StringVarP(privateKey, "private-key", "p", "",
		"Caller's secp256k1 private key (0x-prefixed hex); signs and broadcasts in one step")
	cmd.Flags().StringVar(privateKey, "pk", "", "Alias for --private-key")
	_ = cmd.Flags().MarkHidden("pk")
	cmd.Flags().BoolVarP(prepare, "prepare", "w", false,
		"Return the unsigned transaction only, for external wallet signing")
	cmd.MarkFlagsMutuallyExclusive("private-key", "prepare")
	cmd.MarkFlagsMutuallyExclusive("pk", "prepare")
}

// requireSigningMode returns an error when neither --private-key / --pk nor
// --prepare was provided, giving the user a clear hint about the two options.
func requireSigningMode(privateKey string, prepare bool) error {
	if privateKey == "" && !prepare {
		return errors.New("provide --private-key (-p / --pk) for automated signing, or --prepare for external wallet signing")
	}
	return nil
}

// ── payments authorize ────────────────────────────────────────────────────────

var authorizePrivateKey string
var authorizePrepare bool

var paymentsAuthorizeCmd = &cobra.Command{
	Use:   "authorize <rail0Id>",
	Short: "Authorize a payment (prepare, sign, and broadcast)",
	Long: `Prepare the transaction that calls authorize() on the RAIL0 contract,
pulling funds from the payer into escrow.

The payer signature must have been submitted first via 'payments sign'.
The API returns 202 immediately. The payment stays at "signed" until
rail0-indexer confirms the on-chain event and the API transitions it to
"authorized". Poll 'payments get <rail0Id>' until payment=authorized
and the authorize transaction shows confirmed.

Signing modes (one required):

  --private-key (-p)   Sign locally and broadcast in one step.
  --prepare            Return the unsigned transaction for external signing,
                       then broadcast with 'payments submit'.`,
	Example: `  # 1. Payer creates the payment and receives signingPayload
  rail0 payments create --from 0xBuyer… --to 0xMerchant… \
    --token 0x3600… --amount 1000000 --chain-id 5042002

  # 2. Payer signs and submits the EIP-712 signature
  rail0 payments sign <rail0Id> --signature 0x1a2b…

  # 3. Payee authorizes — direct key
  rail0 payments authorize <rail0Id> -p 0xPayeeKey…

  # 3. Payee authorizes — external wallet
  rail0 payments authorize <rail0Id> --prepare
  rail0 payments submit    <rail0Id> --signed-tx 0x02f8…`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireSigningMode(authorizePrivateKey, authorizePrepare); err != nil {
			return err
		}
		client := newClient()
		ctx := context.Background()
		rail0Id, err := resolvePaymentID(ctx, client, args[0])
		if err != nil {
			return err
		}
		prep, err := client.Payments.AuthorizePrepare(ctx, rail0Id)
		if err != nil {
			return err
		}
		if authorizePrepare {
			return ui.RunAPI(func() (any, error) { return prep, nil })
		}
		signedTx, err := signTx(prep.UnsignedTransaction, authorizePrivateKey)
		if err != nil {
			return fmt.Errorf("sign: %w", err)
		}
		return ui.RunAPI(func() (any, error) {
			return client.Payments.Authorize(ctx, rail0Id,
				rail0.SubmitTransactionRequest{SignedTransaction: signedTx})
		})
	},
}

// ── payments charge ───────────────────────────────────────────────────────────

var chargePrivateKey string
var chargePrepare bool

var paymentsChargeCmd = &cobra.Command{
	Use:   "charge <rail0Id>",
	Short: "Charge a payment — authorize + capture in one call (no escrow)",
	Long: `Prepare the transaction that calls charge() on the RAIL0 contract.
Combines authorize + capture into a single on-chain call; funds move
directly from payer to payee without an escrow window.

The payment must have been created with --mode charge and the payer
signature submitted via 'payments sign'.
The API returns 202 immediately. The payment stays at "signed" until
rail0-indexer confirms and the API transitions it to "charged".
Poll 'payments get <rail0Id>' until payment=charged and the charge
transaction shows confirmed.

Signing modes (one required):

  --private-key (-p)   Sign locally and broadcast in one step.
  --prepare            Return the unsigned transaction for external signing,
                       then broadcast with 'payments submit'.`,
	Example: `  # Direct key
  rail0 payments charge 0xrail0Id… -p 0xPayeeKey…

  # External wallet
  rail0 payments charge 0xrail0Id… --prepare
  rail0 payments submit 0xrail0Id… --signed-tx 0x02f8…`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireSigningMode(chargePrivateKey, chargePrepare); err != nil {
			return err
		}
		client := newClient()
		ctx := context.Background()
		rail0Id, err := resolvePaymentID(ctx, client, args[0])
		if err != nil {
			return err
		}
		prep, err := client.Payments.ChargePrepare(ctx, rail0Id)
		if err != nil {
			return err
		}
		if chargePrepare {
			return ui.RunAPI(func() (any, error) { return prep, nil })
		}
		signedTx, err := signTx(prep.UnsignedTransaction, chargePrivateKey)
		if err != nil {
			return fmt.Errorf("sign: %w", err)
		}
		return ui.RunAPI(func() (any, error) {
			return client.Payments.Charge(ctx, rail0Id,
				rail0.SubmitTransactionRequest{SignedTransaction: signedTx})
		})
	},
}

// ── payments capture ──────────────────────────────────────────────────────────

var captureAmount string
var capturePrivateKey string
var capturePrepare bool

var paymentsCaptureCmd = &cobra.Command{
	Use:   "capture <rail0Id>",
	Short: "Capture a payment (prepare, sign, and broadcast)",
	Long: `Prepare the transaction that calls capture() on the RAIL0 contract.

The payment must be in 'authorized' or 'partially_captured' status.
Partial captures are supported: pass --amount less than capturableAmount
to capture a portion; the remaining balance stays available until
authorizationExpiry.
The API returns 202 immediately. The payment stays at "authorized" until
rail0-indexer confirms and the API transitions it to "captured" or
"partially_captured". Poll 'payments get <rail0Id>' until the payment
reaches the expected state and the capture transaction shows confirmed.

Signing modes (one required):

  --private-key (-p)   Sign locally and broadcast in one step.
  --prepare            Return the unsigned transaction for external signing,
                       then broadcast with 'payments submit'.`,
	Example: `  # Direct key — full capture
  rail0 payments capture 0xrail0Id… --amount 100000000 -p 0xPayeeKey…

  # External wallet — partial capture
  rail0 payments capture 0xrail0Id… --amount 50000000 --prepare
  rail0 payments submit  0xrail0Id… --signed-tx 0x02f8…`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireSigningMode(capturePrivateKey, capturePrepare); err != nil {
			return err
		}
		client := newClient()
		ctx := context.Background()
		rail0Id, err := resolvePaymentID(ctx, client, args[0])
		if err != nil {
			return err
		}
		prep, err := client.Payments.CapturePrepare(ctx, rail0Id, captureAmount)
		if err != nil {
			return err
		}
		if capturePrepare {
			return ui.RunAPI(func() (any, error) { return prep, nil })
		}
		signedTx, err := signTx(prep.UnsignedTransaction, capturePrivateKey)
		if err != nil {
			return fmt.Errorf("sign: %w", err)
		}
		return ui.RunAPI(func() (any, error) {
			return client.Payments.Capture(ctx, rail0Id,
				rail0.SubmitTransactionRequest{SignedTransaction: signedTx})
		})
	},
}

// ── payments void ─────────────────────────────────────────────────────────────

var voidPrivateKey string
var voidPrepare bool

var paymentsVoidCmd = &cobra.Command{
	Use:   "void <rail0Id>",
	Short: "Void a payment (prepare, sign, and broadcast)",
	Long: `Prepare the transaction that calls void() on the RAIL0 contract.

The payment must be in 'authorized' status. Void cancels the authorization
and returns the full escrowed balance to the payer. Only callable before
authorizationExpiry, by the payee.
The API returns 202 immediately; poll 'payments get <rail0Id>' until
status advances to "voided".

Signing modes (one required):

  --private-key (-p)   Sign locally and broadcast in one step.
  --prepare            Return the unsigned transaction for external signing,
                       then broadcast with 'payments submit'.`,
	Example: `  # Direct key
  rail0 payments void 0xrail0Id… -p 0xPayeeKey…

  # External wallet
  rail0 payments void   0xrail0Id… --prepare
  rail0 payments submit 0xrail0Id… --signed-tx 0x02f8…`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireSigningMode(voidPrivateKey, voidPrepare); err != nil {
			return err
		}
		client := newClient()
		ctx := context.Background()
		rail0Id, err := resolvePaymentID(ctx, client, args[0])
		if err != nil {
			return err
		}
		prep, err := client.Payments.VoidPrepare(ctx, rail0Id)
		if err != nil {
			return err
		}
		if voidPrepare {
			return ui.RunAPI(func() (any, error) { return prep, nil })
		}
		signedTx, err := signTx(prep.UnsignedTransaction, voidPrivateKey)
		if err != nil {
			return fmt.Errorf("sign: %w", err)
		}
		return ui.RunAPI(func() (any, error) {
			return client.Payments.Void(ctx, rail0Id,
				rail0.SubmitTransactionRequest{SignedTransaction: signedTx})
		})
	},
}

// ── payments release ──────────────────────────────────────────────────────────

var releaseCallerAddress string
var releasePrivateKey string
var releasePrepare bool

var paymentsReleaseCmd = &cobra.Command{
	Use:   "release <rail0Id>",
	Short: "Release a payment after expiry (prepare, sign, and broadcast)",
	Long: `Prepare the transaction that calls release() on the RAIL0 contract.

Release is only available after authorizationExpiry has passed on-chain.
Returns the remaining escrowed balance to the payer. Permissionless —
both payer and payee can call it.

Use --caller-address to build the transaction for the payer. Omit it to
build for the payee (default).
The API returns 202 immediately; poll 'payments get <rail0Id>' until
status advances to "released".

Signing modes (one required):

  --private-key (-p)   Sign locally and broadcast in one step.
  --prepare            Return the unsigned transaction for external signing,
                       then broadcast with 'payments submit'.`,
	Example: `  # Payee releases — direct key
  rail0 payments release 0xrail0Id… -p 0xPayeeKey…

  # Payer releases — direct key
  rail0 payments release 0xrail0Id… --caller-address 0xBuyer… -p 0xBuyerKey…

  # External wallet
  rail0 payments release 0xrail0Id… --prepare
  rail0 payments submit  0xrail0Id… --signed-tx 0x02f8…`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireSigningMode(releasePrivateKey, releasePrepare); err != nil {
			return err
		}
		client := newClient()
		ctx := context.Background()
		rail0Id, err := resolvePaymentID(ctx, client, args[0])
		if err != nil {
			return err
		}
		prep, err := client.Payments.ReleasePrepare(ctx, rail0Id, releaseCallerAddress)
		if err != nil {
			return err
		}
		if releasePrepare {
			return ui.RunAPI(func() (any, error) { return prep, nil })
		}
		signedTx, err := signTx(prep.UnsignedTransaction, releasePrivateKey)
		if err != nil {
			return fmt.Errorf("sign: %w", err)
		}
		return ui.RunAPI(func() (any, error) {
			return client.Payments.Release(ctx, rail0Id,
				rail0.SubmitTransactionRequest{SignedTransaction: signedTx})
		})
	},
}

// ── payments approve ──────────────────────────────────────────────────────────


// ── payments refund ───────────────────────────────────────────────────────────

var refundAmount string
var refundPrivateKey string
var refundPrepare bool

var paymentsRefundCmd = &cobra.Command{
	Use:   "refund <rail0Id>",
	Short: "Refund a payment (prepare, sign, and broadcast)",
	Long: `Prepare the transaction that calls refund() on the RAIL0 contract,
pulling refundAmount from the payee and returning it to the payer.

The payment must be in 'captured', 'partially_captured', 'charged',
'refunded', or 'partially_refunded' status with a positive
refundableAmount. Partial refunds are supported: pass --amount less
than refundableAmount. Requires prior 'payments approve' if the payee
has insufficient allowance.
The API returns 202 immediately; poll 'payments get <rail0Id>' until
status advances to "refunded" or "partially_refunded".

Signing modes (one required):

  --private-key (-p)   Sign locally and broadcast in one step.
  --prepare            Return the unsigned transaction for external signing,
                       then broadcast with 'payments submit'.`,
	Example: `  # Direct key — full refund
  rail0 payments refund 0xrail0Id… --amount 100000000 -p 0xPayeeKey…

  # External wallet — partial refund
  rail0 payments refund  0xrail0Id… --amount 25000000 --prepare
  rail0 payments submit  0xrail0Id… --signed-tx 0x02f8…`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireSigningMode(refundPrivateKey, refundPrepare); err != nil {
			return err
		}
		client := newClient()
		ctx := context.Background()
		rail0Id, err := resolvePaymentID(ctx, client, args[0])
		if err != nil {
			return err
		}
		// Phase 1: get the EIP-712 ReceiveWithAuthorization signing payload.
		// Omitting the signature returns a Transaction whose SigningPayload is set.
		phase1, err := client.Payments.RefundPrepare(ctx, rail0Id, rail0.PrepareRequest{Amount: refundAmount})
		if err != nil {
			return err
		}
		if refundPrepare {
			return ui.RunAPI(func() (any, error) { return phase1, nil })
		}
		if phase1.SigningPayload == nil {
			return errors.New("refund prepare did not return a signing_payload")
		}

		// Sign the EIP-3009 ReceiveWithAuthorization payload (payee key).
		v, r, s, err := signRefundPayload(phase1.SigningPayload, refundPrivateKey)
		if err != nil {
			return fmt.Errorf("sign refund payload: %w", err)
		}
		// Combine V/R/S into a single 65-byte 0x-prefixed signature for phase 2.
		signature := "0x" + r[2:] + s[2:] + fmt.Sprintf("%02x", v)

		// Phase 2: submit the signature → get the unsigned transaction with the
		// EIP-3009 authorization embedded.
		phase2, err := client.Payments.RefundPrepare(ctx, rail0Id, rail0.PrepareRequest{
			Amount:    refundAmount,
			Signature: signature,
		})
		if err != nil {
			return fmt.Errorf("refund prepare phase 2: %w", err)
		}

		// Sign the unsigned EIP-1559 transaction and broadcast.
		signedTx, err := signTx(phase2.UnsignedTransaction, refundPrivateKey)
		if err != nil {
			return fmt.Errorf("sign tx: %w", err)
		}
		return ui.RunAPI(func() (any, error) {
			return client.Payments.Refund(ctx, rail0Id,
				rail0.SubmitTransactionRequest{SignedTransaction: signedTx})
		})
	},
}

// ── payments dispute ───────────────────────────────────────────────────────

var disputeReason string
var closeDisputeReason string

var paymentsDisputeCmd = &cobra.Command{
	Use:   "dispute <rail0Id>",
	Short: "Prepare a dispute transaction (payer-submitted, signal-only)",
	Long: `Prepare the transaction that calls dispute() on the RAIL0 contract.

Disputes are buyer-driven and signal-only: they move no money. Only the payer
may dispute, while the refund window is open and the merchant holds captured
funds. The returned unsigned transaction is for the payer to sign and broadcast
directly — the gateway records the dispute when it sees the on-chain event.`,
	Example: `  rail0 payments dispute 0xrail0Id… --reason 0x…`,
	Args:    cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		client := newClient()
		return ui.RunAPI(func() (any, error) {
			if disputeReason != "" {
				return client.Payments.DisputePrepare(context.Background(), args[0], disputeReason)
			}
			return client.Payments.DisputePrepare(context.Background(), args[0])
		})
	},
}

var paymentsDisputeCloseCmd = &cobra.Command{
	Use:   "close <rail0Id>",
	Short: "Prepare a close-dispute transaction (payer only)",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		client := newClient()
		return ui.RunAPI(func() (any, error) {
			if closeDisputeReason != "" {
				return client.Payments.CloseDisputePrepare(context.Background(), args[0], closeDisputeReason)
			}
			return client.Payments.CloseDisputePrepare(context.Background(), args[0])
		})
	},
}

var paymentsDisputesCmd = &cobra.Command{
	Use:   "disputes <rail0Id>",
	Short: "List a payment's dispute open/close history",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		client := newClient()
		return ui.RunAPI(func() (any, error) {
			return client.Payments.Disputes(context.Background(), args[0])
		})
	},
}

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	// payments create — creates the payment and signs it with the payer's key
	signCreateFlags.addTo(paymentsSignCmd)
	paymentsSignCmd.Flags().StringVarP(&signPrivateKey, "private-key", "p", "",
		"Payer's secp256k1 private key (0x-prefixed hex); creates the payment and signs locally")
	paymentsSignCmd.Flags().StringVar(&signPrivateKey, "pk", "", "Alias for --private-key")
	_ = paymentsSignCmd.Flags().MarkHidden("pk")
	paymentsSignCmd.Flags().StringVarP(&signSignature, "signature", "s", "",
		"65-byte EIP-712 signature (0x-prefixed 130-char hex) — used with <rail0Id> for external signing")
	paymentsSignCmd.MarkFlagsMutuallyExclusive("private-key", "signature")
	paymentsSignCmd.MarkFlagsMutuallyExclusive("pk", "signature")
	paymentsCmd.AddCommand(paymentsSignCmd)

	// payments get
	paymentsCmd.AddCommand(paymentsGetCmd)

	// payments list
	paymentsListCmd.Flags().StringVar(&paymentsListSort, "sort", "", "Sort fields (e.g. -created_at,amount)")
	paymentsCmd.AddCommand(paymentsListCmd)

	// payments transactions
	paymentsTransactionsCmd.Flags().StringVar(&paymentsTransactionsSort, "sort", "", "Sort fields (e.g. -submitted_at,operation)")
	paymentsCmd.AddCommand(paymentsTransactionsCmd)

	// payments authorize
	addSigningFlags(paymentsAuthorizeCmd, &authorizePrivateKey, &authorizePrepare)
	paymentsCmd.AddCommand(paymentsAuthorizeCmd)

	// payments charge
	addSigningFlags(paymentsChargeCmd, &chargePrivateKey, &chargePrepare)
	paymentsCmd.AddCommand(paymentsChargeCmd)

	// payments capture
	paymentsCaptureCmd.Flags().StringVarP(&captureAmount, "amount", "a", "",
		"Amount to capture in token base units (> 0, ≤ capturableAmount)")
	_ = paymentsCaptureCmd.MarkFlagRequired("amount")
	addSigningFlags(paymentsCaptureCmd, &capturePrivateKey, &capturePrepare)
	paymentsCmd.AddCommand(paymentsCaptureCmd)

	// payments void
	addSigningFlags(paymentsVoidCmd, &voidPrivateKey, &voidPrepare)
	paymentsCmd.AddCommand(paymentsVoidCmd)

	// payments release
	paymentsReleaseCmd.Flags().StringVarP(&releaseCallerAddress, "caller-address", "C", "",
		"Address that will sign the tx (default: payee on record)")
	addSigningFlags(paymentsReleaseCmd, &releasePrivateKey, &releasePrepare)
	paymentsCmd.AddCommand(paymentsReleaseCmd)

	// payments refund
	paymentsRefundCmd.Flags().StringVarP(&refundAmount, "amount", "a", "",
		"Amount to refund in token base units (> 0, ≤ refundableAmount)")
	_ = paymentsRefundCmd.MarkFlagRequired("amount")
	addSigningFlags(paymentsRefundCmd, &refundPrivateKey, &refundPrepare)
	paymentsCmd.AddCommand(paymentsRefundCmd)

	// payments dispute / dispute close / disputes
	paymentsDisputeCmd.Flags().StringVar(&disputeReason, "reason", "", "bytes32 reason code (0x…); defaults to zero")
	paymentsDisputeCloseCmd.Flags().StringVar(&closeDisputeReason, "reason", "", "bytes32 reason code (0x…); defaults to zero")
	paymentsDisputeCmd.AddCommand(paymentsDisputeCloseCmd)
	paymentsCmd.AddCommand(paymentsDisputeCmd)
	paymentsCmd.AddCommand(paymentsDisputesCmd)
}
