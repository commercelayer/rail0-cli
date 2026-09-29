package cmd

import (
	"context"
	"fmt"
	"sync"
	"time"

	rail0 "github.com/commercelayer/rail0-go"

	"github.com/rail0/cli/internal/ui"
)

// Amount and expiry rendering for the output layer.
//
// The project rule is that amounts are shown as human decimals; the CLI printed base
// units everywhere — `payments get`, the interactive picker, the history timeline —
// and epoch integers for the two expiries. Base units are the dangerous kind of wrong:
// 4050000 reads as four million rather than 4.05, a six-order-of-magnitude error an
// operator can act on. The SDK has had FormatAmount/ToBaseUnits all along (they are
// what the gateway and the TS SDK agree on) and the CLI used neither. (#103)
//
// Raw fields are never replaced, only COMPANIONED: a script parsing `amount` keeps
// working, and `amount_human` is additive.

var (
	tokenCatalogMu  sync.Mutex
	tokenCatalogAll []rail0.Token // nil until fetched; every token on every chain
)

// tokenCatalog returns the gateway's WHOLE token catalogue — every chain, retired tokens
// included — fetched at most once per process, in one `GET /tokens`.
//
// It used to be two caches, each filled per chain: activeTokens (the create path, with
// ?active=true) and tokenDecimals (the rendering and signing paths, unfiltered). A
// `payments create` therefore fetched the same chain's tokens twice, and a `payments
// list` spanning k chains made k calls — all on the gateway's per-IP throttle, which
// /tokens sits on whether or not a session is attached. One unfiltered call answers
// every one of those questions: the gateway's chain_id and active filters are plain
// selects over this same list (Catalog.tokens), so applying them here gives exactly what
// the filtered calls returned.
//
// A failure caches nothing, so a transient error stays retryable within the command.
func tokenCatalog(ctx context.Context) ([]rail0.Token, error) {
	tokenCatalogMu.Lock()
	defer tokenCatalogMu.Unlock()
	if tokenCatalogAll != nil {
		return tokenCatalogAll, nil
	}
	tokens, err := newReadClient().Tokens.List(ctx)
	if err != nil {
		return nil, err
	}
	if tokens == nil {
		tokens = []rail0.Token{} // fetched and empty is not "not fetched"
	}
	tokenCatalogAll = tokens
	return tokens, nil
}

// tokenDecimals returns a token's decimals on a chain, and whether they are known.
//
// Read from the whole catalogue, which includes RETIRED tokens, unlike the create path's
// activeTokens: a settled payment references its token forever, and rendering a
// historical amount in base units because the token was later retired is exactly the
// failure this exists to prevent.
//
// A catalogue failure reports unknown, so the caller falls back to the raw value rather
// than to a wrong scale.
func tokenDecimals(ctx context.Context, chainID int, address string) (int, bool) {
	if chainID == 0 || address == "" {
		return 0, false
	}
	tokens, err := tokenCatalog(ctx)
	if err != nil {
		return 0, false
	}
	key := lowerHex(address)
	for _, t := range tokens {
		if t.ChainID == chainID && lowerHex(string(t.Address)) == key {
			return t.Decimals, true
		}
	}
	return 0, false
}

func lowerHex(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// paymentDecimals returns the decimals that scale a payment's amounts, and whether they
// are known.
//
// The payment's own `decimals` comes first (rail0-gateway#367): the gateway resolves it
// from its chain catalogue, retired tokens and archived contract versions included, so
// it is the same figure the /tokens join produced — without the join. A payment view
// therefore no longer fetches the token catalogue at all. The catalogue stays the
// fallback for a payment whose `decimals` is null (a token the gateway cannot resolve)
// or a gateway older than the field, so nothing renders worse than it did.
func paymentDecimals(ctx context.Context, p *rail0.Payment) (int, bool) {
	if p.Decimals != nil {
		return *p.Decimals, true
	}
	return tokenDecimals(ctx, p.ChainID, string(p.Token))
}

// paymentAmountHuman renders a base-unit amount of payment p's token as a decimal, on
// the scale paymentDecimals resolves, or "" when the amount is empty (the scale is then
// not looked up) or the scale is unknown.
//
// "" rather than the raw value on purpose: the companion field is dropped
// (omitempty) instead of carrying a number at an unstated scale next to one that
// says _human. The raw field is right there for anyone who needs it.
func paymentAmountHuman(ctx context.Context, p *rail0.Payment, base string) string {
	if base == "" {
		return ""
	}
	decimals, ok := paymentDecimals(ctx, p)
	if !ok {
		return ""
	}
	return rail0.FormatAmount(base, decimals)
}

// expiryAt renders a unix expiry as an RFC3339 timestamp, "" for the zero value.
func expiryAt(unix int64) string {
	if unix <= 0 {
		return ""
	}
	return time.Unix(unix, 0).UTC().Format(time.RFC3339)
}

// expiresIn renders the time left until a unix expiry ("2h 5m", or "expired"), ""
// for the zero value. Reuses humanUntil, the same wording `rail0 token` uses for the
// session's remaining life.
func expiresIn(unix int64) string {
	if unix <= 0 {
		return ""
	}
	return humanUntil(time.Until(time.Unix(unix, 0)))
}

// paymentView is a payment plus its human companions.
//
// The embedded pointer marshals its fields inline, so the JSON is the gateway's
// payment with extra keys — not a re-shaped object. Every companion is omitempty, so
// an unknown-decimals token or an unset expiry simply produces no extra key.
type paymentView struct {
	*rail0.Payment
	AmountHuman            string `json:"amount_human,omitempty"`
	CapturableAmountHuman  string `json:"capturable_amount_human,omitempty"`
	RefundableAmountHuman  string `json:"refundable_amount_human,omitempty"`
	AuthorizationExpiryAt  string `json:"authorization_expiry_at,omitempty"`
	AuthorizationExpiresIn string `json:"authorization_expires_in,omitempty"`
	RefundExpiryAt         string `json:"refund_expiry_at,omitempty"`
	RefundExpiresIn        string `json:"refund_expires_in,omitempty"`
}

// transactionView is a transaction plus its amount in token units. The gas figures stay
// raw: they are native currency, whose scale is the chain's, not the token's.
type transactionView struct {
	*rail0.Transaction
	AmountHuman string `json:"amount_human,omitempty"`
}

// humanizeTransactions gives each transaction its amount in token units, read on the
// scale of the payment it belongs to. A transaction carries no chain or token of its own,
// so the payment is fetched once — and not at all when no row has an amount to scale
// (an empty page, or only operations that move none), or under --quiet, which prints
// only ids. If that read fails the rows are returned as they came — raw base units, as
// before — rather than failing a listing over a companion.
func humanizeTransactions(ctx context.Context, client *rail0.Client, paymentID string, txs []rail0.Transaction) []*transactionView {
	var p *rail0.Payment
	if anyAmount(txs) && !ui.IsQuiet() {
		p, _ = client.Payments.Get(ctx, paymentID) // nil on failure: rows stay raw
	}
	return humanizeTransactionsOf(ctx, p, txs)
}

// humanizeTransactionsOf is humanizeTransactions for a caller that already holds the
// payment (nil = unknown, rows stay raw).
func humanizeTransactionsOf(ctx context.Context, p *rail0.Payment, txs []rail0.Transaction) []*transactionView {
	scaled := p != nil && !ui.IsQuiet()
	out := make([]*transactionView, len(txs))
	for i := range txs {
		v := &transactionView{Transaction: &txs[i]}
		if scaled {
			v.AmountHuman = paymentAmountHuman(ctx, p, txs[i].Amount)
		}
		out[i] = v
	}
	return out
}

// anyAmount reports whether any transaction moves an amount worth scaling.
func anyAmount(txs []rail0.Transaction) bool {
	for i := range txs {
		if txs[i].Amount != "" {
			return true
		}
	}
	return false
}

// humanize wraps a payment for output. Nil in, nil out, so a caller can pass a
// response through without a guard. Under --quiet the companions are left out: only the
// id is printed, and the amounts would cost a token-catalogue request for nothing.
func humanize(ctx context.Context, p *rail0.Payment) *paymentView {
	if p == nil {
		return nil
	}
	if ui.IsQuiet() {
		return &paymentView{Payment: p}
	}
	return &paymentView{
		Payment:                p,
		AmountHuman:            paymentAmountHuman(ctx, p, string(p.Amount)),
		CapturableAmountHuman:  paymentAmountHuman(ctx, p, p.CapturableAmount),
		RefundableAmountHuman:  paymentAmountHuman(ctx, p, p.RefundableAmount),
		AuthorizationExpiryAt:  expiryAt(p.AuthorizationExpiry),
		AuthorizationExpiresIn: expiresIn(p.AuthorizationExpiry),
		RefundExpiryAt:         expiryAt(p.RefundExpiry),
		RefundExpiresIn:        expiresIn(p.RefundExpiry),
	}
}

// humanizeAll wraps a list. Each payment carries its own decimals, so a page normally
// costs no catalogue fetch; one covers the whole page, whatever its chains
// (tokenCatalog), when some payment's decimals are null.
func humanizeAll(ctx context.Context, payments []rail0.Payment) []*paymentView {
	out := make([]*paymentView, 0, len(payments))
	for i := range payments {
		out = append(out, humanize(ctx, &payments[i]))
	}
	return out
}

// amountLabel renders an amount for a HUMAN-ONLY surface (the interactive picker),
// where there is no script to keep compatible: the decimal alone when the scale is
// known, and the base units marked as such when it is not.
//
// Marked rather than bare, for the reason in this file's header: an unlabelled
// 4050000 next to a 4.05 in the same list is worse than no figure at all.
func amountLabel(ctx context.Context, p *rail0.Payment, base string) string {
	if human := paymentAmountHuman(ctx, p, base); human != "" {
		return human
	}
	if base == "" {
		return "—"
	}
	return fmt.Sprintf("%s (base units)", base)
}

// amountFilters converts the --min-amount/--max-amount decimals into the base units
// the gateway filters on.
//
// These two were the only user-facing flags that TOOK base units, while --amount
// everywhere else is a decimal — so `--min-amount 1.00` silently meant one
// millionth of a USDC and matched everything. (#103)
//
// Both --chain-id and --token are required with them, and that is not a limitation
// worked around: the conversion needs the token's decimals, and an amount range
// across tokens with different scales does not describe anything. Refusing is better
// than guessing a scale for a filter whose whole job is to be exact.
func amountFilters(ctx context.Context, min, max string, chainID int, token string) (string, string, error) {
	if min == "" && max == "" {
		return "", "", nil
	}
	if chainID == 0 || token == "" {
		return "", "", fmt.Errorf(
			"--min-amount/--max-amount are decimals in token units and need --chain-id and --token to convert; pass both")
	}
	decimals, ok := tokenDecimals(ctx, chainID, token)
	if !ok {
		return "", "", fmt.Errorf(
			"unknown token %s on %s — its decimals are needed to convert --min-amount/--max-amount (see 'rail0 tokens')",
			token, chainLabel(chainID))
	}
	convert := func(flag, v string) (string, error) {
		if v == "" {
			return "", nil
		}
		base, err := rail0.ToBaseUnits(v, decimals)
		if err != nil {
			return "", fmt.Errorf("%s: %w (use a decimal in token units, e.g. 1.00)", flag, err)
		}
		return base, nil
	}
	minBase, err := convert("--min-amount", min)
	if err != nil {
		return "", "", err
	}
	maxBase, err := convert("--max-amount", max)
	if err != nil {
		return "", "", err
	}
	return minBase, maxBase, nil
}
