package cmd

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/ethereum/go-ethereum/common"
	"github.com/rail0/cli/internal/config"
)

const arcUSDC = "0x3600000000000000000000000000000000000000"

// fakeArcNode is a JSON-RPC node for chain 5042002: 1.5 native USDC (18 decimals) and
// 2.5 ERC-20 USDC (6 decimals) for any holder.
func fakeArcNode(t *testing.T) *httptest.Server {
	t.Helper()
	word := func(n int64) string { return hex.EncodeToString(common.LeftPadBytes(big.NewInt(n).Bytes(), 32)) }
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var result string
		switch req.Method {
		case "eth_chainId":
			result = "0x4cef52"
		case "eth_getBalance":
			result = "0x14d1120d7b160000" // 1.5e18
		case "eth_call":
			var call struct{ Input, Data string }
			_ = json.Unmarshal(req.Params[0], &call)
			in := call.Input + call.Data
			switch in[:10] {
			case "0x70a08231":
				result = "0x" + word(2_500_000)
			case "0x313ce567":
				result = "0x" + word(6)
			case "0x95d89b41":
				s := hex.EncodeToString([]byte("USDC"))
				result = "0x" + word(32) + word(4) + s + strings.Repeat("0", 64-len(s))
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
}

func resetKeysBalanceFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		keysBalanceChainID, keysBalanceRPCURL, keysBalanceToken, keysBalanceTokens = 0, "", "", false
	})
}

func seedArcTokens(t *testing.T) {
	t.Helper()
	tokenCatalogMu.Lock()
	previous := tokenCatalogAll
	tokenCatalogAll = append(slices.Clone(previous),
		rail0.Token{ChainID: 5042002, Address: arcUSDC, Decimals: 6, Symbol: "USDC", Active: true})
	tokenCatalogMu.Unlock()
	t.Cleanup(func() {
		tokenCatalogMu.Lock()
		tokenCatalogAll = previous
		tokenCatalogMu.Unlock()
	})
}

func TestKeysBalanceReadsThroughTheGatewaysPublicRPC(t *testing.T) {
	clearConfigEnv(t)
	resetKeysBalanceFlags(t)
	node := fakeArcNode(t)
	defer node.Close()
	seedChainCatalog(t, rail0.Blockchain{ChainID: 5042002, Slug: "arc", Name: "Arc Testnet", NativeSymbol: "USDC",
		PublicRPCURLs: []string{"http://127.0.0.1:1/down", node.URL}})
	seedArcTokens(t)

	keysBalanceChainID = 5042002
	keysBalanceTokens = true
	chain, reader, err := balanceReader(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	rows, err := readBalances(context.Background(), reader, chain, common.HexToAddress("0x1111111111111111111111111111111111111111"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want native + USDC, got %+v", rows)
	}
	if rows[0].Balance != "1500000000000000000" || rows[0].BalanceHuman != "1.5 USDC" || rows[0].Decimals != 18 {
		t.Errorf("native row = %+v", rows[0])
	}
	if rows[1].Balance != "2500000" || rows[1].BalanceHuman != "2.5 USDC" || rows[1].Asset != arcUSDC {
		t.Errorf("token row = %+v", rows[1])
	}
}

func TestKeysBalanceRPCURLNamesTheChainAndMustAgreeWithChain(t *testing.T) {
	clearConfigEnv(t)
	resetKeysBalanceFlags(t)
	node := fakeArcNode(t)
	defer node.Close()
	seedChainCatalog(t)

	keysBalanceRPCURL = node.URL
	keysBalanceToken = arcUSDC // not in any catalog: described on-chain
	chain, reader, err := balanceReader(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if chain.ChainID != 5042002 {
		t.Errorf("chain from eth_chainId = %d", chain.ChainID)
	}
	rows, err := readBalances(context.Background(), reader, chain, common.Address{})
	reader.Close()
	if err != nil || len(rows) != 1 || rows[0].Symbol != "USDC" || rows[0].BalanceHuman != "2.5 USDC" {
		t.Errorf("on-chain token row = %+v, %v", rows, err)
	}

	keysBalanceChainID = 84532
	if _, _, err := balanceReader(context.Background()); err == nil || !isUsageError(err) {
		t.Errorf("a --chain disagreeing with the node must be refused, got %v", err)
	}
}

func TestKeysBalanceNeedsAChain(t *testing.T) {
	clearConfigEnv(t)
	resetKeysBalanceFlags(t)
	if _, _, err := balanceReader(context.Background()); err == nil || !strings.Contains(err.Error(), "--chain is required") {
		t.Errorf("want the chain-required error, got %v", err)
	}
}

func TestKeysBalanceChainWithoutRPCEndpoints(t *testing.T) {
	clearConfigEnv(t)
	resetKeysBalanceFlags(t)
	seedChainCatalog(t, rail0.Blockchain{ChainID: 5042002, Slug: "arc", Name: "Arc Testnet"})
	keysBalanceChainID = 5042002
	if _, _, err := balanceReader(context.Background()); err == nil || !strings.Contains(err.Error(), "--rpc-url") {
		t.Errorf("want a pointer to --rpc-url, got %v", err)
	}
}

func TestBalanceHolder(t *testing.T) {
	clearConfigEnv(t)
	addr := "0x1111111111111111111111111111111111111111"
	if got, key, err := balanceHolder(addr); err != nil || got.Hex() != common.HexToAddress(addr).Hex() || key != "" {
		t.Errorf("address: %v %q %v", got, key, err)
	}
	// An @name that is not a saved key falls back to the address book.
	if err := (&config.Config{Aliases: map[string]string{"shop": addr}}).Save(); err != nil {
		t.Fatal(err)
	}
	if got, _, err := balanceHolder("@shop"); err != nil || got != common.HexToAddress(addr) {
		t.Errorf("alias: %v %v", got, err)
	}
	if _, _, err := balanceHolder("nobody-here"); err == nil {
		t.Error("an unknown name must be an error")
	}
}

func TestPrintBalancesQuiet(t *testing.T) {
	one := captureStdout(t, func() { printBalancesQuiet([]balanceRow{{Symbol: "USDC", Balance: "15"}}) })
	if one != "15\n" {
		t.Errorf("single = %q", one)
	}
	two := captureStdout(t, func() {
		printBalancesQuiet([]balanceRow{{Asset: "native", Symbol: "USDC", Balance: "15"}, {Asset: arcUSDC, Symbol: "USDC", Balance: "7"}})
	})
	if two != "native USDC 15\n"+arcUSDC+" USDC 7\n" {
		t.Errorf("several = %q", two)
	}
}
