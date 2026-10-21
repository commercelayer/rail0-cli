package cmd

import (
	"strings"
	"testing"

	rail0 "github.com/commercelayer/rail0-go"
)

// The signing mapper was the least-tested code in the CLI, and it is the code
// where a mistake is invisible: a wrong digest still yields a well-formed
// signature, which the token refuses only after gas is spent. These tests pin
// the rule the mapper exists to follow — the gateway's payload is signed
// VERBATIM, and its primaryType is what picks the typehash (#93).
//
// The key is a well-known test key (Anvil account #0). Its address is
// 0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266.
const testKey = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
const testKeyAddress = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"

// eip3009Types is the `types` envelope the gateway sends with every signing payload.
// rail0-go v1.7.1 checks it (exact EIP-3009 struct, exact domain, nothing else), so a
// fixture without it is refused before any of the mapping under test runs.
func eip3009Types(primaryType string) map[string]any {
	field := func(name, typ string) any { return map[string]any{"name": name, "type": typ} }
	return map[string]any{
		"EIP712Domain": []any{
			field("name", "string"), field("version", "string"),
			field("chainId", "uint256"), field("verifyingContract", "address"),
		},
		primaryType: []any{
			field("from", "address"), field("to", "address"), field("value", "uint256"),
			field("validAfter", "uint256"), field("validBefore", "uint256"), field("nonce", "bytes32"),
		},
	}
}

func payload(primaryType string) *rail0.SigningPayload {
	return &rail0.SigningPayload{
		PrimaryType: primaryType,
		Types:       eip3009Types(primaryType),
		Domain: rail0.EIP712Domain{
			Name:              "USD Coin",
			Version:           "2",
			ChainId:           84532,
			VerifyingContract: rail0.Address("0x036CbD53842c5426634e7929541eC2318f3dCF7e"),
		},
		Message: rail0.EIP3009Message{
			From:        rail0.Address(testKeyAddress),
			To:          rail0.Address("0x13a46eDDBE6105f5c055A2C8729b773C9C7BBa1F"),
			Value:       rail0.Uint256String("2600000"),
			ValidAfter:  rail0.Uint256String("0"),
			ValidBefore: rail0.Uint256String("1800000000"),
			Nonce:       rail0.Bytes32("0x" + strings.Repeat("ab", 32)),
		},
	}
}

// wantFor is the expectation a payload satisfies: every signer now verifies before it
// signs (rail0.SignSigningPayloadFor), so the mapper tests below pass the payload's
// own terms as what was asked for and pin only the verbatim mapping.
func wantFor(p *rail0.SigningPayload) rail0.PayloadExpectation {
	return rail0.PayloadExpectation{
		ChainID: p.Domain.ChainId,
		Token:   p.Domain.VerifyingContract,
		To:      p.Message.To,
		Value:   p.Message.Value,
	}
}

// Both primary types the gateway can emit must sign, and produce DIFFERENT
// signatures: the typehash is part of the digest, so if the two ever matched it
// would mean the primaryType was being ignored.
func TestSignSigningPayloadHonoursPrimaryType(t *testing.T) {
	transfer, err := signSigningPayload(payload(rail0.PrimaryTypeTransfer), testKey, wantFor(payload(rail0.PrimaryTypeTransfer)))
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	receive, err := signSigningPayload(payload(rail0.PrimaryTypeReceive), testKey, wantFor(payload(rail0.PrimaryTypeReceive)))
	if err != nil {
		t.Fatalf("receive: %v", err)
	}

	if transfer == receive {
		t.Error("the two primary types produced the same signature — the typehash is being ignored")
	}
	for name, sig := range map[string]string{"transfer": transfer, "receive": receive} {
		// 0x + r(32) + s(32) + v(1) = 132 characters.
		if len(sig) != 132 {
			t.Errorf("%s signature has length %d, want 132: %q", name, len(sig), sig)
		}
		if !strings.HasPrefix(sig, "0x") {
			t.Errorf("%s signature is not 0x-prefixed: %q", name, sig)
		}
		if tail := sig[130:]; tail != "1b" && tail != "1c" {
			t.Errorf("%s signature ends in v=%q, want 1b or 1c", name, tail)
		}
	}
}

// An unknown primaryType must be a HARD error. Falling back to one of the two
// known typehashes is how a CLI older than its gateway signs the wrong digest and
// reports success — the failure then surfaces on-chain, blamed on the key.
func TestSignSigningPayloadRefusesAnUnknownPrimaryType(t *testing.T) {
	_, err := signSigningPayload(payload("SomeFutureAuthorization"), testKey, wantFor(payload(rail0.PrimaryTypeTransfer)))
	if err == nil {
		t.Fatal("an unknown primaryType must not be signed")
	}
	if !strings.Contains(err.Error(), "SomeFutureAuthorization") {
		t.Errorf("the error should name the offending primaryType, got: %v", err)
	}
	// It must also say WHY, so the reader reaches for an upgrade rather than a key.
	if !strings.Contains(err.Error(), "older than the gateway") {
		t.Errorf("the error should point at the version skew, got: %v", err)
	}
}

// The whole point of the rule: what gets signed comes from the payload, not from
// a payment record. Changing one payload field must change the signature. (`from` is
// not among them: a payload from another address than the key is refused before
// signing — see TestSignSigningPayloadRejectsHostilePayloads.)
func TestSignSigningPayloadReadsTheMessageFields(t *testing.T) {
	base, err := signSigningPayload(payload(rail0.PrimaryTypeTransfer), testKey, wantFor(payload(rail0.PrimaryTypeTransfer)))
	if err != nil {
		t.Fatalf("base: %v", err)
	}

	for _, tc := range []struct {
		field  string
		mutate func(*rail0.SigningPayload)
	}{
		{"value", func(p *rail0.SigningPayload) { p.Message.Value = "2600001" }},
		{"to", func(p *rail0.SigningPayload) {
			p.Message.To = rail0.Address("0x0000000000000000000000000000000000000001")
		}},
		{"validBefore", func(p *rail0.SigningPayload) { p.Message.ValidBefore = "1900000000" }},
		{"validAfter", func(p *rail0.SigningPayload) { p.Message.ValidAfter = "1" }},
		{"nonce", func(p *rail0.SigningPayload) {
			p.Message.Nonce = rail0.Bytes32("0x" + strings.Repeat("cd", 32))
		}},
		{"domain.verifyingContract", func(p *rail0.SigningPayload) {
			p.Domain.VerifyingContract = rail0.Address("0x0000000000000000000000000000000000000003")
		}},
		{"domain.chainId", func(p *rail0.SigningPayload) { p.Domain.ChainId = 1 }},
	} {
		p := payload(rail0.PrimaryTypeTransfer)
		tc.mutate(p)
		got, err := signSigningPayload(p, testKey, wantFor(p))
		if err != nil {
			t.Fatalf("%s: %v", tc.field, err)
		}
		if got == base {
			t.Errorf("changing %s did not change the signature — that field is not being read from the payload", tc.field)
		}
	}
}

// A malformed numeric field must be named, not swallowed: these arrive from the
// wire, and "invalid value" without the field is a debugging dead end.
func TestSignSigningPayloadNamesAMalformedField(t *testing.T) {
	for _, tc := range []struct {
		field  string
		mutate func(*rail0.SigningPayload)
	}{
		{"value", func(p *rail0.SigningPayload) { p.Message.Value = "not-a-number" }},
		{"validAfter", func(p *rail0.SigningPayload) { p.Message.ValidAfter = "" }},
		{"validBefore", func(p *rail0.SigningPayload) { p.Message.ValidBefore = "0x10" }},
	} {
		p := payload(rail0.PrimaryTypeTransfer)
		tc.mutate(p)
		// Expect the UNMUTATED terms, so the refusal is the malformed field itself.
		_, err := signSigningPayload(p, testKey, wantFor(payload(rail0.PrimaryTypeTransfer)))
		if err == nil {
			t.Errorf("%s: a malformed field must be rejected", tc.field)
			continue
		}
		if !strings.Contains(err.Error(), tc.field) {
			t.Errorf("the error should name %q, got: %v", tc.field, err)
		}
	}
}

func TestSignSigningPayloadRejectsAMissingPayload(t *testing.T) {
	if _, err := signSigningPayload(nil, testKey, wantFor(payload(rail0.PrimaryTypeTransfer))); err == nil {
		t.Error("a nil signing payload must be rejected")
	}
	if _, err := signSigningPayload(payload(rail0.PrimaryTypeTransfer), "not-a-key", wantFor(payload(rail0.PrimaryTypeTransfer))); err == nil {
		t.Error("an invalid private key must be rejected")
	}
}

// signRefundPayload is now the same core, returning v/r/s for the phase-2 call.
// It must agree with the packed form, or the two submit paths would disagree.
func TestSignRefundPayloadAgreesWithThePackedForm(t *testing.T) {
	p := payload(rail0.PrimaryTypeReceive)

	packed, err := signSigningPayload(p, testKey, wantFor(p))
	if err != nil {
		t.Fatalf("packed: %v", err)
	}
	v, r, s, err := signRefundPayload(p, testKey, wantFor(p))
	if err != nil {
		t.Fatalf("vrs: %v", err)
	}

	if got := rail0.PackSignature(rail0.Eip3009Signature{V: v, R: r, S: s}); got != packed {
		t.Errorf("packed %q != re-packed v/r/s %q", packed, got)
	}
}
