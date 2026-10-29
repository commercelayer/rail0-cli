package cmd

import (
	"context"
	"encoding/csv"
	"io"
	"strconv"

	rail0 "github.com/commercelayer/rail0-go"
)

// paymentsCSVColumns are the columns `payments list --csv` writes, one row per payment:
// the admin's export (lib/payments-export.ts) plus the two deadlines, so a file made
// from either side reconciles the same way. Amounts come twice — base units, and the
// decimal on the token's scale (empty when the scale is unknown, never the raw value
// under a _human header).
var paymentsCSVColumns = []struct {
	header string
	value  func(ctx context.Context, p *rail0.Payment) string
}{
	{"rail0_id", func(_ context.Context, p *rail0.Payment) string { return string(p.Rail0Id) }},
	{"id", func(_ context.Context, p *rail0.Payment) string { return p.ID }},
	{"status", func(_ context.Context, p *rail0.Payment) string { return p.Status }},
	{"mode", func(_ context.Context, p *rail0.Payment) string { return p.Mode }},
	{"amount", func(_ context.Context, p *rail0.Payment) string { return string(p.Amount) }},
	{"amount_human", func(ctx context.Context, p *rail0.Payment) string {
		return paymentAmountHuman(ctx, p, string(p.Amount))
	}},
	{"token", func(_ context.Context, p *rail0.Payment) string { return string(p.Token) }},
	{"token_symbol", func(ctx context.Context, p *rail0.Payment) string {
		return tokenSymbol(ctx, p.ChainID, string(p.Token))
	}},
	{"chain_id", func(_ context.Context, p *rail0.Payment) string { return intOrEmpty(p.ChainID) }},
	{"payer", func(_ context.Context, p *rail0.Payment) string { return string(p.Payer) }},
	{"payee", func(_ context.Context, p *rail0.Payment) string { return string(p.Payee) }},
	{"capturable_amount", func(_ context.Context, p *rail0.Payment) string { return p.CapturableAmount }},
	{"refundable_amount", func(_ context.Context, p *rail0.Payment) string { return p.RefundableAmount }},
	{"disputed", func(_ context.Context, p *rail0.Payment) string { return strconv.FormatBool(p.Disputed) }},
	{"description", func(_ context.Context, p *rail0.Payment) string { return p.Description }},
	{"reference", func(_ context.Context, p *rail0.Payment) string { return p.Reference }},
	{"reference_origin", func(_ context.Context, p *rail0.Payment) string { return p.ReferenceOrigin }},
	// The payee's own note: the gateway sends it to the payee alone, so it is empty on
	// the rows where you are the payer.
	{"merchant_note", func(_ context.Context, p *rail0.Payment) string { return p.MerchantNote }},
	{"authorization_expiry", func(_ context.Context, p *rail0.Payment) string { return expiryAt(p.AuthorizationExpiry) }},
	{"refund_expiry", func(_ context.Context, p *rail0.Payment) string { return expiryAt(p.RefundExpiry) }},
	{"created_at", func(_ context.Context, p *rail0.Payment) string { return p.CreatedAt }},
}

// writePaymentsCSV writes the header and one row per payment (RFC 4180, CRLF endings
// as spreadsheets expect).
func writePaymentsCSV(ctx context.Context, w io.Writer, payments []rail0.Payment) error {
	out := csv.NewWriter(w)
	out.UseCRLF = true
	header := make([]string, len(paymentsCSVColumns))
	for i, c := range paymentsCSVColumns {
		header[i] = c.header
	}
	if err := out.Write(header); err != nil {
		return err
	}
	row := make([]string, len(paymentsCSVColumns))
	for i := range payments {
		for j, c := range paymentsCSVColumns {
			row[j] = csvSafe(c.value(ctx, &payments[i]))
		}
		if err := out.Write(row); err != nil {
			return err
		}
	}
	out.Flush()
	return out.Error()
}

// csvSafe defuses a cell a spreadsheet would run as a formula. description, reference
// and the note are free text a counterparty or an integration wrote, so a value
// starting with = + - @ (or a tab / CR) is prefixed with a quote — the OWASP rule. An
// amount is never negative, so no number is touched.
func csvSafe(v string) string {
	if v == "" {
		return v
	}
	switch v[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + v
	}
	return v
}

// tokenSymbol is a token's symbol on a chain from the (cached) catalogue, "" when the
// catalogue does not list it.
func tokenSymbol(ctx context.Context, chainID int, address string) string {
	if chainID == 0 || address == "" {
		return ""
	}
	tokens, err := tokenCatalog(ctx)
	if err != nil {
		return ""
	}
	key := lowerHex(address)
	for _, t := range tokens {
		if t.ChainID == chainID && lowerHex(string(t.Address)) == key {
			return t.Symbol
		}
	}
	return ""
}

func intOrEmpty(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}
