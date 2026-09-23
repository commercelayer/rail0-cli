package cmd

// The flags and commands that close the review's functional gaps, run against a
// recording gateway: each test asserts what actually went over the wire, since
// "the flag exists" is not the same as "the gateway receives it".

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rail0/cli/internal/config"
	"github.com/spf13/cobra"
)

type recorded struct {
	method, path string
	query        url.Values
	body         map[string]any
}

// recordingGateway answers every request with `respond` (or an empty list) and keeps
// what it was sent. A live session carrying an account id is seeded, so account-scoped
// commands run without a sign-in.
type recordingGateway struct {
	mu      sync.Mutex
	reqs    []recorded
	respond func(r *http.Request) any
}

func (g *recordingGateway) last(t *testing.T, method, pathSuffix string) recorded {
	t.Helper()
	g.mu.Lock()
	defer g.mu.Unlock()
	for i := len(g.reqs) - 1; i >= 0; i-- {
		if g.reqs[i].method == method && strings.HasSuffix(g.reqs[i].path, pathSuffix) {
			return g.reqs[i]
		}
	}
	t.Fatalf("no %s …%s was sent (got %v)", method, pathSuffix, g.reqs)
	return recorded{}
}

func newRecordingGateway(t *testing.T) *recordingGateway {
	t.Helper()
	isolate(t)
	isolateTokenStore(t)
	g := &recordingGateway{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := recorded{method: r.Method, path: r.URL.Path, query: r.URL.Query()}
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			_ = json.Unmarshal(b, &rec.body)
		}
		g.mu.Lock()
		g.reqs = append(g.reqs, rec)
		g.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		var resp any = []any{}
		if g.respond != nil {
			if v := g.respond(r); v != nil {
				resp = v
			}
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RAIL0_BASE_URL", srv.URL)
	jwt := makeJWT(t, map[string]any{"sub": addressForTestKey(t), "account_id": "acc-1", "exp": time.Now().Add(time.Hour).Unix()})
	if err := saveTokenForTest(jwt); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	return g
}

func run(t *testing.T, cmd *cobra.Command, args ...string) {
	t.Helper()
	_ = captureStdout(t, func() {
		if err := cmd.RunE(cmd, args); err != nil {
			t.Errorf("%s: %v", cmd.CommandPath(), err)
		}
	})
}

func TestPaymentsListSendsOperationAndRail0ID(t *testing.T) {
	g := newRecordingGateway(t)
	paymentsListOperation, paymentsListRail0ID = "dispute", lcRail0ID
	t.Cleanup(func() { paymentsListOperation, paymentsListRail0ID = "", "" })

	run(t, paymentsListCmd)
	q := g.last(t, http.MethodGet, "/payments").query
	if q.Get("operation") != "dispute" || q.Get("rail0_id") != lcRail0ID {
		t.Errorf("query = %v, want operation=dispute and the rail0_id", q)
	}
}

func TestPaymentsTransactionsSendsOperationAndStatus(t *testing.T) {
	g := newRecordingGateway(t)
	paymentsTransactionsOperation, paymentsTransactionsStatus = "capture", "failed"
	t.Cleanup(func() { paymentsTransactionsOperation, paymentsTransactionsStatus = "", "" })

	run(t, paymentsTransactionsCmd, lcRail0ID)
	q := g.last(t, http.MethodGet, "/transactions").query
	if q.Get("operation") != "capture" || q.Get("status") != "failed" {
		t.Errorf("query = %v, want operation=capture&status=failed", q)
	}
}

func TestPaymentsTransactionReadsOneByID(t *testing.T) {
	g := newRecordingGateway(t)
	g.respond = func(*http.Request) any { return map[string]any{"id": "tx-9", "status": "failed"} }

	var out string
	out = captureStdout(t, func() {
		if err := paymentsTransactionCmd.RunE(paymentsTransactionCmd, []string{lcRail0ID, "tx-9"}); err != nil {
			t.Errorf("payments transaction: %v", err)
		}
	})
	g.last(t, http.MethodGet, "/payments/"+lcRail0ID+"/transactions/tx-9")
	if !strings.Contains(out, `"tx-9"`) {
		t.Errorf("output should be the transaction, got:\n%s", out)
	}
}

func TestWalletsListSendsPagingAndTokenActive(t *testing.T) {
	g := newRecordingGateway(t)
	walletsPage, walletsPerPage, walletsTokenActive = 3, 50, "false"
	t.Cleanup(func() { walletsPage, walletsPerPage, walletsTokenActive = 0, 0, "" })

	run(t, walletsCmd)
	q := g.last(t, http.MethodGet, "/accounts/acc-1/wallets").query
	for k, want := range map[string]string{"page": "3", "per_page": "50", "token_active": "false"} {
		if q.Get(k) != want {
			t.Errorf("%s = %q, want %q (query %v)", k, q.Get(k), want, q)
		}
	}

	walletsTokenActive = "nope"
	if err := walletsCmd.RunE(walletsCmd, nil); err == nil {
		t.Error("--token-active must accept only true/false")
	}
}

// `wallets tokens add` resolves a symbol on the chain, and --default is tri-state: an
// omitted flag keeps the holding's current default, which is a different request from
// --default=false. The chain falls back to the configured default.
func TestWalletTokensAdd(t *testing.T) {
	const usdc = "0x036cbd53842c5426634e7929541ec2318f3dcf7e"
	for _, tc := range []struct {
		name       string
		defaultArg string // "" = flag not passed
		wantKey    bool
	}{
		{"default omitted", "", false},
		{"default false", "false", true},
		{"default true", "true", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newRecordingGateway(t)
			g.respond = func(r *http.Request) any {
				if r.URL.Path == "/tokens" {
					return []any{map[string]any{"symbol": "USDC", "address": usdc, "chain_id": 84532, "decimals": 6, "active": true}}
				}
				if r.Method == http.MethodPost {
					return map[string]any{"token_id": "tok-1", "active": true}
				}
				return nil
			}
			if err := (&config.Config{Settings: map[string]string{"chain-id": "84532"}}).Save(); err != nil {
				t.Fatal(err)
			}
			tokenCatalogMu.Lock()
			delete(tokenCatalogCache, 84532)
			tokenCatalogMu.Unlock()

			cmd := walletTokensAddCmd
			walletTokenChainID, walletTokenToken, walletTokenDefault = 0, "USDC", false
			cmd.Flags().Lookup("default").Changed = false
			if tc.defaultArg != "" {
				_ = cmd.Flags().Set("default", tc.defaultArg)
			}
			t.Cleanup(func() {
				walletTokenChainID, walletTokenToken, walletTokenDefault = 0, "", false
				cmd.Flags().Lookup("default").Changed = false
			})

			run(t, cmd, "0xABCD")
			body := g.last(t, http.MethodPost, "/accounts/acc-1/wallets/0xABCD/tokens").body
			if body["token"] != usdc || body["chain_id"] != float64(84532) {
				t.Errorf("body = %v, want the symbol resolved to %s on the configured chain", body, usdc)
			}
			if _, has := body["default"]; has != tc.wantKey {
				t.Errorf("default sent = %v, want %v (body %v)", has, tc.wantKey, body)
			}
			if tc.wantKey && body["default"] != (tc.defaultArg == "true") {
				t.Errorf("default = %v, want %s", body["default"], tc.defaultArg)
			}
		})
	}
}

// enable/disable resolve <token> through the wallet's own holdings and act on the
// token id — never on the symbol the user typed.
func TestWalletTokensDisableActsOnTheResolvedTokenID(t *testing.T) {
	g := newRecordingGateway(t)
	g.respond = func(r *http.Request) any {
		if r.Method == http.MethodGet {
			return []any{map[string]any{"id": "w-1", "address": "0xABCD", "tokens": []any{
				map[string]any{"token_id": "tok-usdc", "active": true, "token": map[string]any{"symbol": "USDC", "chain_id": 84532}},
			}}}
		}
		return map[string]any{"token_id": "tok-usdc", "active": false}
	}
	assumeYes = true
	t.Cleanup(func() { assumeYes = false })

	run(t, walletTokensDisableCmd, "0xabcd", "usdc")
	g.last(t, http.MethodPatch, "/accounts/acc-1/wallets/0xabcd/tokens/tok-usdc/disable")
}

func TestAnalyticsBreakdownCompletesOperation(t *testing.T) {
	complete, ok := analyticsBreakdownCmd.GetFlagCompletionFunc("by")
	if !ok {
		t.Fatal("--by has no completion")
	}
	values, _ := complete(analyticsBreakdownCmd, nil, "")
	if !slices.Contains(values, "operation") {
		t.Errorf("--by completions = %v, want operation among them", values)
	}
}

// Transaction rows carry amount_human beside the raw amount, on the scale of the
// payment they belong to (a transaction has no chain or token of its own).
func TestPaymentsTransactionsHumanizeTheirAmounts(t *testing.T) {
	g := newRecordingGateway(t)
	seedDecimals(t, lcChainID, lcToken, 6)
	g.respond = func(r *http.Request) any {
		if strings.HasSuffix(r.URL.Path, "/transactions") {
			return []any{map[string]any{"id": "tx-1", "operation": "capture", "status": "confirmed", "amount": "2500000"}}
		}
		return map[string]any{"id": "pay-1", "chain_id": lcChainID, "token": lcToken}
	}

	out := captureStdout(t, func() {
		if err := paymentsTransactionsCmd.RunE(paymentsTransactionsCmd, []string{lcRail0ID}); err != nil {
			t.Errorf("payments transactions: %v", err)
		}
	})
	if !strings.Contains(out, `"amount_human": "2.5"`) || !strings.Contains(out, `"amount": "2500000"`) {
		t.Errorf("want the raw amount and its human companion, got:\n%s", out)
	}
}
