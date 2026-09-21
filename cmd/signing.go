package cmd

import (
	"fmt"

	rail0 "github.com/commercelayer/rail0-go"
)

// EIP-3009 signing, for every payment operation that needs a payer/payee
// signature (authorize, charge, refund).
//
// ONE rule governs this file: the gateway builds the signing payload, and we
// sign it VERBATIM — typehash from its primaryType, fields from its message,
// domain from its domain. The rule now lives in the SDK: rail0.SignSigningPayload
// (rail0-go#27) is the shared mapper, so the logic that must be identical across
// every client exists in exactly one place. This file only adapts it to the CLI —
// hex key in, packed-or-VRS signature out. The mapper was first written HERE
// (#93, when the SDK had no payload signer) and moved down once it did (#99).
//
// Why the rule matters: only the gateway knows which contract VERSION a given
// payment lives on, and the version selects the typehash, the EIP-712 domain and
// the field layout. A client that rebuilds the digest re-imports the contract
// versioning the gateway exists to absorb — so a contract change produces a
// well-formed signature over the WRONG digest, which fails only on-chain, after
// gas. rail0#58 (authorize/charge → ReceiveWithAuthorization) is exactly that
// change, and neither this CLI nor the SDK needs an edit for it.

// signSigningPayload signs the gateway's signing_payload and returns the packed
// 65-byte signature (0x + r + s + v) that PUT /payments/:id/sign expects.
//
// Works for authorize, charge and refund alike: which typehash to use is the
// payload's own business, not ours.
func signSigningPayload(sp *rail0.SigningPayload, privateKeyHex string) (string, error) {
	sig, err := signSigningPayloadVRS(sp, privateKeyHex)
	if err != nil {
		return "", err
	}
	return rail0.PackSignature(sig), nil
}

// signSigningPayloadVRS is signSigningPayload without the packing, for the
// refund phase-2 call that submits v, r and s separately.
func signSigningPayloadVRS(sp *rail0.SigningPayload, privateKeyHex string) (rail0.Eip3009Signature, error) {
	var zero rail0.Eip3009Signature

	if sp == nil {
		// The gateway builds the payload on create; when it cannot, create itself
		// answers 502 signing_payload_unavailable. So this is a defensive guard.
		return zero, fmt.Errorf("the gateway response did not include a signing_payload")
	}

	privKey, err := rail0.HexToPrivateKey(privateKeyHex)
	if err != nil {
		return zero, fmt.Errorf("invalid private key: %w", err)
	}

	return rail0.SignSigningPayload(privKey, sp)
}

// signRefundPayload signs the phase-1 ReceiveWithAuthorization payload of
// POST /payments/:id/refund/payload and returns v, r, s for phase 2. The key
// must be the payee's.
func signRefundPayload(sp *rail0.SigningPayload, privateKeyHex string) (int, string, string, error) {
	sig, err := signSigningPayloadVRS(sp, privateKeyHex)
	if err != nil {
		return 0, "", "", err
	}
	return sig.V, sig.R, sig.S, nil
}
