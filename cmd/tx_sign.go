package cmd

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// unsignedEIP1559 is the 9-field RLP layout of a type-2 unsigned transaction
// as produced by rail0-api. go-ethereum's UnmarshalBinary expects 12 fields
// (including V, R, S), so we decode manually then reconstruct.
type unsignedEIP1559 struct {
	ChainID    *big.Int
	Nonce      uint64
	GasTipCap  *big.Int
	GasFeeCap  *big.Int
	Gas        uint64
	To         common.Address
	Value      *big.Int
	Data       []byte
	AccessList types.AccessList
}

// signTx signs an RLP-encoded unsigned EIP-1559 transaction (9-field, type 0x02)
// and returns the signed transaction as a 0x-prefixed hex string.
func signTx(unsignedTxHex, privateKeyHex string) (string, error) {
	rawKey := strings.TrimPrefix(privateKeyHex, "0x")
	keyBytes, err := hex.DecodeString(rawKey)
	if err != nil {
		return "", fmt.Errorf("invalid private key hex: %w", err)
	}
	privKey, err := crypto.ToECDSA(keyBytes)
	if err != nil {
		return "", fmt.Errorf("invalid private key: %w", err)
	}

	rawTx := strings.TrimPrefix(unsignedTxHex, "0x")
	txBytes, err := hex.DecodeString(rawTx)
	if err != nil {
		return "", fmt.Errorf("invalid unsigned transaction hex: %w", err)
	}

	// txBytes[0] must be 0x02 (EIP-1559 type prefix); skip it before RLP decode.
	if len(txBytes) < 2 || txBytes[0] != 0x02 {
		return "", fmt.Errorf("expected EIP-1559 type-2 transaction (0x02 prefix)")
	}
	var inner unsignedEIP1559
	if err := rlp.DecodeBytes(txBytes[1:], &inner); err != nil {
		return "", fmt.Errorf("cannot decode unsigned transaction: %w", err)
	}

	to := inner.To
	dtx := &types.DynamicFeeTx{
		ChainID:    inner.ChainID,
		Nonce:      inner.Nonce,
		GasTipCap:  inner.GasTipCap,
		GasFeeCap:  inner.GasFeeCap,
		Gas:        inner.Gas,
		To:         &to,
		Value:      inner.Value,
		Data:       inner.Data,
		AccessList: inner.AccessList,
	}
	tx := types.NewTx(dtx)

	signer := types.LatestSignerForChainID(inner.ChainID)
	signed, err := types.SignTx(tx, signer, privKey)
	if err != nil {
		return "", fmt.Errorf("signing failed: %w", err)
	}

	signedBytes, err := signed.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("cannot encode signed transaction: %w", err)
	}
	return "0x" + hex.EncodeToString(signedBytes), nil
}
