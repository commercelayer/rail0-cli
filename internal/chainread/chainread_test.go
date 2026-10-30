package chainread

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

var (
	holder = common.HexToAddress("0x1111111111111111111111111111111111111111")
	token  = common.HexToAddress("0x3600000000000000000000000000000000000000")
)

// fakeNode answers the JSON-RPC methods the reader uses: eth_chainId,
// eth_getBalance and eth_call (balanceOf / decimals / symbol by selector). calls
// counts requests, to prove which endpoint served them.
func fakeNode(t *testing.T, calls *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params []json.RawMessage
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
			return
		}
		reply := func(result any) {
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
		}
		switch req.Method {
		case "eth_chainId":
			reply("0x4cef52") // 5042002
		case "eth_getBalance":
			reply("0xde0b6b3a7640000") // 1e18
		case "eth_call":
			var call struct {
				To    string `json:"to"`
				Input string `json:"input"`
				Data  string `json:"data"`
			}
			_ = json.Unmarshal(req.Params[0], &call)
			in := call.Input
			if in == "" {
				in = call.Data
			}
			if !strings.EqualFold(call.To, token.Hex()) {
				reply("0x") // no code there
				return
			}
			switch in[:10] {
			case "0x70a08231": // balanceOf(address)
				reply("0x" + word(big.NewInt(2_500_000)))
			case "0x313ce567": // decimals()
				reply("0x" + word(big.NewInt(6)))
			case "0x95d89b41": // symbol()
				s := hex.EncodeToString([]byte("USDC"))
				reply("0x" + word(big.NewInt(32)) + word(big.NewInt(4)) + s + strings.Repeat("0", 64-len(s)))
			default:
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID,
					"error": map[string]any{"code": 3, "message": "execution reverted"}})
			}
		default:
			t.Errorf("unexpected method %s", req.Method)
		}
	}))
}

func word(n *big.Int) string { return hex.EncodeToString(common.LeftPadBytes(n.Bytes(), 32)) }

func TestReadsBalancesAndTokenMeta(t *testing.T) {
	var calls atomic.Int32
	node := fakeNode(t, &calls)
	defer node.Close()
	r, err := New([]string{node.URL}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx := context.Background()

	if id, err := r.ChainID(ctx); err != nil || id.Int64() != 5042002 {
		t.Errorf("ChainID = %v, %v", id, err)
	}
	if b, err := r.NativeBalance(ctx, holder); err != nil || b.String() != "1000000000000000000" {
		t.Errorf("NativeBalance = %v, %v", b, err)
	}
	if b, err := r.TokenBalance(ctx, token, holder); err != nil || b.Int64() != 2_500_000 {
		t.Errorf("TokenBalance = %v, %v", b, err)
	}
	if sym, dec, err := r.TokenMeta(ctx, token); err != nil || sym != "USDC" || dec != 6 {
		t.Errorf("TokenMeta = %q %d %v", sym, dec, err)
	}
}

func TestNoContractIsAnErrorNotZero(t *testing.T) {
	var calls atomic.Int32
	node := fakeNode(t, &calls)
	defer node.Close()
	r, _ := New([]string{node.URL}, time.Second)
	defer r.Close()
	other := common.HexToAddress("0x2222222222222222222222222222222222222222")
	if _, err := r.TokenBalance(context.Background(), other, holder); err == nil || !strings.Contains(err.Error(), "no ERC-20 contract") {
		t.Errorf("want a no-contract error, got %v", err)
	}
}

func TestFailsOverToTheNextEndpoint(t *testing.T) {
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer down.Close()
	var calls atomic.Int32
	node := fakeNode(t, &calls)
	defer node.Close()

	r, _ := New([]string{down.URL, node.URL}, time.Second)
	defer r.Close()
	if b, err := r.NativeBalance(context.Background(), holder); err != nil || b.Sign() == 0 {
		t.Fatalf("failover: %v, %v", b, err)
	}
	// The endpoint that answered is tried first afterwards.
	before := calls.Load()
	if _, err := r.ChainID(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != before+1 {
		t.Errorf("the second read did not go straight to the working endpoint")
	}
}

func TestTimeoutFailsOver(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(500 * time.Millisecond):
		}
	}))
	defer slow.Close()
	var calls atomic.Int32
	node := fakeNode(t, &calls)
	defer node.Close()

	r, _ := New([]string{slow.URL, node.URL}, 100*time.Millisecond)
	defer r.Close()
	if _, err := r.ChainID(context.Background()); err != nil {
		t.Fatalf("a hanging endpoint must hand over: %v", err)
	}
}

func TestRPCErrorIsNotRetriedElsewhere(t *testing.T) {
	var reverted atomic.Int32
	reverting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reverted.Add(1)
		var req struct {
			ID json.RawMessage `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID,
			"error": map[string]any{"code": 3, "message": "execution reverted"}})
	}))
	defer reverting.Close()
	var second atomic.Int32
	node := fakeNode(t, &second)
	defer node.Close()

	r, _ := New([]string{reverting.URL, node.URL}, time.Second)
	defer r.Close()
	// A revert is the chain's answer, the same on every node: returned, not retried.
	if _, err := r.TokenBalance(context.Background(), token, holder); err == nil || !strings.Contains(err.Error(), "reverted") {
		t.Fatalf("want the revert, got %v", err)
	}
	if second.Load() != 0 {
		t.Errorf("a JSON-RPC error was retried on the next endpoint")
	}
}

func TestAllEndpointsDownNamesHostsOnly(t *testing.T) {
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusServiceUnavailable)
	}))
	defer down.Close()
	r, _ := New([]string{down.URL + "/v1/secret-key-123"}, time.Second)
	defer r.Close()
	_, err := r.ChainID(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no RPC endpoint answered") {
		t.Fatalf("want an all-down error, got %v", err)
	}
	if strings.Contains(err.Error(), "secret-key-123") {
		t.Errorf("error leaks the endpoint path: %v", err)
	}
}

func TestNewRejectsAnEmptyList(t *testing.T) {
	if _, err := New([]string{" ", ""}, 0); err == nil {
		t.Error("want an error for no endpoints")
	}
}

func TestHostOf(t *testing.T) {
	for in, want := range map[string]string{
		"https://rpc.example.com/v2/KEY?x=1": "https://rpc.example.com",
		"https://user:pw@rpc.example.com":    "https://rpc.example.com",
		"http://localhost:8545":              "http://localhost:8545",
	} {
		if got := hostOf(in); got != want {
			t.Errorf("hostOf(%q) = %q, want %q", in, got, want)
		}
	}
}
