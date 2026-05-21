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

// createPaymentFlags collects the CLI flags for the `payments create` command.
// They map 1-to-1 to rail0.PaymentInput + ChainID + Mode.
type createPaymentFlags struct {
	paymentFile string
	payer       string
	payee       string
	token       string
	amount      string
	chainID     int
	mode        string
}

// addTo registers all create-payment flags on cmd.
func (f *createPaymentFlags) addTo(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.paymentFile, "payment-file", "",
		"Path to a JSON file with the full request body (overrides individual flags)")
	cmd.Flags().StringVar(&f.payer, "payer", "",
		"Buyer wallet address (0x-prefixed, funds are pulled from here)")
	cmd.Flags().StringVar(&f.payee, "payee", "",
		"Merchant wallet address (0x-prefixed, walletAddress from merchants payment-methods)")
	cmd.Flags().StringVar(&f.token, "token", "",
		"ERC-20 token address (0x-prefixed, tokenAddress from merchants payment-methods)")
	cmd.Flags().StringVar(&f.amount, "amount", "",
		"Amount to pay in token base units (e.g. '100000000' for 100 USDC)")
	cmd.Flags().IntVar(&f.chainID, "chain-id", 0,
		"EVM chain ID of the target network (e.g. 42220 for Celo)")
	cmd.Flags().StringVar(&f.mode, "mode", "authorize",
		"Payment mode: 'authorize' (escrow then capture) or 'charge' (immediate, no escrow)")
}

// build constructs a rail0.CreatePaymentRequest.
//
// Resolution order:
//  1. --payment-file: reads and unmarshals the JSON file.
//  2. If required flags are missing and stdin is a TTY, launches the interactive form.
//  3. Otherwise validates that all required fields are present.
func (f *createPaymentFlags) build() (rail0.CreatePaymentRequest, error) {
	if f.paymentFile != "" {
		b, err := os.ReadFile(f.paymentFile)
		if err != nil {
			return rail0.CreatePaymentRequest{}, fmt.Errorf("read payment file: %w", err)
		}
		var req rail0.CreatePaymentRequest
		if err := json.Unmarshal(b, &req); err != nil {
			return rail0.CreatePaymentRequest{}, fmt.Errorf("parse payment file: %w", err)
		}
		return req, nil
	}

	missing := f.payer == "" || f.payee == "" || f.token == "" || f.amount == "" || f.chainID == 0
	if missing {
		if term.IsTerminal(os.Stdin.Fd()) {
			return f.buildInteractive()
		}
		return rail0.CreatePaymentRequest{}, fmt.Errorf(
			"--payer, --payee, --token, --amount, and --chain-id are required (or use --payment-file)")
	}
	return f.toRequest(), nil
}

// buildInteractive shows a two-page huh form to collect missing payment fields.
func (f *createPaymentFlags) buildInteractive() (rail0.CreatePaymentRequest, error) {
	chainStr := ""
	if f.chainID != 0 {
		chainStr = strconv.Itoa(f.chainID)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Payer").
				Description("Buyer wallet address (0x-prefixed) — funds are pulled from here").
				Value(&f.payer),
			huh.NewInput().
				Title("Payee").
				Description("Merchant wallet address (0x-prefixed) — use walletAddress from merchants payment-methods").
				Value(&f.payee),
			huh.NewInput().
				Title("Token").
				Description("ERC-20 token address (0x-prefixed) — use tokenAddress from merchants payment-methods").
				Value(&f.token),
			huh.NewInput().
				Title("Amount").
				Description("Amount in token base units (e.g. 100000000 = 100 USDC with 6 decimals)").
				Value(&f.amount),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Chain ID").
				Description("EVM chain ID (e.g. 42220 for Celo, 5042002 for Arc Testnet)").
				Value(&chainStr).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("chain-id is required")
					}
					v, err := strconv.Atoi(s)
					if err != nil || v <= 0 {
						return fmt.Errorf("must be a positive integer")
					}
					return nil
				}),
			huh.NewSelect[string]().
				Title("Mode").
				Description("authorize = escrow then capture; charge = immediate transfer").
				Options(
					huh.NewOption("authorize (escrow + capture)", "authorize"),
					huh.NewOption("charge (immediate, no escrow)", "charge"),
				).
				Value(&f.mode),
		),
	)

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr)
			os.Exit(0)
		}
		return rail0.CreatePaymentRequest{}, err
	}

	if chainStr != "" {
		v, err := strconv.Atoi(chainStr)
		if err != nil {
			return rail0.CreatePaymentRequest{}, fmt.Errorf("chain-id: %w", err)
		}
		f.chainID = v
	}

	return f.toRequest(), nil
}

func (f *createPaymentFlags) toRequest() rail0.CreatePaymentRequest {
	return rail0.CreatePaymentRequest{
		Payment: rail0.PaymentInput{
			Payer:  f.payer,
			Payee:  f.payee,
			Token:  f.token,
			Amount: f.amount,
		},
		ChainId: f.chainID,
		Mode:    f.mode,
	}
}
