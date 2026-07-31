package cmd

import (
	"fmt"
	"math/big"

	rail0 "github.com/rail0/go-sdk"
)

// EIP-3009 signing, for every payment operation that needs a payer/payee
// signature (authorize, charge, refund).
//
// ONE rule governs this file: the gateway builds the signing payload, and we
// sign it VERBATIM — typehash from its primaryType, fields from its message,
// domain from its domain. Nothing here re-derives from/to/value/validBefore
// from a payment record, and nothing picks the typehash from a local `mode`.
//
// Why it matters: only the gateway knows which contract VERSION a given payment
// lives on, and the version selects the typehash, the EIP-712 domain and the
// field layout (it builds its client per-payment with contract_version:). A
// client that rebuilds the digest re-imports the contract versioning the
// gateway exists to absorb — so a contract change produces a well-formed
// signature over the WRONG digest, which fails only on-chain, after gas. The
// authorize/charge path used to do exactly that: it chose the typehash from the
// local mode string and discarded sp.Message entirely, so rail0#58 (moving
// authorize/charge to ReceiveWithAuthorization) would have broken it silently.
// See rail0-cli#93.

// Primary types the gateway can ask us to sign. Anything else is a hard error:
// signing the wrong digest looks like success until the token rejects it.
const (
	primaryTypeTransfer = "TransferWithAuthorization"
	primaryTypeReceive  = "ReceiveWithAuthorization"
)

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
	return packSignature(sig), nil
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

	value, err := parsePayloadInt("value", string(sp.Message.Value))
	if err != nil {
		return zero, err
	}
	validAfter, err := parsePayloadInt("validAfter", string(sp.Message.ValidAfter))
	if err != nil {
		return zero, err
	}
	validBefore, err := parsePayloadInt("validBefore", string(sp.Message.ValidBefore))
	if err != nil {
		return zero, err
	}

	tokenDomain := rail0.TokenDomain{
		Name:              sp.Domain.Name,
		Version:           sp.Domain.Version,
		ChainID:           uint64(sp.Domain.ChainId),
		VerifyingContract: sp.Domain.VerifyingContract,
	}

	switch sp.PrimaryType {
	case primaryTypeReceive:
		return rail0.SignReceiveWithAuthorizationRaw(rail0.SignReceiveParams{
			PrivateKey:  privKey,
			TokenDomain: tokenDomain,
			From:        sp.Message.From,
			To:          sp.Message.To,
			Value:       value,
			ValidAfter:  validAfter,
			ValidBefore: validBefore,
			Nonce:       sp.Message.Nonce,
		})
	case primaryTypeTransfer:
		return rail0.SignTransferWithAuthorization(privKey, tokenDomain, rail0.SignTransferParams{
			From:        sp.Message.From,
			To:          sp.Message.To,
			Value:       value,
			ValidAfter:  validAfter,
			ValidBefore: validBefore,
			Nonce:       sp.Message.Nonce,
		})
	default:
		// Never guess. A silent fallback to one of the two typehashes is how you
		// get a valid signature over a digest the token will refuse.
		return zero, fmt.Errorf(
			"unsupported signing payload primaryType %q (expected %s or %s) — this CLI is older than the gateway it is talking to",
			sp.PrimaryType, primaryTypeTransfer, primaryTypeReceive,
		)
	}
}

// packSignature renders {v, r, s} as the 0x + r || s || v hex the gateway takes.
func packSignature(sig rail0.Eip3009Signature) string {
	return "0x" + sig.R[2:] + sig.S[2:] + fmt.Sprintf("%02x", sig.V)
}

// parsePayloadInt reads a decimal string field of the signing payload, naming
// the field in the error so a malformed payload is diagnosable.
func parsePayloadInt(field, raw string) (*big.Int, error) {
	n := new(big.Int)
	if _, ok := n.SetString(raw, 10); !ok {
		return nil, fmt.Errorf("invalid %s in signing payload: %q", field, raw)
	}
	return n, nil
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
