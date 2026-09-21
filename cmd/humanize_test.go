package cmd

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	rail0 "github.com/commercelayer/rail0-go"
)

// Seeds the per-chain decimals cache so the pure rendering can be tested without a
// gateway. Restored afterwards: the cache is process-wide, and a test that leaves a
// fake token behind would change what a later test sees.
func seedDecimals(t *testing.T, chainID int, address string, decimals int) {
	t.Helper()
	tokenDecimalsMu.Lock()
	previous := tokenDecimalsCache[chainID]
	tokenDecimalsCache[chainID] = map[string]int{lowerHex(address): decimals}
	tokenDecimalsMu.Unlock()
	t.Cleanup(func() {
		tokenDecimalsMu.Lock()
		if previous == nil {
			delete(tokenDecimalsCache, chainID)
		} else {
			tokenDecimalsCache[chainID] = previous
		}
		tokenDecimalsMu.Unlock()
	})
}

const testToken = "0x3600000000000000000000000000000000000000"

func TestHumanAmount(t *testing.T) {
	ctx := context.Background()
	seedDecimals(t, 5042002, testToken, 6)

	// The case the whole file exists for: 4050000 base units is 4.05, not four
	// million.
	if got := humanAmount(ctx, "4050000", 5042002, testToken); got != "4.05" {
		t.Errorf("humanAmount = %q, want 4.05", got)
	}
	// Case-insensitive on the address: the gateway may checksum it.
	if got := humanAmount(ctx, "1000000", 5042002, "0x3600000000000000000000000000000000000000"); got != "1" {
		t.Errorf("humanAmount = %q, want 1", got)
	}
	// Unknown scale → EMPTY, never the raw value: a base-unit integer sitting in a
	// field named _human is worse than no field at all.
	if got := humanAmount(ctx, "4050000", 5042002, "0xUnknownToken"); got != "" {
		t.Errorf("humanAmount for an unknown token = %q, want empty", got)
	}
	if got := humanAmount(ctx, "4050000", 999, testToken); got != "" {
		t.Errorf("humanAmount on an unknown chain = %q, want empty", got)
	}
	if got := humanAmount(ctx, "", 5042002, testToken); got != "" {
		t.Errorf("humanAmount of an empty amount = %q, want empty", got)
	}
}

// The picker has no script to stay compatible with, so an unknown scale is LABELLED
// rather than dropped: the operator is choosing which payment to act on and needs a
// figure, but an unmarked 4050000 next to a 4.05 in the same list would be read as
// the same kind of number.
func TestAmountLabelMarksUnknownScale(t *testing.T) {
	ctx := context.Background()
	seedDecimals(t, 5042002, testToken, 6)

	if got := amountLabel(ctx, "4050000", 5042002, testToken); got != "4.05" {
		t.Errorf("amountLabel = %q, want 4.05", got)
	}
	if got := amountLabel(ctx, "4050000", 5042002, "0xOther"); got != "4050000 (base units)" {
		t.Errorf("amountLabel with unknown decimals = %q", got)
	}
	if got := amountLabel(ctx, "", 5042002, testToken); got != "—" {
		t.Errorf("amountLabel of an empty amount = %q, want an em dash", got)
	}
}

// The raw fields must survive: a script reading `amount` predates the companions and
// must not have to change. Asserted on the marshalled JSON, which is what a script
// actually sees.
func TestHumanizeKeepsRawFieldsAndAddsCompanions(t *testing.T) {
	ctx := context.Background()
	seedDecimals(t, 5042002, testToken, 6)
	expiry := time.Now().Add(2 * time.Hour).Unix()

	view := humanize(ctx, &rail0.Payment{
		Rail0Id:             rail0.Bytes32("0xabc"),
		ChainID:             5042002,
		Token:               rail0.Address(testToken),
		Amount:              rail0.Uint256String("4050000"),
		CapturableAmount:    "1500000",
		RefundableAmount:    "",
		AuthorizationExpiry: expiry,
	})

	var out map[string]any
	b, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if out["amount"] != "4050000" {
		t.Errorf("raw amount = %v, want the base units untouched", out["amount"])
	}
	if out["amount_human"] != "4.05" {
		t.Errorf("amount_human = %v", out["amount_human"])
	}
	if out["capturable_amount_human"] != "1.5" {
		t.Errorf("capturable_amount_human = %v", out["capturable_amount_human"])
	}
	// An empty residual produces NO companion key rather than an empty string.
	if _, present := out["refundable_amount_human"]; present {
		t.Error("refundable_amount_human should be omitted when the raw value is empty")
	}
	if out["authorization_expiry_at"] == nil {
		t.Error("authorization_expiry_at should be an RFC3339 timestamp")
	}
	if out["authorization_expires_in"] == nil {
		t.Error("authorization_expires_in should say how long is left")
	}
	// refund_expiry was never set, so neither companion appears.
	for _, k := range []string{"refund_expiry_at", "refund_expires_in"} {
		if _, present := out[k]; present {
			t.Errorf("%s should be omitted for an unset expiry", k)
		}
	}
}

func TestHumanizeNilIsNil(t *testing.T) {
	if humanize(context.Background(), nil) != nil {
		t.Error("humanize(nil) must be nil so a caller needs no guard")
	}
}

func TestExpiryRendering(t *testing.T) {
	if got := expiryAt(0); got != "" {
		t.Errorf("expiryAt(0) = %q, want empty", got)
	}
	if got := expiresIn(0); got != "" {
		t.Errorf("expiresIn(0) = %q, want empty", got)
	}
	if got := expiryAt(1785000000); got != "2026-07-25T17:20:00Z" {
		t.Errorf("expiryAt = %q", got)
	}
	// A past expiry reads as expired rather than as a negative duration.
	if got := expiresIn(time.Now().Add(-time.Hour).Unix()); got != "expired" {
		t.Errorf("expiresIn for a past expiry = %q, want expired", got)
	}
}

// The flags took base units while every other --amount is a decimal, so
// `--min-amount 1.00` meant one millionth of a USDC and matched everything.
func TestAmountFilters(t *testing.T) {
	ctx := context.Background()
	seedDecimals(t, 5042002, testToken, 6)

	min, max, err := amountFilters(ctx, "1.00", "10", 5042002, testToken)
	if err != nil {
		t.Fatalf("amountFilters: %v", err)
	}
	if min != "1000000" || max != "10000000" {
		t.Errorf("converted to (%q, %q), want (1000000, 10000000)", min, max)
	}

	// No filters → nothing required, nothing converted.
	if min, max, err := amountFilters(ctx, "", "", 0, ""); err != nil || min != "" || max != "" {
		t.Errorf("empty filters returned (%q, %q, %v)", min, max, err)
	}

	// Refusing beats guessing a scale for a filter whose job is to be exact.
	for _, tc := range []struct {
		name    string
		chainID int
		token   string
	}{
		{"no chain", 0, testToken},
		{"no token", 5042002, ""},
		{"neither", 0, ""},
	} {
		if _, _, err := amountFilters(ctx, "1.00", "", tc.chainID, tc.token); err == nil {
			t.Errorf("%s: expected an error, got none", tc.name)
		}
	}

	// A malformed decimal is named as such, with the flag it came from.
	if _, _, err := amountFilters(ctx, "abc", "", 5042002, testToken); err == nil {
		t.Error("a malformed amount must error")
	}
}
