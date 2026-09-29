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
	"github.com/rail0/cli/internal/ui"
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
	paymentsTransactionsOperation, paymentsTransactionsStatuses = "capture", []string{"failed"}
	t.Cleanup(func() { paymentsTransactionsOperation, paymentsTransactionsStatuses = "", nil })

	run(t, paymentsTransactionsCmd, lcRail0ID)
	q := g.last(t, http.MethodGet, "/transactions").query
	if q.Get("operation") != "capture" || q.Get("status") != "failed" {
		t.Errorf("query = %v, want operation=capture&status=failed", q)
	}
}

// Several --status values go out as ONE `status=a,b` (the SDK's Statuses), which the
// gateway matches as any of them — on both status filters. Repeated and
// comma-separated flags land in the same slice (StringSlice), so both forms are
// covered by what reaches the query.
func TestStatusFiltersSendSeveralValues(t *testing.T) {
	g := newRecordingGateway(t)
	paymentsListStatuses = []string{"authorized", " partially_captured", ""}
	paymentsTransactionsStatuses = []string{"submitting", "submitted"}
	t.Cleanup(func() { paymentsListStatuses, paymentsTransactionsStatuses = nil, nil })

	run(t, paymentsListCmd)
	if q := g.last(t, http.MethodGet, "/payments").query; q.Get("status") != "authorized,partially_captured" {
		t.Errorf("payments list query = %v, want status=authorized,partially_captured", q)
	}
	run(t, paymentsTransactionsCmd, lcRail0ID)
	if q := g.last(t, http.MethodGet, "/transactions").query; q.Get("status") != "submitting,submitted" {
		t.Errorf("payments transactions query = %v, want status=submitting,submitted", q)
	}
}

// Each value is checked against ITS vocabulary before any request, and the error names
// the one that is wrong — which the gateway's 400 for the whole filter does not.
func TestStatusFiltersRejectAnUnknownValueLocally(t *testing.T) {
	g := newRecordingGateway(t)
	t.Cleanup(func() { paymentsListStatuses, paymentsTransactionsStatuses = nil, nil })

	paymentsListStatuses = []string{"authorized", "failed"} // a transaction status
	err := paymentsListCmd.RunE(paymentsListCmd, nil)
	if err == nil || !strings.Contains(err.Error(), `"failed"`) {
		t.Errorf("payments list --status authorized,failed: err = %v, want it to name \"failed\"", err)
	}
	paymentsTransactionsStatuses = []string{"captured"} // a payment status
	err = paymentsTransactionsCmd.RunE(paymentsTransactionsCmd, []string{lcRail0ID})
	if err == nil || !strings.Contains(err.Error(), `"captured"`) {
		t.Errorf("payments transactions --status captured: err = %v, want it to name \"captured\"", err)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.reqs) != 0 {
		t.Errorf("%d requests sent; an unknown status must be refused before any", len(g.reqs))
	}
}

// Completion after a comma continues the list and leaves out what is already there.
func TestFixedListCompletionsContinueAfterAComma(t *testing.T) {
	complete := fixedListCompletions("authorized", "captured", "expired")
	got, _ := complete(nil, nil, "authorized,")
	want := []string{"authorized,captured", "authorized,expired"}
	if !slices.Equal(got, want) {
		t.Errorf("completions = %v, want %v", got, want)
	}
	if got, _ := complete(nil, nil, ""); len(got) != 3 {
		t.Errorf("completions for an empty value = %v, want all three", got)
	}
}

// One read: the payment embeds its transactions and carries the chain and token that
// scale the amount, so the single-transaction endpoint (and the payment fetched again
// for the scale) is not called.
func TestPaymentsTransactionReadsOneByIDFromThePayment(t *testing.T) {
	resetTokenCatalog(t)
	g := newRecordingGateway(t)
	g.respond = func(r *http.Request) any {
		switch r.URL.Path {
		case "/payments/" + lcRail0ID:
			return map[string]any{"id": "pay-1", "chain_id": 84532, "token": "0xtok", "transactions": []any{
				map[string]any{"id": "tx-1", "status": "confirmed"},
				map[string]any{"id": "tx-9", "status": "failed", "amount": "1500000"},
			}}
		case "/tokens":
			return []any{map[string]any{"address": "0xtok", "chain_id": 84532, "decimals": 6, "active": true}}
		}
		return nil
	}

	out := captureStdout(t, func() {
		if err := paymentsTransactionCmd.RunE(paymentsTransactionCmd, []string{lcRail0ID, "tx-9"}); err != nil {
			t.Errorf("payments transaction: %v", err)
		}
	})
	if !strings.Contains(out, `"tx-9"`) || !strings.Contains(out, `"1.5"`) {
		t.Errorf("output should be tx-9 with its human amount, got:\n%s", out)
	}
	for _, r := range g.reqs {
		if strings.Contains(r.path, "/transactions") {
			t.Errorf("unexpected request %s %s: the payment already carries the row", r.method, r.path)
		}
	}
}

// An id the payment does not list goes to the endpoint, so the answer is the gateway's
// own — its 404, or a row created after the payment was read.
func TestPaymentsTransactionFallsBackToTheEndpointForAnUnlistedID(t *testing.T) {
	g := newRecordingGateway(t)
	g.respond = func(r *http.Request) any {
		if strings.HasSuffix(r.URL.Path, "/transactions/tx-9") {
			return map[string]any{"id": "tx-9", "status": "failed"}
		}
		return map[string]any{"id": "pay-1"}
	}

	out := captureStdout(t, func() {
		if err := paymentsTransactionCmd.RunE(paymentsTransactionCmd, []string{lcRail0ID, "tx-9"}); err != nil {
			t.Errorf("payments transaction: %v", err)
		}
	})
	g.last(t, http.MethodGet, "/payments/"+lcRail0ID+"/transactions/tx-9")
	if !strings.Contains(out, `"tx-9"`) {
		t.Errorf("output should be the transaction, got:\n%s", out)
	}
}

// --quiet prints only the id, so the human amounts — and the token-catalogue request
// behind them — are skipped.
func TestPaymentsGetQuietSkipsTheTokenCatalogue(t *testing.T) {
	resetTokenCatalog(t)
	g := newRecordingGateway(t)
	g.respond = func(r *http.Request) any {
		if r.URL.Path == "/payments/"+lcRail0ID {
			return map[string]any{"id": "pay-1", "rail0_id": lcRail0ID, "chain_id": 84532, "token": "0xtok", "amount": "1500000"}
		}
		return nil
	}
	ui.SetQuiet(true)
	t.Cleanup(func() { ui.SetQuiet(false) })

	out := captureStdout(t, func() {
		if err := paymentsGetCmd.RunE(paymentsGetCmd, []string{lcRail0ID}); err != nil {
			t.Errorf("payments get: %v", err)
		}
	})
	if strings.TrimSpace(out) != lcRail0ID {
		t.Errorf("quiet output = %q, want just the rail0_id", out)
	}
	for _, r := range g.reqs {
		if r.path == "/tokens" {
			t.Error("--quiet fetched the token catalogue for amounts it never prints")
		}
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
			resetTokenCatalog(t)

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

// A page with no amount to scale costs one call, not two: the payment is read only to
// scale amounts, so rows that carry none skip it.
func TestPaymentsTransactionsSkipThePaymentReadWithoutAmounts(t *testing.T) {
	g := newRecordingGateway(t)
	g.respond = func(r *http.Request) any {
		if strings.HasSuffix(r.URL.Path, "/transactions") {
			return []any{map[string]any{"id": "tx-1", "operation": "void", "status": "confirmed"}}
		}
		t.Errorf("unexpected read of %s", r.URL.Path)
		return map[string]any{}
	}

	captureStdout(t, func() {
		if err := paymentsTransactionsCmd.RunE(paymentsTransactionsCmd, []string{lcRail0ID}); err != nil {
			t.Errorf("payments transactions: %v", err)
		}
	})
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.reqs) != 1 {
		t.Errorf("gateway calls = %d, want 1", len(g.reqs))
	}
}
