package cmd

import (
	"testing"

	rail0 "github.com/rail0/go-sdk"
)

func TestPaymentHistory(t *testing.T) {
	blk := 100
	hash := rail0.Bytes32("0xhash")
	p := &rail0.Payment{
		CreatedAt: "2026-06-30T08:00:00Z",
		SignedAt:  "2026-06-30T08:01:00Z",
		Mode:      "authorize",
		Amount:    rail0.Uint256String("1000000"),
		Payer:     rail0.Address("0xBuyer"),
		Payee:     rail0.Address("0xMerchant"),
		Transactions: []rail0.Transaction{
			// Given out of order; the history must come out chronological.
			{Operation: "capture", Status: "confirmed", Amount: rail0.Uint256String("500000"),
				ConfirmedAt: "2026-06-30T08:05:00Z", TransactionHash: &hash, BlockNumber: &blk},
			{Operation: "authorize", Status: "confirmed", ConfirmedAt: "2026-06-30T08:02:00Z"},
		},
	}

	h := paymentHistory(p)

	// created, signed, authorize, capture — in chronological order.
	want := []string{"created", "signed", "authorize", "capture"}
	if len(h) != len(want) {
		t.Fatalf("got %d events, want %d: %v", len(h), len(want), h)
	}
	for i, w := range want {
		got := h[i].(map[string]any)["event"]
		if got != w {
			t.Errorf("event %d: got %v, want %s", i, got, w)
		}
	}

	// created carries mode + amount; signed has no status field.
	created := h[0].(map[string]any)
	if created["mode"] != "authorize" || created["amount"] != "1000000" ||
		created["from"] != "0xBuyer" || created["to"] != "0xMerchant" {
		t.Errorf("created fields: %v", created)
	}
	if _, ok := h[1].(map[string]any)["status"]; ok {
		t.Error("signed event should not carry a status field")
	}

	// capture carries the significant tx fields; the bare authorize omits amount.
	capture := h[3].(map[string]any)
	if capture["status"] != "confirmed" || capture["amount"] != "500000" ||
		capture["tx_hash"] != "0xhash" || capture["block"] != 100 {
		t.Errorf("capture fields: %v", capture)
	}
	if _, ok := h[2].(map[string]any)["amount"]; ok {
		t.Error("authorize with no amount should omit the amount field")
	}
}

func TestPaymentHistoryInfersSignedWithoutTimestamp(t *testing.T) {
	// A payment past "unsigned" but with no signed_at (signed before signed_at
	// was tracked): the signed event still appears, untimed, right after created.
	p := &rail0.Payment{
		CreatedAt:    "2026-06-30T08:00:00Z",
		Status:       "authorized",
		Mode:         "authorize",
		Amount:       rail0.Uint256String("1000000"),
		Transactions: []rail0.Transaction{{Operation: "authorize", Status: "confirmed", ConfirmedAt: "2026-06-30T08:02:00Z"}},
	}

	h := paymentHistory(p)

	want := []string{"created", "signed", "authorize"}
	if len(h) != len(want) {
		t.Fatalf("got %d events, want %d: %v", len(h), len(want), h)
	}
	for i, w := range want {
		if got := h[i].(map[string]any)["event"]; got != w {
			t.Errorf("event %d: got %v, want %s", i, got, w)
		}
	}
	signed := h[1].(map[string]any)
	if at, ok := signed["at"]; !ok || at != nil {
		t.Errorf("inferred signed event should carry at=nil, got %v (present=%v)", at, ok)
	}

	// An unsigned payment shows no signed event.
	un := paymentHistory(&rail0.Payment{CreatedAt: "2026-06-30T08:00:00Z", Status: "unsigned", Mode: "authorize"})
	if len(un) != 1 || un[0].(map[string]any)["event"] != "created" {
		t.Errorf("unsigned payment should have only a created event, got %v", un)
	}
}
