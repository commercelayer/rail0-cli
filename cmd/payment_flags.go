package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/term"
	rail0 "github.com/rail0/go-sdk"
	"github.com/spf13/cobra"
)

// createPaymentFlags collects the CLI flags for the `payments create` command.
// They map 1-to-1 to rail0.PaymentInput + ChainID + Mode.
type createPaymentFlags struct {
	paymentFile string
	from        string
	to          string
	token       string // symbol (e.g. "USDC") resolved to address at build time
	amount      string
	chain       string // slug (e.g. "arc-testnet") — mutually exclusive with chainID
	chainID     int    // numeric chain id — mutually exclusive with chain
	mode        string
}

// addTo registers all create-payment flags on cmd.
func (f *createPaymentFlags) addTo(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&f.paymentFile, "payment-file", "f", "",
		"Path to a JSON file with the full request body (overrides individual flags)")
	cmd.Flags().StringVarP(&f.from, "from", "F", "",
		"Payer wallet address (from) (0x-prefixed, funds are pulled from here)")
	cmd.Flags().StringVarP(&f.to, "to", "T", "",
		"Payee wallet address (to) (0x-prefixed, address from accounts wallets)")
	cmd.Flags().StringVarP(&f.token, "token", "t", "",
		"Token symbol as listed in GET /tokens (e.g. USDC)")
	cmd.Flags().StringVarP(&f.amount, "amount", "a", "",
		"Amount to pay in token base units (e.g. '1000000' for 1 USDC)")
	cmd.Flags().StringVarP(&f.chain, "chain", "c", "",
		"Chain slug as listed in GET /blockchains (e.g. arc-testnet) — exclusive with --chain-id")
	cmd.Flags().IntVarP(&f.chainID, "chain-id", "n", 0,
		"EVM chain ID (e.g. 5042002) — exclusive with --chain")
	cmd.MarkFlagsMutuallyExclusive("chain", "chain-id")
	cmd.Flags().StringVarP(&f.mode, "mode", "m", "authorize",
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

	missing := f.from == "" || f.to == "" || f.token == "" || f.amount == "" || (f.chain == "" && f.chainID == 0)
	if missing {
		if term.IsTerminal(os.Stdin.Fd()) {
			return f.buildInteractive()
		}
		return rail0.CreatePaymentRequest{}, fmt.Errorf(
			"--from, --to, --token, --amount, and --chain (or --chain-id) are required (or use --payment-file)")
	}
	return f.resolve()
}

// buildInteractive shows a two-page huh form to collect missing payment fields.
func (f *createPaymentFlags) buildInteractive() (rail0.CreatePaymentRequest, error) {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("From (payer)").
				Description("Payer wallet address (0x-prefixed) — funds are pulled from here").
				Value(&f.from),
			huh.NewInput().
				Title("To (payee)").
				Description("Payee wallet address (0x-prefixed) — use address from accounts wallets").
				Value(&f.to),
			huh.NewInput().
				Title("Token").
				Description("Token symbol (e.g. USDC) — see 'rail0 catalog tokens'").
				Value(&f.token),
			huh.NewInput().
				Title("Amount").
				Description("Amount in token base units (e.g. 1000000 = 1 USDC with 6 decimals)").
				Value(&f.amount),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Chain").
				Description("Chain slug (e.g. arc-testnet) — see 'rail0 catalog blockchains'").
				Value(&f.chain).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("chain is required")
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

	return f.resolve()
}

// resolve calls GET /blockchains and GET /tokens to turn the human-readable
// chain slug and token symbol into the chain_id and token address required
// by the API, then builds the CreatePaymentRequest.
func (f *createPaymentFlags) resolve() (rail0.CreatePaymentRequest, error) {
	ctx := context.Background()
	client := newClient()

	// Resolve chain: either slug → chain_id, or chain_id used directly.
	chainID := f.chainID
	if chainID == 0 {
		chains, err := client.Chains.List(ctx)
		if err != nil {
			return rail0.CreatePaymentRequest{}, fmt.Errorf("fetch blockchains: %w", err)
		}
		for _, b := range chains {
			if strings.EqualFold(b.Slug, f.chain) {
				chainID = b.ChainID
				break
			}
		}
		if chainID == 0 {
			slugs := make([]string, 0, len(chains))
			for _, b := range chains {
				slugs = append(slugs, b.Slug)
			}
			return rail0.CreatePaymentRequest{}, fmt.Errorf(
				"unknown chain %q — available: %s", f.chain, strings.Join(slugs, ", "))
		}
	}

	// Resolve token symbol → address on the resolved chain.
	tokens, err := client.Tokens.List(ctx, chainID)
	if err != nil {
		return rail0.CreatePaymentRequest{}, fmt.Errorf("fetch tokens: %w", err)
	}
	var tokenAddress string
	for _, t := range tokens {
		if strings.EqualFold(t.Symbol, f.token) {
			tokenAddress = string(t.Address)
			break
		}
	}
	if tokenAddress == "" {
		syms := make([]string, 0, len(tokens))
		for _, t := range tokens {
			syms = append(syms, t.Symbol)
		}
		return rail0.CreatePaymentRequest{}, fmt.Errorf(
			"unknown token %q on chain %q — available: %s", f.token, f.chain, strings.Join(syms, ", "))
	}

	return rail0.CreatePaymentRequest{
		Payer:   f.from,
		Payee:   f.to,
		Token:   tokenAddress,
		Amount:  f.amount,
		ChainId: chainID,
		Mode:    f.mode,
	}, nil
}

