package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/term"
	rail0 "github.com/rail0/go-sdk"
	"github.com/spf13/cobra"
)

// amountPattern matches a non-negative decimal in token units (e.g. "1", "1.00").
// The positivity (> 0) check is done separately in validateAmount.
var amountPattern = regexp.MustCompile(`^\d+(\.\d+)?$`)

// validateAddress checks a payer/payee value the way resolve() will later use it:
// an "@alias" must exist in the address book, and a literal must be a well-formed
// 0x EVM address. It runs no network I/O (alias lookup is local config), so it's
// safe to call up front — on a flag before any prompt, and inline as a huh field
// validator — to reject a bad address at the point it's entered rather than after
// the rest of the form.
func validateAddress(v string) error {
	if strings.HasPrefix(v, "@") {
		_, err := resolveAlias(v) // unknown alias → error
		return err
	}
	if !evmAddressPattern.MatchString(v) {
		return fmt.Errorf("invalid address %q: expected a 0x-prefixed 40-hex-character EVM address, or an @alias", v)
	}
	return nil
}

// validateAmount checks the amount is a positive decimal in token units. Purely
// syntactic (the gateway converts to base units and enforces token decimals); it
// exists to reject an obviously wrong value at entry rather than after a round-trip.
func validateAmount(v string) error {
	if !amountPattern.MatchString(v) {
		return fmt.Errorf("invalid amount %q: use a decimal in token units (e.g. 1.00)", v)
	}
	// Reject zero (any mix of 0s and a dot strips to empty).
	if strings.Trim(strings.ReplaceAll(v, ".", ""), "0") == "" {
		return fmt.Errorf("amount must be greater than zero")
	}
	return nil
}

var (
	tokenCatalogMu    sync.Mutex
	tokenCatalogCache = map[int][]rail0.Token{}
)

// activeTokens returns the ACTIVE tokens on chainID (GET /tokens), fetched at most once
// per chain per process. Mirrors chainCatalog (cmd/chains.go) for the same reason: an
// interactive create consulted this list three to four times for the SAME chain —
// assertTokenKnown, the huh field validator (re-run on every submit attempt), resolve,
// and the token picker — each on a freshly constructed client, so the form visibly
// stalled between prompts on a slow link. (#69)
//
// One deliberate difference from chainCatalog: a FAILURE is not cached. Chains cache
// the empty slice and degrade to a bare chain id, which is a cosmetic fallback. Here an
// empty list means "unknown token — available: " and the user could not recover inside
// the same command, so a transient error stays retryable.
//
// Active-only for the same reason as lookupTokenAddress: /tokens also returns retired
// tokens (an existing payment references its token forever), but this is the create
// path, where offering one would only earn a 422.
func activeTokens(ctx context.Context, chainID int) ([]rail0.Token, error) {
	tokenCatalogMu.Lock()
	defer tokenCatalogMu.Unlock()
	if cached, ok := tokenCatalogCache[chainID]; ok {
		return cached, nil
	}
	activeOnly := true
	tokens, err := newClient().Tokens.List(ctx, rail0.ListTokensParams{ChainID: chainID, Active: &activeOnly})
	if err != nil {
		return nil, err
	}
	tokenCatalogCache[chainID] = tokens
	return tokens, nil
}

// lookupTokenAddress turns a token symbol into its address on chainID via
// GET /tokens. It returns the resolved address, the token's decimals (for
// precision checks), the symbols available on that chain (for a helpful error),
// and an error when the symbol isn't listed. Shared by resolve() and the
// interactive token validator so the symbol→address rule lives in one place.
//
// Filters to ACTIVE tokens: /tokens returns retired ones too (an existing
// payment references its token forever and must stay resolvable), but this is
// the create path — offering a retired symbol would only produce a 422 from the
// gateway, and listing it among "available" symbols would be misleading.
func lookupTokenAddress(ctx context.Context, chainID int, symbol string) (string, int, []string, error) {
	tokens, err := activeTokens(ctx, chainID)
	if err != nil {
		return "", 0, nil, fmt.Errorf("fetch tokens: %w", err)
	}
	syms := make([]string, 0, len(tokens))
	for _, t := range tokens {
		syms = append(syms, t.Symbol)
		if strings.EqualFold(t.Symbol, symbol) {
			return string(t.Address), t.Decimals, syms, nil
		}
	}
	return "", 0, syms, fmt.Errorf("unknown token %q on %s — available: %s", symbol, chainLabel(chainID), strings.Join(syms, ", "))
}

// assertAmountPrecision rejects a human amount carrying more decimal places than
// the token supports — the gateway would reject it (422 invalid_amount), so we
// fail early with a clearer message once the token's decimals are known.
func assertAmountPrecision(amount string, decimals int) error {
	s := strings.TrimSpace(amount)
	i := strings.IndexByte(s, '.')
	if i < 0 {
		return nil
	}
	if frac := len(s) - i - 1; frac > decimals {
		return fmt.Errorf("amount %q has %d decimal places, but the token supports at most %d", s, frac, decimals)
	}
	return nil
}

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

	// The address flags accept an @alias, so they complete to the address book —
	// which is the whole reason to keep one. (#110)
	_ = cmd.RegisterFlagCompletionFunc("from", completeAliasNames)
	_ = cmd.RegisterFlagCompletionFunc("to", completeAliasNames)
}

// build constructs and validates a rail0.CreatePaymentRequest. It resolves the
// request (buildResolved) and then enforces the cross-field invariant that a
// payment can't be sent from an address to itself.
func (f *createPaymentFlags) build() (rail0.CreatePaymentRequest, error) {
	req, err := f.buildResolved()
	if err != nil {
		return rail0.CreatePaymentRequest{}, err
	}
	// from and to must differ. Checked on the RESOLVED request (0x addresses), so
	// it also catches two aliases — or an alias and a raw address — pointing to
	// the same wallet. Compared case-insensitively. The gateway enforces this too.
	if req.Payer != "" && strings.EqualFold(req.Payer, req.Payee) {
		return rail0.CreatePaymentRequest{}, fmt.Errorf(
			"payer (--from) and payee (--to) must be different addresses — a payment can't be sent from an address to itself")
	}
	return req, nil
}

// buildResolved constructs a rail0.CreatePaymentRequest.
//
// Resolution order:
//  1. --payment-file: reads and unmarshals the JSON file.
//  2. If required flags are missing and stdin is a TTY, launches the interactive
//     form (prompting only for the fields not already supplied on the command line).
//  3. Otherwise validates that all required fields are present.
func (f *createPaymentFlags) buildResolved() (rail0.CreatePaymentRequest, error) {
	if f.paymentFile != "" {
		b, err := os.ReadFile(f.paymentFile)
		if err != nil {
			return rail0.CreatePaymentRequest{}, fmt.Errorf("read payment file: %w", err)
		}
		var req rail0.CreatePaymentRequest
		if err := json.Unmarshal(b, &req); err != nil {
			return rail0.CreatePaymentRequest{}, fmt.Errorf("parse payment file: %w", err)
		}
		// A file-supplied body used to skip every local check the flag path runs,
		// so a file written by analogy with the flags ("token": "USDC" instead of
		// an address, or an over-precise amount) reached the gateway as a bare
		// unknown_token / invalid_amount with none of the local hints. (#97)
		if err := validateFileRequest(req); err != nil {
			return rail0.CreatePaymentRequest{}, err
		}
		return req, nil
	}

	// Fail fast on anything the user DID pass that's malformed, before showing any
	// prompt — it's confusing to ask for the remaining fields and only then reject
	// a value that was wrong from the start. Only local checks here; a supplied
	// token needs the chain to be known, so it's validated later (see build paths).
	if err := f.validateSupplied(); err != nil {
		return rail0.CreatePaymentRequest{}, err
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

// validateFileRequest applies to a --payment-file body the same local checks the
// flag path gets. The file speaks the WIRE shape (token as an address, not a
// symbol), so this is not simply validateSupplied: the token check is the
// interesting one, because writing the symbol there is the natural mistake.
func validateFileRequest(req rail0.CreatePaymentRequest) error {
	if req.Payer == "" || req.Payee == "" || req.Token == "" || req.Amount == "" || req.ChainID == 0 {
		return fmt.Errorf("payment file must set payer, payee, token, amount and chain_id")
	}
	if err := validateAddress(string(req.Payer)); err != nil {
		return fmt.Errorf("payment file payer: %w", err)
	}
	if err := validateAddress(string(req.Payee)); err != nil {
		return fmt.Errorf("payment file payee: %w", err)
	}
	if err := validateAddress(string(req.Token)); err != nil {
		return fmt.Errorf("payment file token: %w (in a payment file the token is its contract ADDRESS, not a symbol like USDC — the symbol form is only for --token)", err)
	}
	if err := validateAmount(string(req.Amount)); err != nil {
		return fmt.Errorf("payment file amount: %w", err)
	}
	// Precision needs the token's decimals, which needs the chain — resolve the
	// token by address among the chain's active ones. A token the chain doesn't
	// have is left to the gateway (it owns the catalogue, and a retired token
	// still resolves there for an existing payment).
	ctx := context.Background()
	if tokens, err := activeTokens(ctx, req.ChainID); err == nil {
		for _, t := range tokens {
			if strings.EqualFold(string(t.Address), string(req.Token)) {
				return assertAmountPrecision(string(req.Amount), t.Decimals)
			}
		}
	}
	return nil
}

// validateSupplied rejects a malformed value among the flags the user actually
// passed, using only local checks (no network) so it can run before any prompt.
// A supplied --token can't be checked here (its validity is chain-scoped, and the
// chain may still be unknown); assertTokenKnown validates it once the chain is
// set.
func (f *createPaymentFlags) validateSupplied() error {
	if f.from != "" {
		if err := validateAddress(f.from); err != nil {
			return fmt.Errorf("--from: %w", err)
		}
	}
	if f.to != "" {
		if err := validateAddress(f.to); err != nil {
			return fmt.Errorf("--to: %w", err)
		}
	}
	if f.amount != "" {
		if err := validateAmount(f.amount); err != nil {
			return fmt.Errorf("--amount: %w", err)
		}
	}
	return nil
}

// assertTokenKnown validates a supplied token symbol against the chain once both
// are known, so a wrong --token fails before the rest of the flow instead of
// after. A no-op until the chain is set (a token's validity is chain-scoped) and
// when no token was given. The non-interactive path relies on resolve() for the
// same check; this exists for the interactive path, where it runs before any
// prompt so the failure isn't buried behind the form.
func (f *createPaymentFlags) assertTokenKnown() error {
	if f.token == "" || f.chainID == 0 {
		return nil
	}
	_, _, _, err := lookupTokenAddress(context.Background(), f.chainID, f.token)
	return err
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
	// chain-id is asked before token: a token symbol is only valid on a specific
	// chain, so the token prompt validates against an already-known chain.
	if f.chainID == 0 {
		need = append(need, "chain-id")
	}
	if f.token == "" {
		need = append(need, "token")
	}
	if f.amount == "" {
		need = append(need, "amount")
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

	// A token passed on the command line can only be checked once the chain is
	// known (supplied, or just chosen above). Do it now, before the form, so a
	// wrong --token fails immediately rather than after the remaining prompts.
	if err := f.assertTokenKnown(); err != nil {
		return rail0.CreatePaymentRequest{}, err
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
				Value(&f.from).
				Validate(validateAddress))
		case "to":
			fields = append(fields, huh.NewInput().
				Title("To (payee)").
				Description("Payee wallet address (0x-prefixed) — use address from 'wallets'").
				Value(&f.to).
				Validate(validateAddress))
		case "token":
			// The token symbol must exist on the chosen chain. The chain is either
			// already known (f.chainID) or entered in the chain-id field just above,
			// so read whichever is set. When neither is known yet (an unreachable
			// catalog left the chain to a free-text prompt), skip — resolve() checks.
			fields = append(fields, huh.NewInput().
				Title("Token").
				Description("Token symbol (e.g. USDC) — see 'rail0 tokens'").
				Value(&f.token).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("token is required")
					}
					cid := f.chainID
					if cid == 0 {
						cid, _ = strconv.Atoi(chainIDStr)
					}
					if cid == 0 {
						return nil
					}
					_, _, _, err := lookupTokenAddress(context.Background(), cid, s)
					return err
				}))
		case "amount":
			fields = append(fields, huh.NewInput().
				Title("Amount").
				Description("Amount as a decimal in token units (e.g. 1.00 = 1 USDC); the gateway converts to base units").
				Value(&f.amount).
				Validate(validateAmount))
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
			// Non-zero: cancelling the create form is not success. See
			// abortOrReturn / ExitCodeAborted. (#65)
			if errors.Is(err, huh.ErrUserAborted) {
				fmt.Fprintln(os.Stderr)
				os.Exit(ExitCodeAborted)
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

	// Resolve any "@name" address-book references to concrete addresses.
	payer, err := resolveAlias(f.from)
	if err != nil {
		return rail0.CreatePaymentRequest{}, err
	}
	payee, err := resolveAlias(f.to)
	if err != nil {
		return rail0.CreatePaymentRequest{}, err
	}

	// Resolve token symbol → address on the selected chain.
	tokenAddress, decimals, _, err := lookupTokenAddress(ctx, f.chainID, f.token)
	if err != nil {
		return rail0.CreatePaymentRequest{}, err
	}
	// Reject an over-precise amount now that the token's decimals are known.
	if err := assertAmountPrecision(f.amount, decimals); err != nil {
		return rail0.CreatePaymentRequest{}, err
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
		ChainID:     f.chainID,
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
			opts = append(opts, huh.NewOption(fmt.Sprintf("%s · %s", m.symbol, chainLabel(m.chainID)), strconv.Itoa(i)))
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
		fmt.Fprintf(os.Stderr, "Using the merchant's %s on %s\n", pick.symbol, chainLabel(pick.chainID))
	}

	if f.token == "" {
		f.token = pick.symbol
	}
	if f.chainID == 0 {
		f.chainID = pick.chainID
	}
}

// pickChainFromCatalog fills f.chainID from the public chain catalog
// (GET /blockchains, cached by chainCatalog): a single active chain is taken
// automatically, several are offered as a picker, each labelled with its name.
// Best-effort — on a fetch error or an empty catalog it returns quietly, leaving
// the field to the free-text prompt.
func (f *createPaymentFlags) pickChainFromCatalog() {
	chains := chainCatalog()
	if len(chains) == 0 {
		return
	}
	if len(chains) == 1 {
		f.chainID = chains[0].ChainID
		return
	}
	opts := make([]huh.Option[string], 0, len(chains))
	for i, c := range chains {
		opts = append(opts, huh.NewOption(formatChainLabel(c.Name, c.ChainID), strconv.Itoa(i)))
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
// (GET /tokens?chain_id=&active=true): a single token is taken automatically,
// several are offered as a picker. Needs f.chainID set (the token catalog is
// per-chain), so it's a no-op until the chain is known. Best-effort, like
// pickChainFromCatalog. Active-only for the same reason as lookupTokenAddress —
// this is the create path, so a retired token is not a choice worth offering.
func (f *createPaymentFlags) pickTokenFromCatalog() {
	if f.chainID == 0 {
		return
	}
	tokens, err := activeTokens(context.Background(), f.chainID)
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
		Title(fmt.Sprintf("Select the token on %s", chainLabel(f.chainID))).
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
		return fmt.Sprintf("Select the token to pay on %s", chainLabel(f.chainID))
	default:
		return fmt.Sprintf("Select the chain to pay %s on", f.token)
	}
}
