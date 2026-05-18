package cmd

import (
	"context"

	rail0 "github.com/rail0/go-sdk"
	"github.com/rail0/cli/internal/output"
	"github.com/spf13/cobra"
)

var paymentsCmd = &cobra.Command{
	Use:   "payments",
	Short: "Payment lifecycle operations",
}

// ── payments get ─────────────────────────────────────────────────────────────

var paymentsGetCmd = &cobra.Command{
	Use:   "get <paymentId>",
	Short: "Get on-chain state for a payment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := newClient().Payments.Get(context.Background(), args[0])
		if err != nil {
			return err
		}
		return output.Print(resp)
	},
}

// ── payments authorize ────────────────────────────────────────────────────────

var (
	authPayment paymentFlags
	authAmount  string
	authV       int
	authR       string
	authS       string
)

var paymentsAuthorizeCmd = &cobra.Command{
	Use:   "authorize <paymentId>",
	Short: "Authorize a payment (pull funds into escrow via EIP-3009)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := authPayment.build()
		if err != nil {
			return err
		}
		resp, err := newClient().Payments.Authorize(context.Background(), args[0], rail0.AuthorizeParams{
			Payment: p,
			Amount:  authAmount,
			V:       authV,
			R:       authR,
			S:       authS,
		})
		if err != nil {
			return err
		}
		return output.Print(resp)
	},
}

// ── payments charge ───────────────────────────────────────────────────────────

var (
	chargePayment paymentFlags
	chargeAmount  string
	chargeV       int
	chargeR       string
	chargeS       string
)

var paymentsChargeCmd = &cobra.Command{
	Use:   "charge <paymentId>",
	Short: "Authorize and immediately capture in one transaction",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := chargePayment.build()
		if err != nil {
			return err
		}
		resp, err := newClient().Payments.Charge(context.Background(), args[0], rail0.ChargeParams{
			Payment: p,
			Amount:  chargeAmount,
			V:       chargeV,
			R:       chargeR,
			S:       chargeS,
		})
		if err != nil {
			return err
		}
		return output.Print(resp)
	},
}

// ── payments capture ──────────────────────────────────────────────────────────

var (
	capturePayment paymentFlags
	captureAmount  string
)

var paymentsCaptureCmd = &cobra.Command{
	Use:   "capture <paymentId>",
	Short: "Move escrowed funds to the payee",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := capturePayment.build()
		if err != nil {
			return err
		}
		resp, err := newClient().Payments.Capture(context.Background(), args[0], rail0.CaptureParams{
			Payment: p,
			Amount:  captureAmount,
		})
		if err != nil {
			return err
		}
		return output.Print(resp)
	},
}

// ── payments void ─────────────────────────────────────────────────────────────

var voidPayment paymentFlags

var paymentsVoidCmd = &cobra.Command{
	Use:   "void <paymentId>",
	Short: "Cancel an authorization and return escrowed funds to the payer",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := voidPayment.build()
		if err != nil {
			return err
		}
		resp, err := newClient().Payments.Void(context.Background(), args[0], rail0.VoidParams{Payment: p})
		if err != nil {
			return err
		}
		return output.Print(resp)
	},
}

// ── payments release ──────────────────────────────────────────────────────────

var releasePayment paymentFlags

var paymentsReleaseCmd = &cobra.Command{
	Use:   "release <paymentId>",
	Short: "Return escrowed funds to the payer after authorizationExpiry (permissionless)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := releasePayment.build()
		if err != nil {
			return err
		}
		resp, err := newClient().Payments.Release(context.Background(), args[0], rail0.ReleaseParams{Payment: p})
		if err != nil {
			return err
		}
		return output.Print(resp)
	},
}

// ── payments refund ───────────────────────────────────────────────────────────

var (
	refundPayment paymentFlags
	refundAmount  string
)

var paymentsRefundCmd = &cobra.Command{
	Use:   "refund <paymentId>",
	Short: "Return a captured amount from the payee to the payer",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := refundPayment.build()
		if err != nil {
			return err
		}
		resp, err := newClient().Payments.Refund(context.Background(), args[0], rail0.RefundParams{
			Payment: p,
			Amount:  refundAmount,
		})
		if err != nil {
			return err
		}
		return output.Print(resp)
	},
}

// ── payments authorize-nonce ──────────────────────────────────────────────────

var authorizeNonceConfigHash string

var paymentsAuthorizeNonceCmd = &cobra.Command{
	Use:   "authorize-nonce <paymentId>",
	Short: "Get the EIP-3009 nonce for an authorize call",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := newClient().Payments.AuthorizeNonce(context.Background(), args[0], authorizeNonceConfigHash)
		if err != nil {
			return err
		}
		return output.Print(resp)
	},
}

// ── payments charge-nonce ─────────────────────────────────────────────────────

var chargeNonceConfigHash string

var paymentsChargeNonceCmd = &cobra.Command{
	Use:   "charge-nonce <paymentId>",
	Short: "Get the EIP-3009 nonce for a charge call",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := newClient().Payments.ChargeNonce(context.Background(), args[0], chargeNonceConfigHash)
		if err != nil {
			return err
		}
		return output.Print(resp)
	},
}

// ── payments hash ─────────────────────────────────────────────────────────────

var hashPayment paymentFlags

var paymentsHashCmd = &cobra.Command{
	Use:   "hash",
	Short: "Compute the EIP-712 digest of a Payment configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := hashPayment.build()
		if err != nil {
			return err
		}
		resp, err := newClient().Payments.Hash(context.Background(), p)
		if err != nil {
			return err
		}
		return output.Print(resp)
	},
}

func init() {
	// payments get — no extra flags
	paymentsCmd.AddCommand(paymentsGetCmd)

	// payments authorize
	authPayment.addTo(paymentsAuthorizeCmd)
	paymentsAuthorizeCmd.Flags().StringVar(&authAmount, "amount", "", "Amount to pull into escrow (decimal uint256)")
	paymentsAuthorizeCmd.Flags().IntVar(&authV, "v", 0, "EIP-3009 signature recovery id (27 or 28)")
	paymentsAuthorizeCmd.Flags().StringVar(&authR, "r", "", "EIP-3009 signature r component (0x-prefixed bytes32)")
	paymentsAuthorizeCmd.Flags().StringVar(&authS, "s", "", "EIP-3009 signature s component (0x-prefixed bytes32)")
	_ = paymentsAuthorizeCmd.MarkFlagRequired("amount")
	_ = paymentsAuthorizeCmd.MarkFlagRequired("v")
	_ = paymentsAuthorizeCmd.MarkFlagRequired("r")
	_ = paymentsAuthorizeCmd.MarkFlagRequired("s")
	paymentsCmd.AddCommand(paymentsAuthorizeCmd)

	// payments charge
	chargePayment.addTo(paymentsChargeCmd)
	paymentsChargeCmd.Flags().StringVar(&chargeAmount, "amount", "", "Amount to charge (decimal uint256)")
	paymentsChargeCmd.Flags().IntVar(&chargeV, "v", 0, "EIP-3009 signature recovery id (27 or 28)")
	paymentsChargeCmd.Flags().StringVar(&chargeR, "r", "", "EIP-3009 signature r component (0x-prefixed bytes32)")
	paymentsChargeCmd.Flags().StringVar(&chargeS, "s", "", "EIP-3009 signature s component (0x-prefixed bytes32)")
	_ = paymentsChargeCmd.MarkFlagRequired("amount")
	_ = paymentsChargeCmd.MarkFlagRequired("v")
	_ = paymentsChargeCmd.MarkFlagRequired("r")
	_ = paymentsChargeCmd.MarkFlagRequired("s")
	paymentsCmd.AddCommand(paymentsChargeCmd)

	// payments capture
	capturePayment.addTo(paymentsCaptureCmd)
	paymentsCaptureCmd.Flags().StringVar(&captureAmount, "amount", "", "Amount to capture (decimal uint256)")
	_ = paymentsCaptureCmd.MarkFlagRequired("amount")
	paymentsCmd.AddCommand(paymentsCaptureCmd)

	// payments void
	voidPayment.addTo(paymentsVoidCmd)
	paymentsCmd.AddCommand(paymentsVoidCmd)

	// payments release
	releasePayment.addTo(paymentsReleaseCmd)
	paymentsCmd.AddCommand(paymentsReleaseCmd)

	// payments refund
	refundPayment.addTo(paymentsRefundCmd)
	paymentsRefundCmd.Flags().StringVar(&refundAmount, "amount", "", "Amount to refund (decimal uint256)")
	_ = paymentsRefundCmd.MarkFlagRequired("amount")
	paymentsCmd.AddCommand(paymentsRefundCmd)

	// payments authorize-nonce
	paymentsAuthorizeNonceCmd.Flags().StringVar(&authorizeNonceConfigHash, "config-hash", "", "EIP-712 digest of the Payment configuration (from: payments hash)")
	_ = paymentsAuthorizeNonceCmd.MarkFlagRequired("config-hash")
	paymentsCmd.AddCommand(paymentsAuthorizeNonceCmd)

	// payments charge-nonce
	paymentsChargeNonceCmd.Flags().StringVar(&chargeNonceConfigHash, "config-hash", "", "EIP-712 digest of the Payment configuration (from: payments hash)")
	_ = paymentsChargeNonceCmd.MarkFlagRequired("config-hash")
	paymentsCmd.AddCommand(paymentsChargeNonceCmd)

	// payments hash
	hashPayment.addTo(paymentsHashCmd)
	paymentsCmd.AddCommand(paymentsHashCmd)
}
