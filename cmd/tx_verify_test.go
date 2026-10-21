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

// unsignedJSON builds the gateway's unsigned_transaction shape, with overrides. The
// default calldata is a void of testRail0ID — the operation testContext asks for, since
// the SDK's check never runs without one.
func unsignedJSON(t *testing.T, overrides map[string]any) string {
	t.Helper()
	fields := map[string]any{
		"chain_id":  testChainID,
		"from":      testSigner,
		"to":        "0x2222222222222222222222222222222222222222",
		"value":     0,
		"data":      calldataFor(t, "void", testRail0ID, ""),
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

// testContext is the signingContext ensureSignerFor builds for a void: the payment
// (with its token's decimals, which the SDK scales a typed amount by, and its RAIL0
// contract, which the SDK requires `to` to equal), the signer and the operation.
func testContext() *signingContext {
	decimals := 6
	return &signingContext{
		payment: &rail0.Payment{
			Rail0Id: rail0.Bytes32(testRail0ID), ChainID: testChainID, Decimals: &decimals,
			Rail0Contract: rail0.Address(testRail0Contract),
		},
		signer: testSigner,
		op:     "void",
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
			overrides: map[string]any{"data": calldataFor(t, "void", "0x"+strings.Repeat("ab", 32), "")},
			wantErr:   "acts on payment",
		},
		{
			// The profitable swap: approve(spender, MAX) on the token contract. Named by
			// its foreign selector — the function is checked before the payment id.
			name: "approve(spender, MAX) on the token",
			overrides: map[string]any{
				"data": "0x095ea7b3" + strings.Repeat("0", 24) + strings.Repeat("9", 40) +
					strings.Repeat("f", 64),
			},
			wantErr: "selector 0x095ea7b3",
		},
		{
			name:      "calldata too short to carry a paymentId",
			overrides: map[string]any{"data": "0x" + rail0.Rail0Selectors["void"]},
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
			// The CLI's own wording, not the SDK's "rail0: " prefix.
			if !strings.HasPrefix(err.Error(), "refusing to sign") {
				t.Errorf("error = %q, want it to open with refusing to sign", err)
			}
		})
	}
}

func TestVerifyRejectsUnparseableJSON(t *testing.T) {
	if err := testContext().verify("{not json"); err == nil {
		t.Fatal("verify accepted an unparseable unsigned transaction")
	}
}

// The SDK's check never runs without the payment it is for: a nil payment (defensive —
// verify is only reached with one, from ensureSignerFor) is a refusal, not a pass.
func TestVerifyRefusesWithoutAPayment(t *testing.T) {
	sc := &signingContext{signer: testSigner, op: "void"}
	err := sc.verify(unsignedJSON(t, nil))
	if err == nil || !strings.Contains(err.Error(), "refusing to sign") {
		t.Errorf("verify with no payment = %v, want a refusal", err)
	}
}

func TestVerifyErrorNamesTheActualValue(t *testing.T) {
	err := testContext().verify(unsignedJSON(t, map[string]any{"value": 42}))
	if err == nil || !strings.Contains(err.Error(), fmt.Sprint(42)) {
		t.Errorf("error = %v, want it to name the offending value", err)
	}
}

// ── EIP-712 authorization (signSigningPayload → rail0.SignSigningPayloadFor) ──
//
// The EIP-1559 guard above had no counterpart for the EIP-712 authorization, which
// is the more dangerous artifact of the two: the token honours it directly, with no
// RAIL0 involvement, for up to message.value. So an endpoint reachable via
// --base-url could answer a refund prepare with the payee's whole balance as `value`
// and have it signed verbatim. The check is the SDK's since rail0-go v1.7.0; these
// pin that every CLI signer runs it, fail-closed, in the CLI's wording.

const (
	testPayloadToken = "0x3333333333333333333333333333333333333333"
	// The attacker's address in the hostile cases below.
	testAttacker = "0x4444444444444444444444444444444444444444"
)

// testPayload is an authorization from the test key (testKeyAddress) to the test
// payment's RAIL0 contract for 1.000000 of testPayloadToken, with overrides.
func testPayload(overrides func(*rail0.SigningPayload)) *rail0.SigningPayload {
	sp := &rail0.SigningPayload{
		PrimaryType: "ReceiveWithAuthorization",
		Types:       eip3009Types("ReceiveWithAuthorization"),
		Domain: rail0.EIP712Domain{
			Name:              "USD Coin",
			Version:           "2",
			ChainId:           testChainID,
			VerifyingContract: rail0.Address(testPayloadToken),
		},
		Message: rail0.EIP3009Message{
			From:        rail0.Address(testKeyAddress),
			To:          rail0.Address(testRail0Contract),
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

func testExpectation() rail0.PayloadExpectation {
	return rail0.PayloadExpectation{
		ChainID: testChainID,
		Token:   testPayloadToken,
		To:      testRail0Contract,
		Value:   "1000000",
	}
}

// The happy path must sail through, or every signing command breaks.
func TestSignSigningPayloadAcceptsAWellFormedPayload(t *testing.T) {
	if _, err := signSigningPayload(testPayload(nil), testKey, testExpectation()); err != nil {
		t.Fatalf("rejected a well-formed payload: %v", err)
	}

	// Addresses are compared case-insensitively — the gateway may return checksummed.
	upper := testPayload(func(sp *rail0.SigningPayload) {
		sp.Message.From = rail0.Address(strings.ToLower(testKeyAddress))
		sp.Domain.VerifyingContract = rail0.Address(strings.ToUpper(testPayloadToken))
	})
	if _, err := signSigningPayload(upper, testKey, testExpectation()); err != nil {
		t.Errorf("rejected differently-cased addresses: %v", err)
	}
}

func TestSignSigningPayloadRejectsHostilePayloads(t *testing.T) {
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
		{
			// Pays someone other than the payment's RAIL0 contract.
			name: "message pays another address",
			mutate: func(sp *rail0.SigningPayload) {
				sp.Message.To = rail0.Address(testAttacker)
			},
			wantErr: "RAIL0 contract",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := signSigningPayload(testPayload(tc.mutate), testKey, testExpectation())
			if err == nil {
				t.Fatal("expected a refusal, got nil")
			}
			if !strings.HasPrefix(err.Error(), "refusing to sign") {
				t.Errorf("error should open with 'refusing to sign' (no SDK prefix), got: %v", err)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error should mention %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

// FLIPPED with rail0-go v1.7.0. The CLI's own copy skipped a check whose expectation
// was unknown; the SDK refuses instead, since the value check skipped is exactly the
// drain this guards against. No CLI caller ever relied on the skip (each always
// supplied every field), so a missing one now means something is wrong and is refused.
func TestSignSigningPayloadFailsClosedOnUnknownExpectations(t *testing.T) {
	for name, clear := range map[string]func(*rail0.PayloadExpectation){
		"token":    func(w *rail0.PayloadExpectation) { w.Token = "" },
		"to":       func(w *rail0.PayloadExpectation) { w.To = "" },
		"chain id": func(w *rail0.PayloadExpectation) { w.ChainID = 0 },
		"amount":   func(w *rail0.PayloadExpectation) { w.Value = "" },
	} {
		want := testExpectation()
		clear(&want)
		_, err := signSigningPayload(testPayload(nil), testKey, want)
		if err == nil || !strings.HasPrefix(err.Error(), "refusing to sign") {
			t.Errorf("an expectation without its %s must refuse, got %v", name, err)
		}
	}
	// A payload the gateway did not include is refused too, never signed.
	if _, err := signSigningPayload(nil, testKey, testExpectation()); err == nil {
		t.Error("a nil payload must be refused")
	}
}

const testRail0Contract = "0x2222222222222222222222222222222222222222"

// The signingContext form takes chain, token and contract off the payment it already
// fetched: amount "" is the payment's stored amount (`payments sign`), a typed amount
// is scaled by the payment's decimals (`payments refund`).
func TestSigningContextPayloadExpectationUsesThePayment(t *testing.T) {
	decimals := 6
	sc := &signingContext{
		payment: &rail0.Payment{
			Rail0Id: rail0.Bytes32(testRail0ID), ChainID: testChainID, Amount: "1000000",
			Token: rail0.Address(testPayloadToken), Rail0Contract: rail0.Address(testRail0Contract),
			Decimals: &decimals,
		},
		signer: testKeyAddress,
	}
	for _, amount := range []string{"", "1", "1.00"} {
		want, err := sc.payloadExpectation(amount)
		if err != nil {
			t.Fatalf("amount %q: %v", amount, err)
		}
		if want.Value != "1000000" || want.ChainID != testChainID || !strings.EqualFold(string(want.Signer), testKeyAddress) {
			t.Errorf("amount %q: expectation %+v does not carry the payment's terms", amount, want)
		}
		if _, err := signSigningPayload(testPayload(nil), testKey, want); err != nil {
			t.Errorf("amount %q: rejected a well-formed payload: %v", amount, err)
		}
		hostile := testPayload(func(sp *rail0.SigningPayload) { sp.Message.Value = "50000000000" })
		if _, err := signSigningPayload(hostile, testKey, want); err == nil {
			t.Errorf("amount %q: expected a refusal for an inflated value", amount)
		}
	}

	// Fail-closed, in the CLI's wording: an amount finer than the token allows, and a
	// payment whose decimals are unknown, refuse rather than skip the value check.
	if _, err := sc.payloadExpectation("0.0000001"); err == nil || strings.HasPrefix(err.Error(), "rail0:") {
		t.Errorf("an amount finer than the token's decimals must be refused without the SDK prefix, got %v", err)
	}
	sc.payment.Decimals = nil
	if _, err := sc.payloadExpectation("1"); err == nil {
		t.Error("a typed amount on a payment with unknown decimals must be refused")
	}
}

// The transaction and the authorization must both be addressed to the payment's own
// RAIL0 contract — and, since rail0-go v1.7.0, a payment that names none refuses
// rather than skipping the check (an empty `to` would be a contract creation).
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
	if err := sc.verify(unsignedJSON(t, map[string]any{"to": ""})); err == nil {
		t.Error("a transaction without `to` (a contract creation) must be refused")
	}
	sc.payment.Rail0Contract = ""
	if err := sc.verify(unsignedJSON(t, nil)); err == nil || !strings.Contains(err.Error(), "rail0_contract") {
		t.Errorf("a payment without a rail0_contract must refuse, got %v", err)
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
	sel, ok := rail0.Rail0Selectors[rail0.CanonicalOperation(op)]
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
	if rail0.CanonicalOperation(op) == "refund" {
		b.WriteString(strings.Repeat("0", 64*3)) // v, r, s
	}
	return b.String()
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
		sc.op, sc.amount = op, "1.5"
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
		sc.op, sc.amount = tc.asked, "1"
		tx := unsignedJSON(t, map[string]any{"data": calldataFor(t, tc.served, testRail0ID, tc.amount)})
		err := sc.verify(tx)
		if err == nil {
			t.Errorf("asked to %s, served %s: signed it", tc.asked, tc.served)
			continue
		}
		name := rail0.Rail0FunctionSignatures[rail0.CanonicalOperation(tc.served)]
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
	err := sc.verify(unsignedJSON(t, map[string]any{"data": "0xdeadbeef" + strings.TrimPrefix(testRail0ID, "0x")}))
	if err == nil || !strings.Contains(err.Error(), "0xdeadbeef") {
		t.Errorf("a foreign selector must be refused by name, got %v", err)
	}
}

// Capture and refund: the right function for a different quantity than the user typed.
func TestVerifyRejectsAnAmountOtherThanAskedFor(t *testing.T) {
	for _, op := range []string{"capture", "refund"} {
		sc := testContext()
		sc.op, sc.amount = op, "1"
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

// Fail closed: an amount-taking operation with no --amount to compare against, an
// operation with no known selector, and empty calldata all refuse rather than skip.
func TestVerifyFailsClosed(t *testing.T) {
	sc := testContext()
	sc.op = "capture"
	if err := sc.verify(unsignedJSON(t, map[string]any{"data": calldataFor(t, "capture", testRail0ID, "1")})); err == nil {
		t.Error("capture with no expected amount must be refused")
	}
	sc = testContext()
	sc.op = "approve"
	if err := sc.verify(unsignedJSON(t, nil)); err == nil {
		t.Error("an operation with no known selector must be refused")
	}
	sc = testContext()
	if err := sc.verify(unsignedJSON(t, map[string]any{"data": "0x"})); err == nil || !strings.Contains(err.Error(), "too short") {
		t.Errorf("empty calldata must be refused as too short, got %v", err)
	}
}

// ── checkCreatedPayment ──────────────────────────────────────────────────────

func TestCheckCreatedPayment(t *testing.T) {
	req := rail0.CreatePaymentRequest{
		ChainID: testChainID, Mode: "authorize", Amount: "1.50", Token: testPayloadToken,
		Payer: testSigner, Payee: testRail0Contract,
	}
	good := func() *rail0.Payment {
		return &rail0.Payment{
			ChainID: testChainID, Mode: "authorize", Amount: "1500000",
			Token: rail0.Address(strings.ToUpper(testPayloadToken)), Payer: testSigner,
			Payee: rail0.Address(strings.ToUpper(testRail0Contract)),
		}
	}
	if err := checkCreatedPayment(good(), req, "1500000"); err != nil {
		t.Fatalf("rejected the payment asked for (checksum-cased addresses): %v", err)
	}
	for name, mutate := range map[string]func(*rail0.Payment){
		"chain_id": func(p *rail0.Payment) { p.ChainID = 1 },
		"payer":    func(p *rail0.Payment) { p.Payer = testAttacker },
		"payee":    func(p *rail0.Payment) { p.Payee = testAttacker },
		"token":    func(p *rail0.Payment) { p.Token = testAttacker },
		"mode":     func(p *rail0.Payment) { p.Mode = "charge" },
		"amount":   func(p *rail0.Payment) { p.Amount = "1500001" },
		// Absent is a mismatch, not a skip.
		"payee ": func(p *rail0.Payment) { p.Payee = "" },
	} {
		p := good()
		mutate(p)
		err := checkCreatedPayment(p, req, "1500000")
		if err == nil || !strings.Contains(err.Error(), strings.TrimSpace(name)) {
			t.Errorf("%s: want a refusal naming the field, got %v", name, err)
		}
	}
}
