package cmd

import (
	"errors"
	"fmt"
	"math/big"

	rail0 "github.com/rail0/go-sdk"
)

// signEIP712FromCreate signs the EIP-3009 payload using the Payment returned
// directly by the create endpoint (no extra GET needed). The create response
// carries the embedded signing_payload and rail0_contract needed to sign.
func signEIP712FromCreate(p *rail0.Payment, mode string, privateKeyHex string) (string, error) {
	if p.SigningPayload == nil {
		return "", signingPayloadUnavailable(p.SigningPayloadError)
	}
	pc := rail0.PaymentConfig{
		Amount:              string(p.Amount),
		AuthorizationExpiry: p.AuthorizationExpiry,
		Payee:               p.Payee,
		Payer:               p.Payer,
		RefundExpiry:        p.RefundExpiry,
		Token:               p.Token,
	}
	return signPayload(
		pc,
		*p.SigningPayload,
		p.Rail0Contract,
		string(p.Amount),
		mode,
		privateKeyHex,
	)
}

// signingPayloadUnavailable turns the gateway's reason for a missing EIP-3009
// payload (the payment's signing_payload_error) into an actionable message, so a
// null payload isn't surfaced as an opaque failure.
func signingPayloadUnavailable(reason string) error {
	switch reason {
	case "rpc_unreachable":
		return errors.New("gateway could not build the signing payload: its RPC endpoint for this chain is unreachable — retry shortly, or check the chain's rpc_urls on the gateway")
	case "rpc_error":
		return errors.New("gateway could not build the signing payload: reading the token's EIP-712 domain on-chain failed (the token may not expose name()/version(), or the RPC rejected the call)")
	case "", "error":
		return errors.New("gateway could not build the signing payload for this payment — check that the gateway (--base-url / RAIL0_BASE_URL) is configured for this token and chain")
	default:
		return fmt.Errorf("gateway could not build the signing payload (reason: %s)", reason)
	}
}

// signPayload is the shared signing core.
func signPayload(pc rail0.PaymentConfig, sp rail0.SigningPayload, contractAddress rail0.Address, amountStr string, mode string, privateKeyHex string) (string, error) {
	privKey, err := rail0.HexToPrivateKey(privateKeyHex)
	if err != nil {
		return "", fmt.Errorf("invalid private key: %w", err)
	}

	amount := new(big.Int)
	if _, ok := amount.SetString(amountStr, 10); !ok {
		return "", fmt.Errorf("invalid payment amount: %s", amountStr)
	}

	tokenDomain := rail0.TokenDomain{
		Name:              sp.Domain.Name,
		Version:           sp.Domain.Version,
		ChainID:           uint64(sp.Domain.ChainId),
		VerifyingContract: sp.Domain.VerifyingContract,
	}

	signFn := rail0.SignAuthorize
	if mode == "charge" {
		signFn = rail0.SignCharge
	}

	sig, err := signFn(rail0.SignPaymentParams{
		PrivateKey:      privKey,
		Payment:         pc,
		Amount:          amount,
		Nonce:           sp.Message.Nonce,
		ContractAddress: contractAddress,
		TokenDomain:     tokenDomain,
	})
	if err != nil {
		return "", fmt.Errorf("EIP-712 signing failed: %w", err)
	}

	return "0x" + sig.R[2:] + sig.S[2:] + fmt.Sprintf("%02x", sig.V), nil
}

// signRefundPayload signs the EIP-3009 ReceiveWithAuthorization payload returned
// by phase 1 of POST /payments/:id/refund/payload and returns V, R, S ready for
// phase 2. The signing key must be the payee's private key.
//
// Uses sp.Message fields directly (from/to/value/validBefore/nonce) so the
// digest matches exactly what the API computed — no risk of field confusion
// (e.g. message.from is the payee, not the payer).
func signRefundPayload(sp *rail0.SigningPayload, privateKeyHex string) (int, string, string, error) {
	privKey, err := rail0.HexToPrivateKey(privateKeyHex)
	if err != nil {
		return 0, "", "", fmt.Errorf("invalid private key: %w", err)
	}

	value := new(big.Int)
	if _, ok := value.SetString(string(sp.Message.Value), 10); !ok {
		return 0, "", "", fmt.Errorf("invalid value in signing payload: %s", sp.Message.Value)
	}

	validAfter := new(big.Int)
	if _, ok := validAfter.SetString(string(sp.Message.ValidAfter), 10); !ok {
		return 0, "", "", fmt.Errorf("invalid validAfter in signing payload: %s", sp.Message.ValidAfter)
	}

	validBefore := new(big.Int)
	if _, ok := validBefore.SetString(string(sp.Message.ValidBefore), 10); !ok {
		return 0, "", "", fmt.Errorf("invalid validBefore in signing payload: %s", sp.Message.ValidBefore)
	}

	tokenDomain := rail0.TokenDomain{
		Name:              sp.Domain.Name,
		Version:           sp.Domain.Version,
		ChainID:           uint64(sp.Domain.ChainId),
		VerifyingContract: sp.Domain.VerifyingContract,
	}

	sig, err := rail0.SignReceiveWithAuthorizationRaw(rail0.SignReceiveParams{
		PrivateKey:  privKey,
		TokenDomain: tokenDomain,
		From:        sp.Message.From,
		To:          sp.Message.To,
		Value:       value,
		ValidAfter:  validAfter,
		ValidBefore: validBefore,
		Nonce:       sp.Message.Nonce,
	})
	if err != nil {
		return 0, "", "", fmt.Errorf("EIP-712 ReceiveWithAuthorization signing failed: %w", err)
	}

	return sig.V, sig.R, sig.S, nil
}
