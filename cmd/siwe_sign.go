package cmd

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

// privateKeyToAddress derives the checksummed Ethereum address from raw key bytes.
func privateKeyToAddress(privKeyBytes []byte) (string, error) {
	privKey, err := crypto.ToECDSA(privKeyBytes)
	if err != nil {
		return "", err
	}
	pub := privKey.Public().(*ecdsa.PublicKey)
	return crypto.PubkeyToAddress(*pub).Hex(), nil
}

// buildSIWEMessage builds a minimal EIP-4361 message.
func buildSIWEMessage(host, address, nonce string) string {
	lines := []string{
		host + " wants you to sign in with your Ethereum account:",
		address,
		"",
		"Sign in to RAIL0",
		"",
		"URI: https://" + host,
		"Version: 1",
		"Chain ID: 1",
		"Nonce: " + nonce,
		"Issued At: " + time.Now().UTC().Format(time.RFC3339),
	}
	return strings.Join(lines, "\n")
}

// personalSign signs a message with the Ethereum personal_sign prefix.
// Returns the 65-byte signature as a 0x-prefixed hex string.
func personalSign(privKeyBytes []byte, message string) (string, error) {
	privKey, err := crypto.ToECDSA(privKeyBytes)
	if err != nil {
		return "", err
	}
	// Ethereum personal_sign: keccak256("\x19Ethereum Signed Message:\n" + len(msg) + msg)
	prefixed := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	hash := crypto.Keccak256([]byte(prefixed))

	sig, err := crypto.Sign(hash, privKey)
	if err != nil {
		return "", err
	}
	// go-ethereum Sign returns [R || S || V] with V in {0,1}; wallets expect V in {27,28}
	sig[64] += 27
	return "0x" + hex.EncodeToString(sig), nil
}

// parseJWTClaims decodes the payload of a JWT without verifying the signature.
// Used only for display purposes (auth status).
func parseJWTClaims(jwt string) (map[string]any, error) {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT")
	}
	// Base64url decode (no padding)
	padded := parts[1]
	switch len(padded) % 4 {
	case 2:
		padded += "=="
	case 3:
		padded += "="
	}
	padded = strings.ReplaceAll(padded, "-", "+")
	padded = strings.ReplaceAll(padded, "_", "/")
	b, err := decodeBase64(padded)
	if err != nil {
		return nil, err
	}
	var claims map[string]any
	if err := json.Unmarshal(b, &claims); err != nil {
		return nil, err
	}
	exp, ok := claims["exp"].(float64)
	if !ok || int64(exp) < time.Now().Unix() {
		return nil, fmt.Errorf("token expired")
	}
	return claims, nil
}

func decodeBase64(s string) ([]byte, error) {
	// Standard base64
	n := new(big.Int)
	n.SetString("0", 16)
	_ = n
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	lookup := make(map[byte]int, 64)
	for i, c := range chars {
		lookup[byte(c)] = i
	}
	// Remove padding
	s = strings.TrimRight(s, "=")
	var bits int
	var buf int
	var out []byte
	for i := 0; i < len(s); i++ {
		v, ok := lookup[s[i]]
		if !ok {
			return nil, fmt.Errorf("invalid base64 char")
		}
		buf = (buf << 6) | v
		bits += 6
		if bits >= 8 {
			bits -= 8
			out = append(out, byte(buf>>bits))
			buf &= (1 << bits) - 1
		}
	}
	return out, nil
}
