package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	rail0 "github.com/commercelayer/rail0-go"
)

func holding(id, symbol, address string, chainID int) rail0.WalletTokenHolding {
	return rail0.WalletTokenHolding{
		TokenID: id,
		Token:   rail0.Token{Symbol: symbol, Address: rail0.Address(address), ChainID: chainID},
	}
}

// A merchant names a holding the way they know it — a symbol or an address — and the
// gateway wants the token's UUID. A symbol held on two chains is never guessed.
func TestMatchHolding(t *testing.T) {
	w := rail0.WalletWithTokens{Address: "0xabc", Tokens: []rail0.WalletTokenHolding{
		holding("tok-base", "USDC", "0x036C", 84532),
		holding("tok-arc", "USDC", "0x3600", 5042002),
		holding("tok-eurc", "EURC", "0x808", 84532),
	}}
	for _, c := range []struct{ arg, want string }{
		{"EURC", "tok-eurc"}, {"eurc", "tok-eurc"}, {"0x3600", "tok-arc"}, {"tok-base", "tok-base"},
	} {
		if got, err := matchHolding(w, c.arg); err != nil || got != c.want {
			t.Errorf("matchHolding(%q) = %q, %v; want %q", c.arg, got, err, c.want)
		}
	}
	if _, err := matchHolding(w, "USDC"); err == nil || !strings.Contains(err.Error(), "--chain") {
		t.Errorf("an ambiguous symbol must ask for --chain, got %v", err)
	}
	if _, err := matchHolding(w, "DAI"); err == nil || !strings.Contains(err.Error(), "wallets tokens add") {
		t.Errorf("a token the wallet never held must point at 'add', got %v", err)
	}
}

// The wallet is found by id or by address in any case, across pages, with the
// chain filter passed through so the symbol match is narrowed server-side.
func TestResolveHoldingTokenIDWalksThePages(t *testing.T) {
	var sawChain string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawChain = r.URL.Query().Get("chain_id")
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "1" {
			w.Header().Set("Link", `<`+r.URL.Path+`?page=2>; rel="next"`)
			_ = json.NewEncoder(w).Encode([]rail0.WalletWithTokens{{ID: "w-1", Address: "0x111"}})
			return
		}
		_ = json.NewEncoder(w).Encode([]rail0.WalletWithTokens{{
			ID: "w-2", Address: "0xAbCd",
			Tokens: []rail0.WalletTokenHolding{holding("tok-base", "USDC", "0x036C", 84532)},
		}})
	}))
	defer srv.Close()
	client := rail0.NewClient(rail0.ClientOptions{BaseURL: srv.URL})

	got, err := resolveHoldingTokenID(context.Background(), client, "acc-1", "0xabcd", "USDC", 84532)
	if err != nil || got != "tok-base" {
		t.Fatalf("resolveHoldingTokenID = %q, %v; want tok-base", got, err)
	}
	if sawChain != "84532" {
		t.Errorf("chain_id = %q, want the --chain-id passed through", sawChain)
	}
	if _, err := resolveHoldingTokenID(context.Background(), client, "acc-1", "0xnope", "USDC", 0); err == nil {
		t.Error("a wallet that is not on the account must be an error")
	}
}
