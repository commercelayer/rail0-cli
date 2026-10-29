package cmd

import (
	"context"
	"encoding/csv"
	"strings"
	"testing"

	rail0 "github.com/commercelayer/rail0-go"
)

// `payments list --csv`: a header, one row per payment, the amount on the payment's own
// scale, and free text defused against spreadsheet formulas.
func TestWritePaymentsCSV(t *testing.T) {
	six := 6
	payments := []rail0.Payment{{
		Rail0Id:      rail0.Bytes32("0x" + strings.Repeat("ab", 32)),
		Status:       "captured",
		Mode:         "authorize",
		Amount:       "1500000",
		Decimals:     &six,
		ChainID:      5042002,
		Description:  "=HYPERLINK(\"x\")",
		Reference:    "ORD-42",
		MerchantNote: "VIP, call first",
		RefundExpiry: 1_800_000_000,
		CreatedAt:    "2026-10-29T10:00:00Z",
	}}
	var b strings.Builder
	if err := writePaymentsCSV(context.Background(), &b, payments); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "\r\n") {
		t.Error("rows should end in CRLF")
	}
	rows, err := csv.NewReader(strings.NewReader(b.String())).ReadAll()
	if err != nil || len(rows) != 2 {
		t.Fatalf("want header + 1 row, got %d rows (%v)", len(rows), err)
	}
	col := map[string]string{}
	for i, h := range rows[0] {
		col[h] = rows[1][i]
	}
	want := map[string]string{
		"status":        "captured",
		"amount":        "1500000",
		"amount_human":  "1.5",
		"chain_id":      "5042002",
		"reference":     "ORD-42",
		"merchant_note": "VIP, call first",
		"description":   "'=HYPERLINK(\"x\")",
		"refund_expiry": "2027-01-15T08:00:00Z",
		"disputed":      "false",
		"token_symbol":  "",
	}
	for k, v := range want {
		if col[k] != v {
			t.Errorf("%s = %q, want %q", k, col[k], v)
		}
	}
}
