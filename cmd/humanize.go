package cmd

import (
	"context"
	"fmt"
	"sync"
	"time"

	rail0 "github.com/commercelayer/rail0-go"
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
	tokenDecimalsMu    sync.Mutex
	tokenDecimalsCache = map[int]map[string]int{} // chainID → lowercased address → decimals
)

// tokenDecimals returns a token's decimals on a chain, and whether they are known.
//
// The catalog is fetched at most once per chain per process and includes RETIRED
// tokens, unlike the create path's activeTokens: a settled payment references its
// token forever, and rendering a historical amount in base units because the token
// was later retired is exactly the failure this exists to prevent.
//
// A catalog failure caches nothing and reports unknown, so a transient error stays
// retryable and the caller falls back to the raw value rather than to a wrong scale.
func tokenDecimals(ctx context.Context, chainID int, address string) (int, bool) {
	if chainID == 0 || address == "" {
		return 0, false
	}
	key := lowerHex(address)

	tokenDecimalsMu.Lock()
	defer tokenDecimalsMu.Unlock()
	if byAddr, ok := tokenDecimalsCache[chainID]; ok {
		d, found := byAddr[key]
		return d, found
	}
	tokens, err := newReadClient().Tokens.List(ctx, rail0.ListTokensParams{ChainID: chainID})
	if err != nil {
		return 0, false
	}
	byAddr := make(map[string]int, len(tokens))
	for _, t := range tokens {
		byAddr[lowerHex(string(t.Address))] = t.Decimals
	}
	tokenDecimalsCache[chainID] = byAddr
	d, found := byAddr[key]
	return d, found
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

// humanAmount renders a base-unit amount as a decimal, or "" when the token's
// decimals are unknown or the amount is empty.
//
// "" rather than the raw value on purpose: the companion field is dropped
// (omitempty) instead of carrying a number at an unstated scale next to one that
// says _human. The raw field is right there for anyone who needs it.
func humanAmount(ctx context.Context, base string, chainID int, token string) string {
	if base == "" {
		return ""
	}
	decimals, ok := tokenDecimals(ctx, chainID, token)
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
// (an empty page, or only operations that move none). If that read fails the rows are
// returned as they came — raw base units, as before — rather than failing a listing
// over a companion.
func humanizeTransactions(ctx context.Context, client *rail0.Client, paymentID string, txs []rail0.Transaction) []*transactionView {
	chainID, token := 0, ""
	if anyAmount(txs) {
		if p, err := client.Payments.Get(ctx, paymentID); err == nil && p != nil {
			chainID, token = p.ChainID, string(p.Token)
		}
	}
	out := make([]*transactionView, len(txs))
	for i := range txs {
		v := &transactionView{Transaction: &txs[i]}
		if chainID != 0 && txs[i].Amount != "" {
			v.AmountHuman = humanAmount(ctx, txs[i].Amount, chainID, token)
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
// response through without a guard.
func humanize(ctx context.Context, p *rail0.Payment) *paymentView {
	if p == nil {
		return nil
	}
	chainID, token := p.ChainID, string(p.Token)
	return &paymentView{
		Payment:                p,
		AmountHuman:            humanAmount(ctx, string(p.Amount), chainID, token),
		CapturableAmountHuman:  humanAmount(ctx, p.CapturableAmount, chainID, token),
		RefundableAmountHuman:  humanAmount(ctx, p.RefundableAmount, chainID, token),
		AuthorizationExpiryAt:  expiryAt(p.AuthorizationExpiry),
		AuthorizationExpiresIn: expiresIn(p.AuthorizationExpiry),
		RefundExpiryAt:         expiryAt(p.RefundExpiry),
		RefundExpiresIn:        expiresIn(p.RefundExpiry),
	}
}

// humanizeAll wraps a list. One catalog fetch per chain covers the whole page, since
// tokenDecimals caches per chain.
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
func amountLabel(ctx context.Context, base string, chainID int, token string) string {
	if human := humanAmount(ctx, base, chainID, token); human != "" {
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
