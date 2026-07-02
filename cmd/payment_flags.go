package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/term"
	rail0 "github.com/rail0/go-sdk"
	"github.com/spf13/cobra"
)

// createPaymentFlags collects the CLI flags for the `payments create` command.
// They map 1-to-1 to the fields of rail0.CreatePaymentRequest.
type createPaymentFlags struct {
	paymentFile string
	from        string
	to          string
	token       string // symbol (e.g. "USDC") resolved to address at build time
	amount      string
	chainID     int    // numeric chain id
	mode        string
}

// addTo registers all create-payment flags on cmd.
func (f *createPaymentFlags) addTo(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&f.paymentFile, "payment-file", "f", "",
		"Path to a JSON file with the full request body (overrides individual flags)")
	cmd.Flags().StringVarP(&f.from, "from", "F", "",
		"Payer wallet address (from) (0x-prefixed, funds are pulled from here)")
	cmd.Flags().StringVarP(&f.to, "to", "T", "",
		"Payee wallet address (to) (0x-prefixed, address from 'wallets')")
	cmd.Flags().StringVarP(&f.token, "token", "t", "",
		"Token symbol as listed in GET /tokens (e.g. USDC)")
	cmd.Flags().StringVarP(&f.amount, "amount", "a", "",
		"Amount as a decimal in token units (e.g. '1.00' for 1 USDC); the gateway converts to base units")
	cmd.Flags().IntVarP(&f.chainID, "chain-id", "c", 0,
		"EVM chain ID as listed in GET /blockchains (e.g. 5042002)")
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

	missing := f.from == "" || f.to == "" || f.token == "" || f.amount == "" || f.chainID == 0
	if missing {
		if term.IsTerminal(os.Stdin.Fd()) {
			return f.buildInteractive()
		}
		return rail0.CreatePaymentRequest{}, fmt.Errorf(
			"--from, --to, --token, --amount, and --chain-id are required (or use --payment-file)")
	}
	return f.resolve()
}

// buildInteractive shows a two-page huh form to collect missing payment fields.
func (f *createPaymentFlags) buildInteractive() (rail0.CreatePaymentRequest, error) {
	chainIDStr := ""
	if f.chainID > 0 {
		chainIDStr = strconv.Itoa(f.chainID)
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("From (payer)").
				Description("Payer wallet address (0x-prefixed) — funds are pulled from here").
				Value(&f.from),
			huh.NewInput().
				Title("To (payee)").
				Description("Payee wallet address (0x-prefixed) — use address from 'wallets'").
				Value(&f.to),
			huh.NewInput().
				Title("Token").
				Description("Token symbol (e.g. USDC) — see 'rail0 catalog tokens'").
				Value(&f.token),
			huh.NewInput().
				Title("Amount").
				Description("Amount as a decimal in token units (e.g. 1.00 = 1 USDC); the gateway converts to base units").
				Value(&f.amount),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Chain ID").
				Description("EVM chain ID (e.g. 5042002) — see 'rail0 chains'").
				Value(&chainIDStr).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("chain id is required")
					}
					if _, err := strconv.Atoi(s); err != nil {
						return fmt.Errorf("chain id must be a number")
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

	if chainIDStr != "" {
		id, err := strconv.Atoi(chainIDStr)
		if err != nil {
			return rail0.CreatePaymentRequest{}, fmt.Errorf("invalid chain id: %w", err)
		}
		f.chainID = id
	}

	return f.resolve()
}

// resolve calls GET /tokens to turn the human-readable token symbol into the
// token address required by the API on the selected chain, then builds the
// CreatePaymentRequest.
func (f *createPaymentFlags) resolve() (rail0.CreatePaymentRequest, error) {
	ctx := context.Background()
	client := newClient()

	// Resolve any "@name" address-book references to concrete addresses.
	payer, err := resolveAlias(f.from)
	if err != nil {
		return rail0.CreatePaymentRequest{}, err
	}
	payee, err := resolveAlias(f.to)
	if err != nil {
		return rail0.CreatePaymentRequest{}, err
	}

	chainID := f.chainID

	// Resolve token symbol → address on the selected chain.
	tokens, err := client.Tokens.List(ctx, rail0.ListTokensParams{ChainID: chainID})
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
			"unknown token %q on chain %d — available: %s", f.token, chainID, strings.Join(syms, ", "))
	}

	return rail0.CreatePaymentRequest{
		Payer:   payer,
		Payee:   payee,
		Token:   tokenAddress,
		Amount:  f.amount,
		ChainID: chainID,
		Mode:    f.mode,
	}, nil
}

