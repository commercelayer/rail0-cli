package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/term"
	rail0 "github.com/rail0/go-sdk"
	"github.com/spf13/cobra"
)

const zeroAddress = "0x0000000000000000000000000000000000000000"

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

// build constructs a Payment from flags. When running interactively (TTY) and
// required fields are absent it launches a Huh form to collect them.
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
		if term.IsTerminal(os.Stdin.Fd()) {
			return f.buildInteractive()
		}
		return rail0.Payment{}, fmt.Errorf("--payer, --payee, --token, and --max-amount are required (or use --payment-file)")
	}
	return f.toPayment(), nil
}

func (f *paymentFlags) buildInteractive() (rail0.Payment, error) {
	authStr := ""
	if f.authExpiry != 0 {
		authStr = strconv.FormatInt(f.authExpiry, 10)
	}
	refundStr := ""
	if f.refundExpiry != 0 {
		refundStr = strconv.FormatInt(f.refundExpiry, 10)
	}
	feeBpsStr := ""
	if f.feeBps != 0 {
		feeBpsStr = strconv.Itoa(f.feeBps)
	}
	if f.feeReceiver == "" {
		f.feeReceiver = zeroAddress
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Payer").
				Description("Source of funds — buyer address").
				Value(&f.payer),
			huh.NewInput().
				Title("Payee").
				Description("Recipient of captured funds — merchant address").
				Value(&f.payee),
			huh.NewInput().
				Title("Token").
				Description("ERC-20 stablecoin address").
				Value(&f.token),
			huh.NewInput().
				Title("Max amount").
				Description("Upper bound in token base units (e.g. 100000000 for 100 USDC)").
				Value(&f.maxAmount),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Auth expiry").
				Description("Authorization deadline — Unix timestamp").
				Value(&authStr).
				Validate(validateTimestamp),
			huh.NewInput().
				Title("Refund expiry").
				Description("Refund deadline — Unix timestamp").
				Value(&refundStr).
				Validate(validateTimestamp),
			huh.NewInput().
				Title("Fee bps").
				Description("Protocol fee in basis points (0–10000), empty = 0").
				Value(&feeBpsStr),
			huh.NewInput().
				Title("Fee receiver").
				Description("Address receiving the fee (zero address if no fee)").
				Value(&f.feeReceiver).
				Placeholder(zeroAddress),
		),
	)

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr)
			os.Exit(0)
		}
		return rail0.Payment{}, err
	}

	if authStr != "" {
		v, err := strconv.ParseInt(authStr, 10, 64)
		if err != nil {
			return rail0.Payment{}, fmt.Errorf("auth-expiry: %w", err)
		}
		f.authExpiry = v
	}
	if refundStr != "" {
		v, err := strconv.ParseInt(refundStr, 10, 64)
		if err != nil {
			return rail0.Payment{}, fmt.Errorf("refund-expiry: %w", err)
		}
		f.refundExpiry = v
	}
	if feeBpsStr != "" {
		v, err := strconv.Atoi(feeBpsStr)
		if err != nil {
			return rail0.Payment{}, fmt.Errorf("fee-bps: %w", err)
		}
		f.feeBps = v
	}

	return f.toPayment(), nil
}

func (f *paymentFlags) toPayment() rail0.Payment {
	return rail0.Payment{
		Payer:               f.payer,
		Payee:               f.payee,
		Token:               f.token,
		MaxAmount:           f.maxAmount,
		AuthorizationExpiry: f.authExpiry,
		RefundExpiry:        f.refundExpiry,
		FeeBps:              f.feeBps,
		FeeReceiver:         f.feeReceiver,
	}
}

func validateTimestamp(s string) error {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("must be a Unix timestamp (integer)")
	}
	if v < 0 {
		return fmt.Errorf("timestamp must be positive")
	}
	return nil
}
