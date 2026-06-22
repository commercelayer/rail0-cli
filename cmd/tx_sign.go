package cmd

import (
	"fmt"

	rail0 "github.com/rail0/go-sdk"
)

// signTx builds and signs the EIP-1559 transaction described by the gateway's
// `unsigned_transaction` JSON (the transaction *fields*, not a serialized tx)
// and returns the signed raw transaction as a 0x-prefixed hex string.
//
// The assembling/hashing/signing logic lives in the SDK
// (rail0.SignTransaction); this wrapper only adapts the hex private key the CLI
// flags carry into the raw bytes the SDK expects.
func signTx(unsignedJSON, privateKeyHex string) (string, error) {
	key, err := rail0.HexToPrivateKey(privateKeyHex)
	if err != nil {
		return "", fmt.Errorf("invalid private key: %w", err)
	}
	return rail0.SignTransaction(unsignedJSON, key)
}
