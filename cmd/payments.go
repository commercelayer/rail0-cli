package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/rail0/cli/internal/ui"
	rail0 "github.com/rail0/go-sdk"
	"github.com/spf13/cobra"
)

var paymentsCmd = &cobra.Command{
	Use:   "payments",
	Short: "Payment lifecycle operations",
	Long: `Manage the full RAIL0 payment lifecycle.

Every on-chain operation is atomic: pass --private-key (-p) and the CLI prepares
the unsigned transaction, signs it locally, and broadcasts it in a single step.

  rail0 payments authorize <rail0Id> -p 0xYourPrivateKey…

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
	Long: `Fetch a payment's current status from the RAIL0 gateway together with live
on-chain balances (capturableAmount, refundableAmount) when the payment
is in an active on-chain state.

Status values:
  unsigned   — created, payer signature not yet submitted
  signed              — payer signature stored, awaiting payee action
  authorized          — funds held in escrow, capture available
  charged             — one-shot charge executed, refund available
  captured            — funds fully captured by payee
  partially_captured  — partial capture executed; remaining escrow still capturable
  voided              — authorization cancelled (only before any capture), funds returned to payer
  released            — full authorization released back to payer (total release, nothing captured)
  refunded            — payment fully settled by refund (escrow and refundable both drained)
  partially_refunded  — legacy; no longer produced (a partial refund leaves the status unchanged)
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

var (
	paymentsListSort        string
	paymentsListChainID     int
	paymentsListDisputed    bool
	paymentsListStatus      string
	paymentsListMode        string
	paymentsListPayer       string
	paymentsListPayee       string
	paymentsListToken       string
	paymentsListMinAmount   string
	paymentsListMaxAmount   string
	paymentsListCreatedFrom string
	paymentsListCreatedTo   string
)

var paymentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List payments for the authenticated wallet (requires login)",
	Long: `Returns all payments where your wallet is payer or payee.
Requires authentication — run 'rail0 auth login -p 0xKey…' first.

Filter with --status, --mode, --payer, --payee, --token, --chain-id, --disputed,
--min-amount / --max-amount (token base units), and --created-from /
--created-to (ISO-8601).`,
	Example: `  rail0 payments list
  rail0 payments list --sort -created_at
  rail0 payments list --status authorized --mode charge
  rail0 payments list --chain-id 5042002 --disputed
  rail0 payments list --token 0x3600… --min-amount 1000000 --created-from 2026-01-01T00:00:00Z`,
	RunE: func(cmd *cobra.Command, args []string) error {
		p := rail0.ListPaymentsParams{
			Sort:        paymentsListSort,
			Status:      paymentsListStatus,
			Mode:        paymentsListMode,
			Payer:       paymentsListPayer,
			Payee:       paymentsListPayee,
			Token:       paymentsListToken,
			ChainID:     paymentsListChainID,
			MinAmount:   paymentsListMinAmount,
			MaxAmount:   paymentsListMaxAmount,
			CreatedFrom: paymentsListCreatedFrom,
			CreatedTo:   paymentsListCreatedTo,
		}
		// Only send disputed when the flag was explicitly set (tri-state).
		if cmd.Flags().Changed("disputed") {
			p.Disputed = &paymentsListDisputed
		}
		return ui.RunAPI(func() (any, error) {
			resp, err := newClient().Payments.List(context.Background(), p)
			if err != nil {
				return nil, err
			}
			return resp.Data, nil // bare array; pagination is in headers
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
			resp, err := client.Payments.Transactions(ctx, id, p)
			if err != nil {
				return nil, err
			}
			return resp.Data, nil // bare array; pagination is in headers
		})
	},
}

// ── payments sign ─────────────────────────────────────────────────────────────

var signPrivateKey string
var signCreateFlags createPaymentFlags

var paymentsSignCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a payment and sign it with the payer's key",
	Long: `Create a RAIL0 payment and sign the EIP-3009 payload with the payer's key in
one step (--private-key / -p). The API stores the signature; the payee can then
authorize.`,
	Example: `  rail0 payments create \
    -F 0xBuyer… -T 0xMerchant… -t USDC -a 10.50 -c 5042002 \
    -p 0xPayerKey…`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		client := newClient()

		// Create + sign: payer signs the EIP-712 payload locally with the key
		// (--private-key, or RAIL0_PRIVATE_KEY).
		signingKey, err := resolveSigningKey(signPrivateKey)
		if err != nil {
			return errors.New("provide --private-key (-p) to create and sign the payment")
		}
		req, err := signCreateFlags.build(cmd.Flags().Changed("mode"))
		if err != nil {
			return err
		}
		// Preventive check: the payer signs with their own key. Compare the signing
		// key's address to the resolved payer up front, so signing with the wrong
		// key (e.g. the payee's) fails immediately with a clear message instead of a
		// gateway signer_mismatch after the create round-trip.
		signer, err := addressFromKey(signingKey)
		if err != nil {
			return err
		}
		if !strings.EqualFold(signer, string(req.Payer)) {
			return fmt.Errorf("signing key address %s does not match the payer %s (--from) — you're signing with the wrong key", signer, req.Payer)
		}
		return ui.RunAPIWithMessage("Creating payment…", func() (any, error) {
			created, err := client.Payments.Create(ctx, req)
			if err != nil {
				return nil, err
			}
			sig, err := signEIP712FromCreate(created, req.Mode, signingKey)
			if err != nil {
				return nil, fmt.Errorf("sign: %w", err)
			}
			return client.Payments.Sign(ctx, created.ID,
				rail0.PayerSignatureRequest{Signature: sig})
		})
	},
}

// ── action command helpers ────────────────────────────────────────────────────

// addSigningFlags attaches the signing-key flags to cmd: --private-key / -p and
// the hidden alias --pk. Every on-chain operation is atomic: the CLI prepares
// the unsigned transaction, signs it locally with this key, and broadcasts it in
// a single call.
func addSigningFlags(cmd *cobra.Command, privateKey *string) {
	cmd.Flags().StringVarP(privateKey, "private-key", "p", "",
		"Caller's secp256k1 private key (0x-prefixed hex); signs and broadcasts in one step")
	cmd.Flags().StringVar(privateKey, "pk", "", "Alias for --private-key")
	_ = cmd.Flags().MarkHidden("pk")
}

// resolveSigningKey determines the private key to sign with.
// Precedence: --private-key / --pk (explicit) > RAIL0_PRIVATE_KEY env var.
// When the explicit value starts with "@", it names a key stored in the OS
// keychain ('rail0 keys add') and is resolved to the underlying hex via loadKey.
// Returns a clear error when none is available.
func resolveSigningKey(explicitKey string) (string, error) {
	if explicitKey != "" {
		// "@name" resolves to a key saved in the OS keychain ('rail0 keys add').
		if strings.HasPrefix(explicitKey, "@") {
			return loadKey(strings.TrimPrefix(explicitKey, "@"))
		}
		return explicitKey, nil
	}
	if env := os.Getenv("RAIL0_PRIVATE_KEY"); env != "" {
		return env, nil
	}
	return "", errors.New("no signing key: pass --private-key (-p / --pk), set RAIL0_PRIVATE_KEY, or save one with 'rail0 keys add' and pass it as @name")
}

// ── payments authorize ────────────────────────────────────────────────────────

var authorizePrivateKey string

var paymentsAuthorizeCmd = &cobra.Command{
	Use:   "authorize <rail0Id>",
	Short: "Authorize a payment (prepare, sign, and broadcast)",
	Long: `Prepare the transaction that calls authorize() on the RAIL0 contract,
pulling funds from the payer into escrow.

The payer must have created and signed the payment first via
'payments create -p' (create + sign in one atomic step).
The API returns 202 immediately. The payment stays at "signed" until
rail0-indexer confirms the on-chain event and the API transitions it to
"authorized". Poll 'payments get <rail0Id>' until payment=authorized
and the authorize transaction shows confirmed.

Pass --private-key (-p): the CLI signs locally and broadcasts in one step.`,
	Example: `  # 1. Payer creates and signs the payment in one step
  rail0 payments create --from 0xBuyer… --to 0xMerchant… \
    --token 0x3600… --amount 1.00 --chain-id 5042002 -p 0xPayerKey…

  # 2. Payee authorizes
  rail0 payments authorize <rail0Id> -p 0xPayeeKey…`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		signingKey, err := resolveSigningKey(authorizePrivateKey)
		if err != nil {
			return err
		}
		client := newClient()
		ctx := context.Background()
		rail0Id, err := resolvePaymentID(ctx, client, args[0])
		if err != nil {
			return err
		}
		return ui.RunAPIWithMessage("Authorizing…", func() (any, error) {
			prep, err := client.Payments.AuthorizePrepare(ctx, rail0Id)
			if err != nil {
				return nil, err
			}
			signedTx, err := signTx(prep.UnsignedTransaction, signingKey)
			if err != nil {
				return nil, fmt.Errorf("sign: %w", err)
			}
			return client.Payments.Authorize(ctx, rail0Id,
				rail0.SubmitTransactionRequest{SignedTransaction: signedTx})
		})
	},
}

// ── payments charge ───────────────────────────────────────────────────────────

var chargePrivateKey string

var paymentsChargeCmd = &cobra.Command{
	Use:   "charge <rail0Id>",
	Short: "Charge a payment — authorize + capture in one call (no escrow)",
	Long: `Prepare the transaction that calls charge() on the RAIL0 contract.
Combines authorize + capture into a single on-chain call; funds move
directly from payer to payee without an escrow window.

The payment must have been created with --mode charge and signed by the
payer via 'payments create -p' (create + sign in one atomic step).
The API returns 202 immediately. The payment stays at "signed" until
rail0-indexer confirms and the API transitions it to "charged".
Poll 'payments get <rail0Id>' until payment=charged and the charge
transaction shows confirmed.

Pass --private-key (-p): the CLI signs locally and broadcasts in one step.`,
	Example: `  rail0 payments charge 0xrail0Id… -p 0xPayeeKey…`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		signingKey, err := resolveSigningKey(chargePrivateKey)
		if err != nil {
			return err
		}
		client := newClient()
		ctx := context.Background()
		rail0Id, err := resolvePaymentID(ctx, client, args[0])
		if err != nil {
			return err
		}
		return ui.RunAPIWithMessage("Charging…", func() (any, error) {
			prep, err := client.Payments.ChargePrepare(ctx, rail0Id)
			if err != nil {
				return nil, err
			}
			signedTx, err := signTx(prep.UnsignedTransaction, signingKey)
			if err != nil {
				return nil, fmt.Errorf("sign: %w", err)
			}
			return client.Payments.Charge(ctx, rail0Id,
				rail0.SubmitTransactionRequest{SignedTransaction: signedTx})
		})
	},
}

// ── payments capture ──────────────────────────────────────────────────────────

var captureAmount string
var capturePrivateKey string

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

Pass --private-key (-p): the CLI signs locally and broadcasts in one step.`,
	Example: `  # Full capture
  rail0 payments capture 0xrail0Id… --amount 1.00 -p 0xPayeeKey…

  # Partial capture
  rail0 payments capture 0xrail0Id… --amount 0.50 -p 0xPayeeKey…`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		signingKey, err := resolveSigningKey(capturePrivateKey)
		if err != nil {
			return err
		}
		client := newClient()
		ctx := context.Background()
		rail0Id, err := resolvePaymentID(ctx, client, args[0])
		if err != nil {
			return err
		}
		return ui.RunAPIWithMessage("Capturing…", func() (any, error) {
			prep, err := client.Payments.CapturePrepare(ctx, rail0Id, captureAmount)
			if err != nil {
				return nil, err
			}
			signedTx, err := signTx(prep.UnsignedTransaction, signingKey)
			if err != nil {
				return nil, fmt.Errorf("sign: %w", err)
			}
			return client.Payments.Capture(ctx, rail0Id,
				rail0.SubmitTransactionRequest{SignedTransaction: signedTx})
		})
	},
}

// ── payments void ─────────────────────────────────────────────────────────────

var voidPrivateKey string

var paymentsVoidCmd = &cobra.Command{
	Use:   "void <rail0Id>",
	Short: "Void a payment (prepare, sign, and broadcast)",
	Long: `Prepare the transaction that calls void() on the RAIL0 contract.

The payment must be in 'authorized' status with NOTHING captured yet: void is
forbidden once any (even partial) capture has occurred (the contract reverts
AlreadyCaptured) — recover the uncaptured remainder via 'release' instead. Void
cancels the authorization and returns the full escrowed balance to the payer.
Only callable before authorizationExpiry, by the payee.
The API returns 202 immediately; poll 'payments get <rail0Id>' until
status advances to "voided".

Pass --private-key (-p): the CLI signs locally and broadcasts in one step.`,
	Example: `  rail0 payments void 0xrail0Id… -p 0xPayeeKey…`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		signingKey, err := resolveSigningKey(voidPrivateKey)
		if err != nil {
			return err
		}
		client := newClient()
		ctx := context.Background()
		rail0Id, err := resolvePaymentID(ctx, client, args[0])
		if err != nil {
			return err
		}
		return ui.RunAPIWithMessage("Voiding…", func() (any, error) {
			prep, err := client.Payments.VoidPrepare(ctx, rail0Id)
			if err != nil {
				return nil, err
			}
			signedTx, err := signTx(prep.UnsignedTransaction, signingKey)
			if err != nil {
				return nil, fmt.Errorf("sign: %w", err)
			}
			return client.Payments.Void(ctx, rail0Id,
				rail0.SubmitTransactionRequest{SignedTransaction: signedTx})
		})
	},
}

// ── payments release ──────────────────────────────────────────────────────────

var releasePrivateKey string

var paymentsReleaseCmd = &cobra.Command{
	Use:   "release <rail0Id>",
	Short: "Release a payment after expiry (prepare, sign, and broadcast)",
	Long: `Prepare the transaction that calls release() on the RAIL0 contract.

Release is only available after authorizationExpiry has passed on-chain.
Returns the remaining (uncaptured) escrowed balance to the payer. Permissionless
— both payer and payee can call it; the caller is whoever the signing key belongs
to (derived from the key), so pass the payer's key to release as the payer.
The API returns 202 immediately; poll 'payments get <rail0Id>'. The status
advances to "released" only on a TOTAL release (an untouched authorization,
nothing captured); if a capture left a refundable residual the status stays
unchanged and that residual remains refundable.

Pass --private-key (-p): the CLI signs locally and broadcasts in one step.`,
	Example: `  # Payee releases
  rail0 payments release 0xrail0Id… -p 0xPayeeKey…

  # Payer releases (just use the payer's key)
  rail0 payments release 0xrail0Id… -p 0xPayerKey…`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		signingKey, err := resolveSigningKey(releasePrivateKey)
		if err != nil {
			return err
		}
		// The on-chain caller is whoever signs, so derive it from the signing key
		// rather than taking a separate flag.
		caller, err := addressFromKey(signingKey)
		if err != nil {
			return err
		}
		client := newClient()
		ctx := context.Background()
		rail0Id, err := resolvePaymentID(ctx, client, args[0])
		if err != nil {
			return err
		}
		return ui.RunAPIWithMessage("Releasing…", func() (any, error) {
			prep, err := client.Payments.ReleasePrepare(ctx, rail0Id, caller)
			if err != nil {
				return nil, err
			}
			signedTx, err := signTx(prep.UnsignedTransaction, signingKey)
			if err != nil {
				return nil, fmt.Errorf("sign: %w", err)
			}
			return client.Payments.Release(ctx, rail0Id,
				rail0.SubmitTransactionRequest{SignedTransaction: signedTx})
		})
	},
}

// ── payments refund ───────────────────────────────────────────────────────────

var refundAmount string
var refundPrivateKey string

var paymentsRefundCmd = &cobra.Command{
	Use:   "refund <rail0Id>",
	Short: "Refund a payment (prepare, sign, and broadcast)",
	Long: `Prepare the transaction that calls refund() on the RAIL0 contract,
pulling refundAmount from the payee and returning it to the payer.

The payment must be in 'charged', 'captured', or 'partially_captured' status
with a positive refundableAmount. Partial refunds are supported: pass --amount
less than refundableAmount.

No separate approval step is needed: the CLI signs the payee's EIP-3009
ReceiveWithAuthorization payload locally with the same key, so the refund
funds are pulled from the payee without a prior allowance grant.
The API returns 202 immediately; poll 'payments get <rail0Id>'. The status
advances to "refunded" only when the payment is fully settled (both escrow and
refundable drained); a partial refund leaves the status unchanged.

Pass --private-key (-p): the CLI signs locally and broadcasts in one step.`,
	Example: `  # Full refund
  rail0 payments refund 0xrail0Id… --amount 1.00 -p 0xPayeeKey…

  # Partial refund
  rail0 payments refund 0xrail0Id… --amount 0.25 -p 0xPayeeKey…`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		signingKey, err := resolveSigningKey(refundPrivateKey)
		if err != nil {
			return err
		}
		client := newClient()
		ctx := context.Background()
		rail0Id, err := resolvePaymentID(ctx, client, args[0])
		if err != nil {
			return err
		}
		return ui.RunAPIWithMessage("Refunding…", func() (any, error) {
			// Phase 1: get the EIP-712 ReceiveWithAuthorization signing payload.
			// Omitting the signature returns a Transaction whose SigningPayload is set.
			phase1, err := client.Payments.RefundPrepare(ctx, rail0Id, rail0.PrepareRequest{Amount: refundAmount})
			if err != nil {
				return nil, err
			}
			if phase1.SigningPayload == nil {
				return nil, errors.New("refund prepare did not return a signing_payload")
			}

			// Sign the EIP-3009 ReceiveWithAuthorization payload (payee key).
			v, r, s, err := signRefundPayload(phase1.SigningPayload, signingKey)
			if err != nil {
				return nil, fmt.Errorf("sign refund payload: %w", err)
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
				return nil, fmt.Errorf("refund prepare phase 2: %w", err)
			}

			// Sign the unsigned EIP-1559 transaction and broadcast.
			signedTx, err := signTx(phase2.UnsignedTransaction, signingKey)
			if err != nil {
				return nil, fmt.Errorf("sign tx: %w", err)
			}
			return client.Payments.Refund(ctx, rail0Id,
				rail0.SubmitTransactionRequest{SignedTransaction: signedTx})
		})
	},
}

// ── payments dispute ───────────────────────────────────────────────────────

var disputeReason string
var disputePrivateKey string
var closeDisputeReason string
var closeDisputePrivateKey string

var paymentsDisputeCmd = &cobra.Command{
	Use:   "dispute <rail0Id>",
	Short: "Open a dispute (payer-submitted, signal-only): prepare, sign, broadcast",
	Long: `Prepare the transaction that calls dispute() on the RAIL0 contract, then
sign and submit it like any other operation (the gateway broadcasts it).

Disputes are payer-driven and signal-only: they move no money. Only the payer
may dispute, while the refund window is open and the merchant holds the funds.

Pass --private-key (-p): the CLI signs locally with the payer key and broadcasts.`,
	Example: `  rail0 payments dispute 0xrail0Id… -p 0xPayerKey… --reason 0x…`,
	Args:    cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		signingKey, err := resolveSigningKey(disputePrivateKey)
		if err != nil {
			return err
		}
		client := newClient()
		ctx := context.Background()
		rail0Id, err := resolvePaymentID(ctx, client, args[0])
		if err != nil {
			return err
		}
		return ui.RunAPIWithMessage("Opening dispute…", func() (any, error) {
			prep, err := client.Payments.DisputePrepare(ctx, rail0Id, disputeReason)
			if err != nil {
				return nil, err
			}
			signedTx, err := signTx(prep.UnsignedTransaction, signingKey)
			if err != nil {
				return nil, fmt.Errorf("sign: %w", err)
			}
			return client.Payments.Dispute(ctx, rail0Id,
				rail0.SubmitTransactionRequest{SignedTransaction: signedTx})
		})
	},
}

var paymentsDisputeCloseCmd = &cobra.Command{
	Use:     "close <rail0Id>",
	Short:   "Close a dispute (payer only): prepare, sign, broadcast",
	Args:    cobra.ExactArgs(1),
	Example: `  rail0 payments dispute close 0xrail0Id… -p 0xPayerKey…`,
	RunE: func(_ *cobra.Command, args []string) error {
		signingKey, err := resolveSigningKey(closeDisputePrivateKey)
		if err != nil {
			return err
		}
		client := newClient()
		ctx := context.Background()
		rail0Id, err := resolvePaymentID(ctx, client, args[0])
		if err != nil {
			return err
		}
		return ui.RunAPIWithMessage("Closing dispute…", func() (any, error) {
			prep, err := client.Payments.CloseDisputePrepare(ctx, rail0Id, closeDisputeReason)
			if err != nil {
				return nil, err
			}
			signedTx, err := signTx(prep.UnsignedTransaction, signingKey)
			if err != nil {
				return nil, fmt.Errorf("sign: %w", err)
			}
			return client.Payments.CloseDispute(ctx, rail0Id,
				rail0.SubmitTransactionRequest{SignedTransaction: signedTx})
		})
	},
}

var (
	disputesStatus string
	disputesSort   string
)

var paymentsDisputesCmd = &cobra.Command{
	Use:   "disputes <rail0Id>",
	Short: "List a payment's dispute open/close history",
	Args:  cobra.ExactArgs(1),
	Example: `  rail0 payments disputes 0xrail0Id…
  rail0 payments disputes 0xrail0Id… --status open`,
	RunE: func(_ *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			ctx := context.Background()
			client := newClient()
			id, err := resolvePaymentID(ctx, client, args[0])
			if err != nil {
				return nil, err
			}
			resp, err := client.Payments.Disputes(ctx, id,
				rail0.ListDisputesParams{Status: disputesStatus, Sort: disputesSort})
			if err != nil {
				return nil, err
			}
			return resp.Data, nil // bare array; pagination is in headers
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
	paymentsCmd.AddCommand(paymentsSignCmd)

	// payments get
	paymentsCmd.AddCommand(paymentsGetCmd)

	// payments list
	paymentsListCmd.Flags().StringVar(&paymentsListSort, "sort", "", "Sort fields (e.g. -created_at,amount)")
	paymentsListCmd.Flags().StringVar(&paymentsListStatus, "status", "", "Filter by payment status (e.g. authorized, captured, refunded)")
	paymentsListCmd.Flags().StringVar(&paymentsListMode, "mode", "", "Filter by mode (authorize or charge)")
	paymentsListCmd.Flags().StringVar(&paymentsListPayer, "payer", "", "Filter by payer address")
	paymentsListCmd.Flags().StringVar(&paymentsListPayee, "payee", "", "Filter by payee address")
	paymentsListCmd.Flags().StringVar(&paymentsListToken, "token", "", "Filter by token address")
	paymentsListCmd.Flags().IntVar(&paymentsListChainID, "chain-id", 0, "Filter by EVM chain ID")
	paymentsListCmd.Flags().BoolVar(&paymentsListDisputed, "disputed", false, "Filter by whether an open dispute exists")
	paymentsListCmd.Flags().StringVar(&paymentsListMinAmount, "min-amount", "", "Minimum amount in token base units (inclusive)")
	paymentsListCmd.Flags().StringVar(&paymentsListMaxAmount, "max-amount", "", "Maximum amount in token base units (inclusive)")
	paymentsListCmd.Flags().StringVar(&paymentsListCreatedFrom, "created-from", "", "Only payments created at/after this ISO-8601 time")
	paymentsListCmd.Flags().StringVar(&paymentsListCreatedTo, "created-to", "", "Only payments created at/before this ISO-8601 time")
	paymentsCmd.AddCommand(paymentsListCmd)

	// payments transactions
	paymentsTransactionsCmd.Flags().StringVar(&paymentsTransactionsSort, "sort", "", "Sort fields (e.g. -submitted_at,operation)")
	paymentsCmd.AddCommand(paymentsTransactionsCmd)

	// payments authorize
	addSigningFlags(paymentsAuthorizeCmd, &authorizePrivateKey)
	paymentsCmd.AddCommand(paymentsAuthorizeCmd)

	// payments charge
	addSigningFlags(paymentsChargeCmd, &chargePrivateKey)
	paymentsCmd.AddCommand(paymentsChargeCmd)

	// payments capture
	paymentsCaptureCmd.Flags().StringVarP(&captureAmount, "amount", "a", "",
		"Amount to capture as a decimal in token units (e.g. 1.00); > 0 and ≤ capturable")
	_ = paymentsCaptureCmd.MarkFlagRequired("amount")
	addSigningFlags(paymentsCaptureCmd, &capturePrivateKey)
	paymentsCmd.AddCommand(paymentsCaptureCmd)

	// payments void
	addSigningFlags(paymentsVoidCmd, &voidPrivateKey)
	paymentsCmd.AddCommand(paymentsVoidCmd)

	// payments release
	addSigningFlags(paymentsReleaseCmd, &releasePrivateKey)
	paymentsCmd.AddCommand(paymentsReleaseCmd)

	// payments refund
	paymentsRefundCmd.Flags().StringVarP(&refundAmount, "amount", "a", "",
		"Amount to refund as a decimal in token units (e.g. 1.00); > 0 and ≤ refundable")
	_ = paymentsRefundCmd.MarkFlagRequired("amount")
	addSigningFlags(paymentsRefundCmd, &refundPrivateKey)
	paymentsCmd.AddCommand(paymentsRefundCmd)

	// payments dispute / dispute close / disputes
	addSigningFlags(paymentsDisputeCmd, &disputePrivateKey)
	paymentsDisputeCmd.Flags().StringVar(&disputeReason, "reason", "", "bytes32 reason code (0x…); defaults to zero")
	addSigningFlags(paymentsDisputeCloseCmd, &closeDisputePrivateKey)
	paymentsDisputeCloseCmd.Flags().StringVar(&closeDisputeReason, "reason", "", "bytes32 reason code (0x…); defaults to zero")
	paymentsDisputeCmd.AddCommand(paymentsDisputeCloseCmd)
	paymentsCmd.AddCommand(paymentsDisputeCmd)
	paymentsDisputesCmd.Flags().StringVar(&disputesStatus, "status", "", "Filter by dispute status: open or closed")
	paymentsDisputesCmd.Flags().StringVar(&disputesSort, "sort", "", "Sort fields (e.g. -opened_at)")
	paymentsCmd.AddCommand(paymentsDisputesCmd)
}
