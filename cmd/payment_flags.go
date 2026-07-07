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
	charge      bool   // -C/--charge: create with mode=charge instead of the default authorize
	description string // optional, human-readable note (e.g. what's being paid for)
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
	cmd.Flags().BoolVarP(&f.charge, "charge", "C", false,
		"Create with mode=charge (immediate transfer, no escrow); default is authorize (escrow then capture)")
	cmd.Flags().StringVarP(&f.description, "description", "d", "",
		"Optional reconciliation note (e.g. order id / invoice); prompted when omitted on a terminal")
}

// build constructs a rail0.CreatePaymentRequest.
//
// Resolution order:
//  1. --payment-file: reads and unmarshals the JSON file.
//  2. If required flags are missing and stdin is a TTY, launches the interactive
//     form (prompting only for the fields not already supplied on the command line).
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
	if term.IsTerminal(os.Stdin.Fd()) {
		// On a terminal, go interactive when a required field is missing OR when
		// --description was omitted. The description is optional, but it's the
		// reconciliation reference, so the creator is always asked — and leaves it
		// blank consciously rather than by silently forgetting the flag.
		if missing || f.description == "" {
			return f.buildInteractive()
		}
	} else if missing {
		return rail0.CreatePaymentRequest{}, fmt.Errorf(
			"--from, --to, --token, --amount, and --chain-id are required (or use --payment-file)")
	}
	return f.resolve()
}

// fieldsToPrompt returns, in prompt order, the required fields left empty on the
// command line — the ones the interactive form should ask for. Pure (no I/O) so
// it's unit-testable; a value supplied via a flag is never included. Mode isn't
// prompted: it defaults to authorize and is switched to charge via -C/--charge.
func (f *createPaymentFlags) fieldsToPrompt() []string {
	var need []string
	if f.from == "" {
		need = append(need, "from")
	}
	if f.to == "" {
		need = append(need, "to")
	}
	if f.token == "" {
		need = append(need, "token")
	}
	if f.amount == "" {
		need = append(need, "amount")
	}
	if f.chainID == 0 {
		need = append(need, "chain-id")
	}
	return need
}

// buildInteractive shows a huh form to collect only the fields fieldsToPrompt
// reports as missing — a value passed via a flag is never re-asked.
func (f *createPaymentFlags) buildInteractive() (rail0.CreatePaymentRequest, error) {
	// When the payee is known, offer the token/chain the merchant actually
	// accepts (discovered from its wallet address) for whichever of the two the
	// buyer omitted — so a buyer who knows only the payee address doesn't have to
	// look its tokens/chains up by hand. Best-effort: it fills f.token/f.chainID
	// when it can; anything left unset falls through to the prompts below.
	if f.to != "" && (f.token == "" || f.chainID == 0) {
		f.proposeFromPaymentMethods()
	}

	// Catalog-backed fallback: for chain/token still unset (no payee, or the
	// merchant discovery above found nothing), let the user pick from the public
	// catalog (GET /blockchains, then GET /tokens for the chosen chain) instead of
	// typing an id/symbol blind. Chain first — the token list is per-chain. Both
	// best-effort: an unreachable gateway or empty catalog falls through to the
	// free-text prompt below.
	if f.chainID == 0 {
		f.pickChainFromCatalog()
	}
	if f.token == "" {
		f.pickTokenFromCatalog()
	}

	chainIDStr := ""
	if f.chainID > 0 {
		chainIDStr = strconv.Itoa(f.chainID)
	}

	var fields []huh.Field
	for _, name := range f.fieldsToPrompt() {
		switch name {
		case "from":
			fields = append(fields, huh.NewInput().
				Title("From (payer)").
				Description("Payer wallet address (0x-prefixed) — funds are pulled from here").
				Value(&f.from))
		case "to":
			fields = append(fields, huh.NewInput().
				Title("To (payee)").
				Description("Payee wallet address (0x-prefixed) — use address from 'wallets'").
				Value(&f.to))
		case "token":
			fields = append(fields, huh.NewInput().
				Title("Token").
				Description("Token symbol (e.g. USDC) — see 'rail0 tokens'").
				Value(&f.token))
		case "amount":
			fields = append(fields, huh.NewInput().
				Title("Amount").
				Description("Amount as a decimal in token units (e.g. 1.00 = 1 USDC); the gateway converts to base units").
				Value(&f.amount))
		case "chain-id":
			fields = append(fields, huh.NewInput().
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
				}))
		}
	}

	// Always ask for the description when it wasn't passed on the command line.
	// It's optional (blank is accepted), but it's the reconciliation reference,
	// so prompting makes leaving it empty a conscious choice rather than a
	// forgotten flag. No Validate → an empty answer is fine.
	if f.description == "" {
		fields = append(fields, huh.NewInput().
			Title("Description (optional)").
			Description("Reference to reconcile this payment later (order id, invoice, note). Leave blank to omit.").
			Value(&f.description))
	}

	// Only run the form when there's actually something to ask. (build() enters
	// here when a required field OR the description is missing, so this holds.)
	if len(fields) > 0 {
		if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				fmt.Fprintln(os.Stderr)
				os.Exit(0)
			}
			return rail0.CreatePaymentRequest{}, err
		}
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

	mode := "authorize"
	if f.charge {
		mode = "charge"
	}

	return rail0.CreatePaymentRequest{
		Payer:       payer,
		Payee:       payee,
		Token:       tokenAddress,
		Amount:      f.amount,
		ChainID:     chainID,
		Mode:        mode,
		Description: f.description,
	}, nil
}

// acceptedMethod is one (token symbol, chain id) combination a merchant accepts.
type acceptedMethod struct {
	symbol  string
	chainID int
}

// acceptedMethods flattens the discovered payee wallets to the distinct active
// (symbol, chain) pairs the merchant accepts, narrowed to a token symbol and/or
// chain id when the buyer already supplied one ("" / 0 = no filter). Order
// follows first appearance so the resulting picker is stable. Pure (no I/O) so
// it's unit-testable.
func acceptedMethods(wallets []rail0.WalletWithTokens, token string, chainID int) []acceptedMethod {
	var out []acceptedMethod
	seen := map[string]bool{}
	for _, w := range wallets {
		for _, h := range w.Tokens {
			if !h.Active || h.Token.Symbol == "" || h.Token.ChainID == 0 {
				continue
			}
			if token != "" && !strings.EqualFold(h.Token.Symbol, token) {
				continue
			}
			if chainID != 0 && h.Token.ChainID != chainID {
				continue
			}
			key := fmt.Sprintf("%s@%d", strings.ToUpper(h.Token.Symbol), h.Token.ChainID)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, acceptedMethod{symbol: h.Token.Symbol, chainID: h.Token.ChainID})
		}
	}
	return out
}

// proposeFromPaymentMethods discovers what the merchant accepts from the payee
// address (public GET /payment_methods) and fills whichever of --token /
// --chain-id the buyer omitted: a single match (given what was already supplied)
// is taken automatically, several are offered as a picker. Best-effort — on a
// discovery error, an unknown @alias, or a merchant exposing no active methods
// it returns quietly, leaving the fields to the normal prompts and to resolve()'s
// validation.
func (f *createPaymentFlags) proposeFromPaymentMethods() {
	payee, err := resolveAlias(f.to)
	if err != nil || payee == "" {
		return
	}
	wallets, err := newClient().PaymentMethods.List(context.Background(), rail0.PaymentMethodsQuery{Address: payee})
	if err != nil {
		return
	}
	methods := acceptedMethods(wallets, f.token, f.chainID)
	if len(methods) == 0 {
		return
	}

	pick := methods[0]
	if len(methods) > 1 {
		opts := make([]huh.Option[string], 0, len(methods))
		for i, m := range methods {
			opts = append(opts, huh.NewOption(fmt.Sprintf("%s · chain %d", m.symbol, m.chainID), strconv.Itoa(i)))
		}
		var chosen string
		if err := huh.NewSelect[string]().
			Title(f.methodPromptTitle()).
			Options(opts...).
			Value(&chosen).
			Run(); err != nil {
			abortOrReturn(err)
			return
		}
		idx, _ := strconv.Atoi(chosen)
		pick = methods[idx]
	} else {
		// Exactly one accepted method given what was already supplied — take it
		// and say so, since no picker is shown.
		fmt.Fprintf(os.Stderr, "Using the merchant's %s on chain %d\n", pick.symbol, pick.chainID)
	}

	if f.token == "" {
		f.token = pick.symbol
	}
	if f.chainID == 0 {
		f.chainID = pick.chainID
	}
}

// pickChainFromCatalog fills f.chainID from the public chain catalog
// (GET /blockchains): a single active chain is taken automatically, several are
// offered as a picker. Best-effort — on a fetch error or an empty catalog it
// returns quietly, leaving the field to the free-text prompt.
func (f *createPaymentFlags) pickChainFromCatalog() {
	chains, err := newClient().Chains.List(context.Background())
	if err != nil || len(chains) == 0 {
		return
	}
	if len(chains) == 1 {
		f.chainID = chains[0].ChainID
		return
	}
	opts := make([]huh.Option[string], 0, len(chains))
	for i, c := range chains {
		opts = append(opts, huh.NewOption(fmt.Sprintf("%s (%d)", c.Name, c.ChainID), strconv.Itoa(i)))
	}
	var chosen string
	if err := huh.NewSelect[string]().
		Title("Select the chain").
		Options(opts...).
		Value(&chosen).
		Run(); err != nil {
		abortOrReturn(err)
		return
	}
	idx, _ := strconv.Atoi(chosen)
	f.chainID = chains[idx].ChainID
}

// pickTokenFromCatalog fills f.token from the active tokens on the chosen chain
// (GET /tokens?chain_id=): a single token is taken automatically, several are
// offered as a picker. Needs f.chainID set (the token catalog is per-chain), so
// it's a no-op until the chain is known. Best-effort, like pickChainFromCatalog.
func (f *createPaymentFlags) pickTokenFromCatalog() {
	if f.chainID == 0 {
		return
	}
	tokens, err := newClient().Tokens.List(context.Background(), rail0.ListTokensParams{ChainID: f.chainID})
	if err != nil || len(tokens) == 0 {
		return
	}
	if len(tokens) == 1 {
		f.token = tokens[0].Symbol
		return
	}
	opts := make([]huh.Option[string], 0, len(tokens))
	for _, t := range tokens {
		opts = append(opts, huh.NewOption(t.Symbol, t.Symbol))
	}
	var chosen string
	if err := huh.NewSelect[string]().
		Title(fmt.Sprintf("Select the token on chain %d", f.chainID)).
		Options(opts...).
		Value(&chosen).
		Run(); err != nil {
		abortOrReturn(err)
		return
	}
	f.token = chosen
}

// methodPromptTitle phrases the payment-method picker for whichever of token /
// chain the buyer left unspecified.
func (f *createPaymentFlags) methodPromptTitle() string {
	switch {
	case f.token == "" && f.chainID == 0:
		return "Select the token and chain the merchant accepts"
	case f.token == "":
		return fmt.Sprintf("Select the token to pay on chain %d", f.chainID)
	default:
		return fmt.Sprintf("Select the chain to pay %s on", f.token)
	}
}
