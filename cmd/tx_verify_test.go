package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	rail0 "github.com/rail0/go-sdk"
)

const (
	testRail0ID = "0xa1b2c3d40000000000000000000000000000000000000000000000000000beef"
	testSigner  = "0x1111111111111111111111111111111111111111"
	testChainID = 84532
)

// unsignedJSON builds the gateway's unsigned_transaction shape, with overrides.
func unsignedJSON(t *testing.T, overrides map[string]any) string {
	t.Helper()
	// A well-formed call: 4-byte selector, then paymentId as the first argument.
	calldata := "0xdeadbeef" + strings.TrimPrefix(testRail0ID, "0x")
	fields := map[string]any{
		"chain_id":  testChainID,
		"from":      testSigner,
		"to":        "0x2222222222222222222222222222222222222222",
		"value":     0,
		"data":      calldata,
		"nonce":     7,
		"gas_limit": 200000,
	}
	for k, v := range overrides {
		fields[k] = v
	}
	b, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func testContext() *signingContext {
	return &signingContext{
		payment: &rail0.Payment{Rail0Id: rail0.Bytes32(testRail0ID), ChainID: testChainID},
		signer:  testSigner,
	}
}

// The happy path must not be disturbed: a transaction the real gateway produces has
// to sail through, or the fix breaks every signing command.
func TestVerifyAcceptsAWellFormedGatewayTransaction(t *testing.T) {
	if err := testContext().verify(unsignedJSON(t, nil)); err != nil {
		t.Fatalf("verify rejected a well-formed transaction: %v", err)
	}
	// Case-insensitive on addresses (the gateway may return checksummed).
	upper := unsignedJSON(t, map[string]any{"from": strings.ToUpper(testSigner)})
	if err := testContext().verify(upper); err != nil {
		t.Errorf("verify rejected a checksum-cased from: %v", err)
	}
}

func TestVerifyRejectsHostileTransactions(t *testing.T) {
	cases := []struct {
		name      string
		overrides map[string]any
		wantErr   string
	}{
		{
			name:      "attaches native currency",
			overrides: map[string]any{"value": 1000000000000000000},
			wantErr:   "attaches native currency",
		},
		{
			name:      "native value as a hex quantity",
			overrides: map[string]any{"value": "0xde0b6b3a7640000"},
			wantErr:   "attaches native currency",
		},
		{
			// Must refuse rather than overflow into a plausible small number.
			name:      "native value beyond int64",
			overrides: map[string]any{"value": json.Number("99999999999999999999999999")},
			wantErr:   "attaches native currency",
		},
		{
			name:      "wrong chain",
			overrides: map[string]any{"chain_id": 1},
			wantErr:   "targets chain 1",
		},
		{
			name:      "prepared for a different account",
			overrides: map[string]any{"from": "0x9999999999999999999999999999999999999999"},
			wantErr:   "prepared from",
		},
		{
			name:      "calldata acts on a different payment",
			overrides: map[string]any{"data": "0xdeadbeef" + strings.Repeat("ab", 32)},
			wantErr:   "acts on payment",
		},
		{
			// The profitable swap: approve(spender, MAX) on the token contract. Its
			// first argument is a left-padded address, so it cannot be a rail0_id.
			name: "approve(spender, MAX) on the token",
			overrides: map[string]any{
				"data": "0x095ea7b3" + strings.Repeat("0", 24) + strings.Repeat("9", 40) +
					strings.Repeat("f", 64),
			},
			wantErr: "acts on payment",
		},
		{
			name:      "calldata too short to carry a paymentId",
			overrides: map[string]any{"data": "0xdeadbeef"},
			wantErr:   "too short",
		},
		{
			name:      "no calldata at all",
			overrides: map[string]any{"data": ""},
			wantErr:   "too short",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := testContext().verify(unsignedJSON(t, tc.overrides))
			if err == nil {
				t.Fatalf("verify accepted a hostile transaction (%s)", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to mention %q", err, tc.wantErr)
			}
			if !strings.Contains(err.Error(), "refusing to sign") {
				t.Errorf("error = %q, want it to say it is refusing to sign", err)
			}
		})
	}
}

func TestVerifyRejectsUnparseableJSON(t *testing.T) {
	if err := testContext().verify("{not json"); err == nil {
		t.Fatal("verify accepted an unparseable unsigned transaction")
	}
}

// Zero-value encodings that must be accepted, and everything that must not. Inputs
// are RAW JSON, so a quoted form is how a hex-string quantity arrives.
func TestIsZeroNativeValue(t *testing.T) {
	for _, ok := range []string{"", "null", "0", `"0"`, "0x0", `"0x0"`, `"0X0"`, " 0 "} {
		if !isZeroNativeValue(json.RawMessage(ok)) {
			t.Errorf("isZeroNativeValue(%s) = false, want true", ok)
		}
	}
	for _, bad := range []string{
		"1", `"0x1"`, "00", "0.0", "-0",
		`"0xde0b6b3a7640000"`,        // 1 ETH as a hex string
		"99999999999999999999999999", // beyond any Go integer type
	} {
		if isZeroNativeValue(json.RawMessage(bad)) {
			t.Errorf("isZeroNativeValue(%s) = true, want false (unrecognised must refuse)", bad)
		}
	}
}

// With no rail0_id to compare against there is nothing to assert — a missing id must
// not become a false alarm that blocks a legitimate operation.
func TestCheckCalldataPaymentIDSkipsWithoutAnID(t *testing.T) {
	for _, id := range []string{"", "0x", "not-an-id", "0xabc"} {
		if err := checkCalldataPaymentID("0xdeadbeef", id); err != nil {
			t.Errorf("checkCalldataPaymentID(_, %q) = %v, want nil", id, err)
		}
	}
}

// A nil payment (defensive: verify is only reached with one) must not panic.
func TestVerifyToleratesAMissingPayment(t *testing.T) {
	sc := &signingContext{signer: testSigner}
	if err := sc.verify(unsignedJSON(t, nil)); err != nil {
		t.Errorf("verify with no payment = %v, want nil", err)
	}
	if err := sc.verify(unsignedJSON(t, map[string]any{"value": 1})); err == nil {
		t.Error("verify with no payment still must reject a non-zero value")
	}
}

func TestVerifyErrorNamesTheActualValue(t *testing.T) {
	err := testContext().verify(unsignedJSON(t, map[string]any{"value": 42}))
	if err == nil || !strings.Contains(err.Error(), fmt.Sprint(42)) {
		t.Errorf("error = %v, want it to name the offending value", err)
	}
}
