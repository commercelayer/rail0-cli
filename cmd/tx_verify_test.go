package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"

	rail0 "github.com/commercelayer/rail0-go"
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

// ── verifySigningPayload ─────────────────────────────────────────────────────
//
// The EIP-1559 guard above had no counterpart for the EIP-712 authorization, which
// is the more dangerous artifact of the two: the token honours it directly, with no
// RAIL0 involvement, for up to message.value. So an endpoint reachable via
// --base-url could answer a refund prepare with the payee's whole balance as `value`
// and have it signed verbatim.

const (
	testPayloadToken = "0x3333333333333333333333333333333333333333"
	// The attacker's address in the hostile cases below.
	testAttacker = "0x4444444444444444444444444444444444444444"
)

func testPayload(overrides func(*rail0.SigningPayload)) *rail0.SigningPayload {
	sp := &rail0.SigningPayload{
		PrimaryType: "ReceiveWithAuthorization",
		Domain: rail0.EIP712Domain{
			Name:              "USD Coin",
			Version:           "2",
			ChainId:           testChainID,
			VerifyingContract: rail0.Address(testPayloadToken),
		},
		Message: rail0.EIP3009Message{
			From:        rail0.Address(testSigner),
			To:          "0x2222222222222222222222222222222222222222",
			Value:       "1000000",
			ValidAfter:  "0",
			ValidBefore: "2000000000",
			Nonce:       rail0.Bytes32("0x" + strings.Repeat("ab", 32)),
		},
	}
	if overrides != nil {
		overrides(sp)
	}
	return sp
}

func testExpectation() payloadExpectation {
	return payloadExpectation{
		signer:         testSigner,
		chainID:        testChainID,
		token:          testPayloadToken,
		valueBaseUnits: "1000000",
	}
}

// The happy path must sail through, or every signing command breaks.
func TestVerifySigningPayloadAcceptsAWellFormedPayload(t *testing.T) {
	if err := verifySigningPayload(testPayload(nil), testExpectation()); err != nil {
		t.Fatalf("rejected a well-formed payload: %v", err)
	}

	// Addresses are compared case-insensitively — the gateway may return checksummed.
	upper := testPayload(func(sp *rail0.SigningPayload) {
		sp.Message.From = rail0.Address(strings.ToUpper(testSigner))
		sp.Domain.VerifyingContract = rail0.Address(strings.ToUpper(testPayloadToken))
	})
	if err := verifySigningPayload(upper, testExpectation()); err != nil {
		t.Errorf("rejected checksum-cased addresses: %v", err)
	}
}

func TestVerifySigningPayloadRejectsHostilePayloads(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*rail0.SigningPayload)
		wantErr string
	}{
		{
			// THE case this exists for: a drain of the signer's whole balance, which
			// the token would honour on its own.
			name: "value inflated far past the amount asked for",
			mutate: func(sp *rail0.SigningPayload) {
				sp.Message.Value = "999999999999"
			},
			wantErr: "999999999999",
		},
		{
			// An authorization from someone else is useless to sign, and is the shape a
			// swapped payload takes.
			name: "from is not the signing key",
			mutate: func(sp *rail0.SigningPayload) {
				sp.Message.From = rail0.Address(testAttacker)
			},
			wantErr: "the signing key is",
		},
		{
			// Minted against a different token the signer happens to hold.
			name: "domain names another token",
			mutate: func(sp *rail0.SigningPayload) {
				sp.Domain.VerifyingContract = rail0.Address(testAttacker)
			},
			wantErr: "token",
		},
		{
			name: "domain names another chain",
			mutate: func(sp *rail0.SigningPayload) {
				sp.Domain.ChainId = testChainID + 1
			},
			wantErr: "chain",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := verifySigningPayload(testPayload(tc.mutate), testExpectation())
			if err == nil {
				t.Fatal("expected a refusal, got nil")
			}
			if !strings.Contains(err.Error(), "refusing to sign") {
				t.Errorf("error should say it is refusing to sign, got: %v", err)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error should mention %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

// Every field is optional: an expectation the caller could not resolve must skip its
// check rather than compare against a zero value and refuse a legitimate payload.
func TestVerifySigningPayloadSkipsUnknownExpectations(t *testing.T) {
	// Only the signer is known — e.g. the token's decimals could not be resolved, so
	// the amount cannot be scaled locally.
	if err := verifySigningPayload(testPayload(nil), payloadExpectation{signer: testSigner}); err != nil {
		t.Errorf("an expectation with only a signer must not refuse: %v", err)
	}
	// A payload the gateway did not include is reported by the caller, not here.
	if err := verifySigningPayload(nil, testExpectation()); err != nil {
		t.Errorf("a nil payload must be left to the caller: %v", err)
	}
}

// The signingContext form takes chain and token off the payment it already fetched.
func TestSigningContextVerifySigningPayloadUsesThePayment(t *testing.T) {
	sc := &signingContext{
		payment: &rail0.Payment{
			Rail0Id: rail0.Bytes32(testRail0ID),
			ChainID: testChainID,
			Token:   rail0.Address(testPayloadToken),
		},
		signer: testSigner,
	}
	if err := sc.verifySigningPayload(testPayload(nil), "1000000"); err != nil {
		t.Fatalf("rejected a well-formed payload: %v", err)
	}
	hostile := testPayload(func(sp *rail0.SigningPayload) { sp.Message.Value = "50000000000" })
	if err := sc.verifySigningPayload(hostile, "1000000"); err == nil {
		t.Error("expected a refusal for an inflated value")
	}
}

const testRail0Contract = "0x2222222222222222222222222222222222222222"

// The transaction and the authorization must both be addressed to the payment's own
// RAIL0 contract when the record names one; an unknown contract skips the check.
func TestVerifyChecksTheRail0Contract(t *testing.T) {
	sc := testContext()
	sc.payment.Rail0Contract = rail0.Address(strings.ToUpper(testRail0Contract))
	if err := sc.verify(unsignedJSON(t, nil)); err != nil {
		t.Fatalf("rejected a transaction to the payment's contract: %v", err)
	}
	other := unsignedJSON(t, map[string]any{"to": "0x3333333333333333333333333333333333333333"})
	if err := sc.verify(other); err == nil || !strings.Contains(err.Error(), "RAIL0 contract") {
		t.Errorf("a transaction to another address must be refused, got %v", err)
	}

	sc.payment.Token = rail0.Address(testPayloadToken)
	if err := sc.verifySigningPayload(testPayload(nil), "1000000"); err != nil {
		t.Fatalf("rejected an authorization paying the payment's contract: %v", err)
	}
	elsewhere := testPayload(func(sp *rail0.SigningPayload) { sp.Message.To = "0x3333333333333333333333333333333333333333" })
	if err := sc.verifySigningPayload(elsewhere, "1000000"); err == nil {
		t.Error("an authorization paying another address must be refused")
	}
}

// The amount check fails CLOSED: an unresolvable token or an amount finer than the
// token's decimals used to return "", which skipped the value check entirely.
func TestExpectedBaseUnitsFailsClosed(t *testing.T) {
	ctx := context.Background()
	seedDecimals(t, testChainID, testPayloadToken, 6)

	if got, err := expectedBaseUnits(ctx, testChainID, testPayloadToken, "1.5"); err != nil || got != "1500000" {
		t.Fatalf("1.5 at 6 decimals: got %q err %v, want 1500000", got, err)
	}
	if _, err := expectedBaseUnits(ctx, testChainID, testPayloadToken, "1.0000001"); err == nil {
		t.Error("an amount finer than the token's decimals must be refused, not skipped")
	}
	if _, err := expectedBaseUnits(ctx, testChainID, "0x9999999999999999999999999999999999999999", "1"); err == nil {
		t.Error("a token whose decimals are unknown must be refused, not skipped")
	}
}

// ── selector and amount (checkCalldataCall) ──────────────────────────────────

// calldataFor builds RAIL0 calldata the way the ABI lays it out: selector, paymentId,
// the six inline Payment words (zeros — their content is not checked here), then any
// trailing arguments. amountUnits, when set, is capture/refund's uint256 amount.
func calldataFor(t *testing.T, op, rail0ID, amountUnits string) string {
	t.Helper()
	sel, ok := rail0Selectors[canonicalOp(op)]
	if !ok {
		t.Fatalf("no selector for %q", op)
	}
	b := strings.Builder{}
	b.WriteString("0x" + sel + strings.TrimPrefix(strings.ToLower(rail0ID), "0x"))
	b.WriteString(strings.Repeat("0", 64*6))
	if amountUnits != "" {
		n, ok := new(big.Int).SetString(amountUnits, 10)
		if !ok {
			t.Fatalf("bad amount %q", amountUnits)
		}
		b.WriteString(fmt.Sprintf("%064x", n))
	}
	if canonicalOp(op) == "refund" {
		b.WriteString(strings.Repeat("0", 64*3)) // v, r, s
	}
	return b.String()
}

// The selectors are pinned against RAIL0.sol's compiled methodIdentifiers
// (rail0/contracts/out/RAIL0.sol/RAIL0.json), not against the table they are computed
// from — a typo in a signature string would otherwise agree with itself.
func TestRail0SelectorsMatchTheABI(t *testing.T) {
	abi := map[string]string{
		"authorize":     "c36c951c",
		"charge":        "1eddc5e5",
		"capture":       "11bf1806",
		"void":          "69438554",
		"release":       "955b0ee2",
		"refund":        "e8ed339d",
		"dispute":       "eebaa720",
		"close_dispute": "6e6923ab",
	}
	if len(rail0Selectors) != len(abi) {
		t.Errorf("%d selectors, want %d", len(rail0Selectors), len(abi))
	}
	for op, want := range abi {
		if got := rail0Selectors[op]; got != want {
			t.Errorf("%s selector = %s, want %s (from the contract ABI)", op, got, want)
		}
	}
}

// The right calldata for each operation passes; the amount-taking ones only with the
// amount the user asked for.
func TestVerifyAcceptsTheRightCallForEachOperation(t *testing.T) {
	for _, op := range []string{"authorize", "charge", "void", "release", "dispute", "close dispute"} {
		sc := testContext()
		sc.op = op
		tx := unsignedJSON(t, map[string]any{"data": calldataFor(t, op, testRail0ID, "")})
		if err := sc.verify(tx); err != nil {
			t.Errorf("%s: rejected its own calldata: %v", op, err)
		}
	}
	for _, op := range []string{"capture", "refund"} {
		sc := testContext()
		sc.op, sc.amountBaseUnits = op, "1500000"
		tx := unsignedJSON(t, map[string]any{"data": calldataFor(t, op, testRail0ID, "1500000")})
		if err := sc.verify(tx); err != nil {
			t.Errorf("%s: rejected its own calldata: %v", op, err)
		}
	}
}

// The swaps checkCalldataPaymentID could not see: the right payment, the wrong
// function. Each would have been signed before.
func TestVerifyRejectsASwappedOperation(t *testing.T) {
	cases := []struct{ asked, served, amount string }{
		{"capture", "void", ""},
		{"void", "capture", "1000000"},
		{"capture", "release", ""},
		{"release", "capture", "1000000"},
		{"dispute", "close_dispute", ""},
		{"close dispute", "dispute", ""},
		{"refund", "capture", "1000000"},
	}
	for _, tc := range cases {
		sc := testContext()
		sc.op, sc.amountBaseUnits = tc.asked, "1000000"
		tx := unsignedJSON(t, map[string]any{"data": calldataFor(t, tc.served, testRail0ID, tc.amount)})
		err := sc.verify(tx)
		if err == nil {
			t.Errorf("asked to %s, served %s: signed it", tc.asked, tc.served)
			continue
		}
		name := rail0FunctionSignatures[canonicalOp(tc.served)]
		name = name[:strings.IndexByte(name, '(')]
		if !strings.Contains(err.Error(), "refusing to sign") || !strings.Contains(err.Error(), name+"()") {
			t.Errorf("asked to %s, served %s: error %q should name %s()", tc.asked, tc.served, err, name)
		}
	}
}

// An unknown selector (the old test fixture's 0xdeadbeef, or approve's 0x095ea7b3)
// is refused and named as hex.
func TestVerifyRejectsAForeignSelector(t *testing.T) {
	sc := testContext()
	sc.op = "void"
	err := sc.verify(unsignedJSON(t, nil)) // 0xdeadbeef + the right id
	if err == nil || !strings.Contains(err.Error(), "0xdeadbeef") {
		t.Errorf("a foreign selector must be refused by name, got %v", err)
	}
}

// Capture and refund: the right function for a different quantity than the user typed.
func TestVerifyRejectsAnAmountOtherThanAskedFor(t *testing.T) {
	for _, op := range []string{"capture", "refund"} {
		sc := testContext()
		sc.op, sc.amountBaseUnits = op, "1000000"
		tx := unsignedJSON(t, map[string]any{"data": calldataFor(t, op, testRail0ID, "999000000")})
		err := sc.verify(tx)
		if err == nil || !strings.Contains(err.Error(), "999000000") || !strings.Contains(err.Error(), "1000000") {
			t.Errorf("%s: an inflated amount must be refused naming both, got %v", op, err)
		}
		// Truncated before the amount word.
		short := calldataFor(t, op, testRail0ID, "")
		if op == "refund" {
			short = short[:len(short)-64*3]
		}
		if err := sc.verify(unsignedJSON(t, map[string]any{"data": short})); err == nil || !strings.Contains(err.Error(), "too short") {
			t.Errorf("%s: calldata with no amount word must be refused, got %v", op, err)
		}
	}
}

// Fail closed: an amount-taking operation with nothing local to compare against, and an
// operation with no known selector, both refuse rather than skip.
func TestCheckCalldataCallFailsClosed(t *testing.T) {
	if err := checkCalldataCall(calldataFor(t, "capture", testRail0ID, "1"), "capture", ""); err == nil {
		t.Error("capture with no expected amount must be refused")
	}
	if err := checkCalldataCall(calldataFor(t, "void", testRail0ID, ""), "approve", ""); err == nil {
		t.Error("an operation with no known selector must be refused")
	}
	if err := checkCalldataCall("0x", "void", ""); err == nil || !strings.Contains(err.Error(), "too short") {
		t.Errorf("empty calldata must be refused as too short, got %v", err)
	}
}
