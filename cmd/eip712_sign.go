package cmd

import (
	"fmt"
	"math/big"
	"strconv"

	rail0 "github.com/rail0/go-sdk"
)

// signEIP712FromCreate signs the EIP-3009 payload using the CreatePaymentResponse
// returned directly by the create endpoint (no extra GET needed).
func signEIP712FromCreate(resp *rail0.CreatePaymentResponse, privateKeyHex string) (string, error) {
	return signPayload(
		resp.Payment,
		resp.SigningPayload,
		resp.Rail0Contract,
		string(resp.Payment.Amount),
		resp.Mode,
		privateKeyHex,
	)
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
func signRefundPayload(sp *rail0.SigningPayload, payment *rail0.PaymentResponse, amountStr string, privateKeyHex string) (int, string, string, error) {
	privKey, err := rail0.HexToPrivateKey(privateKeyHex)
	if err != nil {
		return 0, "", "", fmt.Errorf("invalid private key: %w", err)
	}

	amount := new(big.Int)
	if _, ok := amount.SetString(amountStr, 10); !ok {
		return 0, "", "", fmt.Errorf("invalid amount: %s", amountStr)
	}

	validBefore, err := strconv.ParseInt(string(sp.Message.ValidBefore), 10, 64)
	if err != nil {
		return 0, "", "", fmt.Errorf("invalid validBefore in signing payload: %w", err)
	}

	tokenDomain := rail0.TokenDomain{
		Name:              sp.Domain.Name,
		Version:           sp.Domain.Version,
		ChainID:           uint64(sp.Domain.ChainId),
		VerifyingContract: sp.Domain.VerifyingContract,
	}

	// Build a PaymentConfig from the PaymentResponse fields.
	pc := rail0.PaymentConfig{
		Payer:               payment.Payer,
		Payee:               payment.Payee,
		Token:               payment.Token,
		Amount:              payment.Amount,
		AuthorizationExpiry: payment.AuthorizationExpiry,
		RefundExpiry:        validBefore, // use validBefore from the payload (matches API nonce)
		FeeBps:              payment.FeeBps,
		FeeReceiver:         payment.FeeReceiver,
	}

	sig, err := rail0.SignReceiveWithAuthorization(rail0.SignPaymentParams{
		PrivateKey:      privKey,
		Payment:         pc,
		Amount:          amount,
		Nonce:           sp.Message.Nonce,
		ContractAddress: sp.Message.To,
		TokenDomain:     tokenDomain,
	})
	if err != nil {
		return 0, "", "", fmt.Errorf("EIP-712 ReceiveWithAuthorization signing failed: %w", err)
	}

	return sig.V, sig.R, sig.S, nil
}
