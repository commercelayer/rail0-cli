package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	rail0 "github.com/rail0/go-sdk"
	"github.com/spf13/cobra"
)

const zeroAddress = "0x0000000000000000000000000000000000000000"

// paymentFlags collects the flags needed to construct a rail0.Payment.
// Use addTo to register the flags on a command and build to create the struct.
type paymentFlags struct {
	paymentFile  string
	payer        string
	payee        string
	token        string
	maxAmount    string
	authExpiry   int64
	refundExpiry int64
	feeBps       int
	feeReceiver  string
}

// addTo registers the payment flags on cmd.
func (f *paymentFlags) addTo(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.paymentFile, "payment-file", "", "Path to a JSON file containing the Payment object (overrides individual flags)")
	cmd.Flags().StringVar(&f.payer, "payer", "", "Payer address (0x-prefixed)")
	cmd.Flags().StringVar(&f.payee, "payee", "", "Payee address (0x-prefixed)")
	cmd.Flags().StringVar(&f.token, "token", "", "ERC-20 token address (0x-prefixed)")
	cmd.Flags().StringVar(&f.maxAmount, "max-amount", "", "Maximum amount (decimal uint256 string)")
	cmd.Flags().Int64Var(&f.authExpiry, "auth-expiry", 0, "Authorization expiry Unix timestamp")
	cmd.Flags().Int64Var(&f.refundExpiry, "refund-expiry", 0, "Refund expiry Unix timestamp")
	cmd.Flags().IntVar(&f.feeBps, "fee-bps", 0, "Protocol fee in basis points (0–10000)")
	cmd.Flags().StringVar(&f.feeReceiver, "fee-receiver", zeroAddress, "Fee receiver address (zero when fee-bps is 0)")
}

// build returns a rail0.Payment from the registered flags.
// If --payment-file is set it takes priority over individual flags.
func (f *paymentFlags) build() (rail0.Payment, error) {
	if f.paymentFile != "" {
		b, err := os.ReadFile(f.paymentFile)
		if err != nil {
			return rail0.Payment{}, fmt.Errorf("read payment file: %w", err)
		}
		var p rail0.Payment
		if err := json.Unmarshal(b, &p); err != nil {
			return rail0.Payment{}, fmt.Errorf("parse payment file: %w", err)
		}
		return p, nil
	}
	if f.payer == "" || f.payee == "" || f.token == "" || f.maxAmount == "" {
		return rail0.Payment{}, fmt.Errorf("--payer, --payee, --token, and --max-amount are required (or use --payment-file)")
	}
	return rail0.Payment{
		Payer:               f.payer,
		Payee:               f.payee,
		Token:               f.token,
		MaxAmount:           f.maxAmount,
		AuthorizationExpiry: f.authExpiry,
		RefundExpiry:        f.refundExpiry,
		FeeBps:              f.feeBps,
		FeeReceiver:         f.feeReceiver,
	}, nil
}
