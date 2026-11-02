package cmd

// Payout commands against a fake gateway: the details attestation (never sent without
// consent), the send flow (prepare → verify → sign → submit, refusing a tampered
// transfer before submit) and the local checks that run before any request.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/spf13/cobra"
)

const (
	poDestID  = "11111111-1111-4111-8111-111111111111"
	poID      = "22222222-2222-4222-8222-222222222222"
	poDeposit = "0x1234567890abcdef1234567890abcdef12345678"
	poToken   = "0x3600000000000000000000000000000000000000"
	poChain   = 5042002
)

// payoutGateway serves one destination, the token catalog (USDC, 6 decimals) and the
// payout routes; prepared is what POST /payouts/prepare and GET /payouts/{id} answer.
type payoutGateway struct {
	mu       sync.Mutex
	dest     map[string]any
	prepared map[string]any
	requests []string
	bodies   map[string]map[string]any // "METHOD path" → last JSON body
	queries  map[string]string         // "METHOD path" → last raw query
	// polls, once the payout was submitted or resent, is what successive GET
	// /payouts/{id} answer (the last one repeats): the payout as a --wait sees it.
	polls     []map[string]any
	submitted bool
	// reserves is what GET /payout_destinations/{id}/reserves answers.
	reserves map[string]any
}

// pollsAfterSubmit makes GET /payouts/{id} answer the submitted payout with each
// status in turn once a submit or resend went out; extra fields go on the last one.
func (g *payoutGateway) pollsAfterSubmit(statuses []string, last map[string]any) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i, st := range statuses {
		p := map[string]any{}
		for k, v := range g.prepared {
			p[k] = v
		}
		p["status"], p["signed"], p["transaction_hash"] = st, true, "0x"+strings.Repeat("ab", 32)
		if i == len(statuses)-1 {
			for k, v := range last {
				p[k] = v
			}
		}
		g.polls = append(g.polls, p)
	}
}

func (g *payoutGateway) count(methodPath string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	n := 0
	for _, r := range g.requests {
		if r == methodPath {
			n++
		}
	}
	return n
}

// all is every request the fake received, in order.
func (g *payoutGateway) all() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.requests...)
}

func (g *payoutGateway) sent(methodPath string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, r := range g.requests {
		if r == methodPath {
			return true
		}
	}
	return false
}

func (g *payoutGateway) query(methodPath string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.queries[methodPath]
}

func (g *payoutGateway) body(methodPath string) map[string]any {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.bodies[methodPath]
}

func newPayoutGateway(t *testing.T) *payoutGateway {
	t.Helper()
	isolate(t)
	isolateTokenStore(t)
	t.Setenv("RAIL0_PRIVATE_KEY", "")
	signer := addressForTestKey(t)
	g := &payoutGateway{
		bodies: map[string]map[string]any{},
		dest: map[string]any{
			"id": poDestID, "chain_id": poChain, "token": poToken, "token_symbol": "USDC",
			"address": poDeposit, "status": "verified", "min_amount": "10000000",
		},
		queries: map[string]string{},
		reserves: map[string]any{
			"destination_id": poDestID, "chain_id": poChain, "token": poToken, "token_symbol": "USDC", "decimals": 6,
			"balance": "100000000", "balance_formatted": "100", "refund_reserve": "5000000", "refund_reserve_formatted": "5",
			"refundable_payments": 3, "native_gas": true, "gas_reserve": "150000", "gas_reserve_formatted": "0.15",
			"gas_refunds": 3, "open_authorizations": 1, "base_captures": 5,
			"gas_allowance": "20000", "gas_allowance_formatted": "0.02", "available": "94830000", "available_formatted": "94.83",
		},
	}
	g.prepared = map[string]any{
		"id": poID, "destination_id": poDestID, "purpose": "payout", "status": "pending",
		"chain_id": poChain, "token": poToken, "token_symbol": "USDC", "address": poDeposit,
		"amount": "25000000", "amount_formatted": "25", "signed": false,
		"unsigned_transaction": payoutTransfer(t, signer, poDeposit, "25000000"),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		g.mu.Lock()
		g.requests = append(g.requests, key)
		g.bodies[key] = body
		g.queries[key] = r.URL.RawQuery
		dest, prepared, reserves := g.dest, g.prepared, g.reserves
		if r.Method == http.MethodPost && (r.URL.Path == "/payouts/"+poID+"/submit" || r.URL.Path == "/payouts/"+poID+"/resend") {
			g.submitted = true
		} else if r.Method == http.MethodGet && r.URL.Path == "/payouts/"+poID && g.submitted && len(g.polls) > 0 {
			prepared = g.polls[0]
			if len(g.polls) > 1 {
				g.polls = g.polls[1:]
			}
		}
		g.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/tokens":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"symbol": "USDC", "address": poToken, "chain_id": poChain, "decimals": 6, "active": true,
			}})
		case r.URL.Path == "/payout_destinations/"+poDestID+"/reserves":
			_ = json.NewEncoder(w).Encode(reserves)
		case r.URL.Path == "/payout_destinations/"+poDestID && r.Method == http.MethodDelete:
			out := map[string]any{}
			for k, v := range dest {
				out[k] = v
			}
			out["status"], out["deleted_at"] = "disabled", "2026-11-02T10:00:00Z"
			_ = json.NewEncoder(w).Encode(out)
		case r.URL.Path == "/payout_destinations/"+poDestID && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(dest)
		case r.URL.Path == "/payout_destinations/"+poDestID:
			_ = json.NewEncoder(w).Encode(dest)
		case r.URL.Path == "/payout_destinations" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]map[string]any{dest})
		case r.URL.Path == "/payout_destinations":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(dest)
		case r.URL.Path == "/payouts/prepare", r.URL.Path == "/payouts/"+poID:
			_ = json.NewEncoder(w).Encode(prepared)
		case r.URL.Path == "/payouts/"+poID+"/submit", r.URL.Path == "/payouts/"+poID+"/resend":
			w.WriteHeader(http.StatusAccepted)
			out := map[string]any{}
			for k, v := range prepared {
				out[k] = v
			}
			out["status"], out["signed"], out["transaction_hash"] = "submitted", true, "0x"+strings.Repeat("ab", 32)
			_ = json.NewEncoder(w).Encode(out)
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RAIL0_BASE_URL", srv.URL)
	if err := saveTokenForTest(makeJWT(t, map[string]any{"sub": signer, "exp": time.Now().Add(time.Hour).Unix()})); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	previous := tokenCatalogAll
	tokenCatalogAll = nil // the fake's catalog, not another test's
	t.Cleanup(func() {
		tokenCatalogAll = previous
		assumeYes, payoutConfirmDetails = false, false
		payoutSendDestination, payoutSendAmount, payoutSendProof, payoutSendKey, payoutSubmitKey = "", "", false, "", ""
		payoutSendNoRefund, payoutSendNoGas = false, false
		waitFlag, waitTimeout, waitTimeoutExplicit = false, defaultWaitTimeout, false
	})
	return g
}

// payoutTransfer is the gateway's unsigned transfer(to, amount) on poToken.
func payoutTransfer(t *testing.T, from, to, amount string) string {
	t.Helper()
	n, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		t.Fatalf("bad amount %q", amount)
	}
	b, _ := json.Marshal(map[string]any{
		"chain_id": poChain, "from": from, "to": poToken, "value": 0,
		"data":  "0xa9059cbb" + strings.Repeat("0", 24) + strings.TrimPrefix(strings.ToLower(to), "0x") + fmt.Sprintf("%064x", n),
		"nonce": 1, "gas_limit": 65000, "max_priority_fee_per_gas": 1, "max_fee_per_gas": 2,
	})
	return string(b)
}

// ── confirm_details ─────────────────────────────────────────────────────────

// Without a TTY and without --confirm-details, add refuses locally: nothing is sent,
// and --yes does not stand in for the attestation.
func TestDestinationAddRefusesWithoutTheDetailsConfirmation(t *testing.T) {
	g := newPayoutGateway(t)
	payoutDestAddChainID, payoutDestAddToken, payoutDestAddAddress, payoutDestAddMin = poChain, "USDC", poDeposit, "10"
	t.Cleanup(func() {
		payoutDestAddChainID, payoutDestAddToken, payoutDestAddAddress, payoutDestAddMin = 0, "", "", ""
	})
	assumeYes = true

	err := payoutDestinationsAddCmd.RunE(payoutDestinationsAddCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "--confirm-details") {
		t.Fatalf("want a refusal naming --confirm-details, got %v", err)
	}
	var ue *usageError
	if !errors.As(err, &ue) || ue.Code != "confirmation_required" {
		t.Errorf("want usage error confirmation_required, got %#v", err)
	}
	if g.sent("POST /payout_destinations") {
		t.Error("the destination was created without the details confirmation")
	}
}

// With --confirm-details the request carries confirm_details=true and the minimum
// converted from decimals to base units — and no reserve: the gateway decides the
// reserves at payout time.
func TestDestinationAddSendsTheConfirmationAndBaseUnits(t *testing.T) {
	g := newPayoutGateway(t)
	payoutDestAddChainID, payoutDestAddToken, payoutDestAddAddress = poChain, "USDC", poDeposit
	payoutDestAddMin = "10"
	payoutConfirmDetails = true
	t.Cleanup(func() {
		payoutDestAddChainID, payoutDestAddToken, payoutDestAddAddress, payoutDestAddMin = 0, "", "", ""
	})

	_ = captureStdout(t, func() {
		if err := payoutDestinationsAddCmd.RunE(payoutDestinationsAddCmd, nil); err != nil {
			t.Fatalf("add: %v", err)
		}
	})
	b := g.body("POST /payout_destinations")
	if b == nil {
		t.Fatal("no create request")
	}
	if b["confirm_details"] != true {
		t.Errorf("confirm_details = %v, want true", b["confirm_details"])
	}
	if b["min_amount"] != "10000000" {
		t.Errorf("min_amount = %v, want base units 10000000", b["min_amount"])
	}
	if _, present := b["reserve_amount"]; present {
		t.Errorf("reserve_amount sent: %v", b)
	}
	if b["token"] != poToken || !strings.EqualFold(fmt.Sprint(b["wallet_id"]), addressForTestKey(t)) {
		t.Errorf("token/wallet = %v/%v, want the resolved token and the session wallet", b["token"], b["wallet_id"])
	}
}

// ── the required minimum ────────────────────────────────────────────────────

// The minimum deposit is a required, explicit answer. Without a terminal, add with
// neither --min-amount nor --no-minimum, with both, or with a negative minimum is
// refused locally — nothing at all reaches the gateway, not even a token lookup.
func TestDestinationAddRefusesWithoutAnExplicitMinimum(t *testing.T) {
	for _, c := range []struct {
		name, min string
		noMin     bool
		code      string
		want      string
	}{
		{"neither flag", "", false, "minimum_required", "deposit page"},
		{"both flags", "10", true, "minimum_conflict", "mutually exclusive"},
		{"negative", "-1", false, "usage_error", "may not be negative"},
	} {
		t.Run(c.name, func(t *testing.T) {
			g := newPayoutGateway(t)
			payoutDestAddChainID, payoutDestAddToken, payoutDestAddAddress = poChain, "USDC", poDeposit
			payoutDestAddMin, payoutDestAddNoMin = c.min, c.noMin
			payoutConfirmDetails = true // the minimum is refused regardless
			t.Cleanup(func() {
				payoutDestAddChainID, payoutDestAddToken, payoutDestAddAddress, payoutDestAddMin = 0, "", "", ""
				payoutDestAddNoMin = false
			})

			err := payoutDestinationsAddCmd.RunE(payoutDestinationsAddCmd, nil)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want a refusal containing %q, got %v", c.want, err)
			}
			var ue *usageError
			if !errors.As(err, &ue) || ue.Code != c.code {
				t.Errorf("want usage error %s, got %#v", c.code, err)
			}
			if len(g.all()) > 0 {
				t.Errorf("requests %v sent before the usage error", g.all())
			}
		})
	}
}

// --no-minimum sends the flag no_minimum: true and no figure; --min-amount 0 is an
// ordinary figure, sent as "0" without the flag. The summary says which.
func TestDestinationAddSendsTheMinimumAnswer(t *testing.T) {
	for _, c := range []struct {
		name, min string
		noMin     bool
		wantMin   any
		wantFlag  any
		summary   string
	}{
		{"--no-minimum", "", true, nil, true, "none (no minimum at this venue)"},
		{"--min-amount 0", "0", false, "0", nil, "minimum deposit  0 USDC"},
	} {
		t.Run(c.name, func(t *testing.T) {
			g := newPayoutGateway(t)
			payoutDestAddChainID, payoutDestAddToken, payoutDestAddAddress = poChain, "USDC", poDeposit
			payoutDestAddMin, payoutDestAddNoMin = c.min, c.noMin
			payoutConfirmDetails = true
			t.Cleanup(func() {
				payoutDestAddChainID, payoutDestAddToken, payoutDestAddAddress, payoutDestAddMin = 0, "", "", ""
				payoutDestAddNoMin = false
			})

			var notice string
			_ = captureStdout(t, func() {
				notice = captureStderr(t, func() {
					if err := payoutDestinationsAddCmd.RunE(payoutDestinationsAddCmd, nil); err != nil {
						t.Fatalf("add: %v", err)
					}
				})
			})
			b := g.body("POST /payout_destinations")
			if b == nil || b["min_amount"] != c.wantMin || b["no_minimum"] != c.wantFlag {
				t.Errorf("body = %v, want min_amount %v and no_minimum %v", b, c.wantMin, c.wantFlag)
			}
			if !strings.Contains(notice, c.summary) {
				t.Errorf("the summary does not state %q:\n%s", c.summary, notice)
			}
		})
	}
}

// update: --min-amount "" is refused locally (nothing sent); --no-minimum sends the
// flag and, on a destination that had a figure, needs the details consent; both
// flags are a conflict.
func TestDestinationUpdateMinimumAnswer(t *testing.T) {
	cmd := payoutDestinationsUpdateCmd
	reset := func() {
		payoutDestUpdateMin, payoutDestUpdateNoMin = "", false
		cmd.Flags().Lookup("min-amount").Changed = false
		cmd.Flags().Lookup("no-minimum").Changed = false
	}
	t.Cleanup(reset)

	t.Run(`--min-amount ""`, func(t *testing.T) {
		g := newPayoutGateway(t)
		_ = cmd.Flags().Set("min-amount", "")
		t.Cleanup(reset)
		payoutConfirmDetails = true
		err := cmd.RunE(cmd, []string{poDestID})
		var ue *usageError
		if !errors.As(err, &ue) || ue.Code != "minimum_required" || !strings.Contains(err.Error(), "--no-minimum") {
			t.Fatalf("want usage error minimum_required naming --no-minimum, got %v", err)
		}
		if len(g.all()) > 0 {
			t.Errorf("requests %v sent before the usage error", g.all())
		}
	})

	t.Run("--no-minimum without consent", func(t *testing.T) {
		g := newPayoutGateway(t)
		_ = cmd.Flags().Set("no-minimum", "true")
		t.Cleanup(reset)
		err := cmd.RunE(cmd, []string{poDestID})
		if err == nil || !strings.Contains(err.Error(), "--confirm-details") {
			t.Fatalf("want a refusal, got %v", err)
		}
		if g.sent("PATCH /payout_destinations/" + poDestID) {
			t.Error("the minimum was removed without the details confirmation")
		}
	})

	t.Run("--no-minimum", func(t *testing.T) {
		g := newPayoutGateway(t)
		_ = cmd.Flags().Set("no-minimum", "true")
		t.Cleanup(reset)
		payoutConfirmDetails = true
		_ = captureStdout(t, func() {
			if err := cmd.RunE(cmd, []string{poDestID}); err != nil {
				t.Fatalf("update: %v", err)
			}
		})
		b := g.body("PATCH /payout_destinations/" + poDestID)
		if b["no_minimum"] != true || b["min_amount"] != nil || b["confirm_details"] != true {
			t.Errorf("body = %v, want no_minimum true, no min_amount, confirm_details", b)
		}
	})

	t.Run("--no-minimum, already none", func(t *testing.T) {
		g := newPayoutGateway(t)
		g.dest["min_amount"], g.dest["no_minimum"] = nil, true
		_ = cmd.Flags().Set("no-minimum", "true")
		t.Cleanup(reset)
		_ = captureStdout(t, func() {
			if err := cmd.RunE(cmd, []string{poDestID}); err != nil {
				t.Fatalf("update: %v", err)
			}
		})
		if b := g.body("PATCH /payout_destinations/" + poDestID); b["confirm_details"] != nil {
			t.Errorf("an unchanged answer sent confirm_details: %v", b)
		}
	})

	t.Run("both", func(t *testing.T) {
		g := newPayoutGateway(t)
		_ = cmd.Flags().Set("min-amount", "5")
		_ = cmd.Flags().Set("no-minimum", "true")
		t.Cleanup(reset)
		err := cmd.RunE(cmd, []string{poDestID})
		var ue *usageError
		if !errors.As(err, &ue) || ue.Code != "minimum_conflict" {
			t.Fatalf("want usage error minimum_conflict, got %v", err)
		}
		if len(g.all()) > 0 {
			t.Errorf("requests %v sent before the usage error", g.all())
		}
	})
}

// list/get show the minimum as a reader needs it: the figure, "none" when the
// merchant stated the venue has none, "not set" on a destination that never stated it.
func TestPayoutDestinationViewMinimum(t *testing.T) {
	for _, c := range []struct {
		name string
		d    rail0.PayoutDestination
		want string
	}{
		{"figure", rail0.PayoutDestination{MinAmount: "10000000", MinAmountFormatted: "10", TokenSymbol: "USDC"}, "10 USDC"},
		{"zero figure", rail0.PayoutDestination{MinAmount: "0", MinAmountFormatted: "0", TokenSymbol: "USDC"}, "0 USDC"},
		{"none", rail0.PayoutDestination{NoMinimum: true}, "none"},
		{"not set", rail0.PayoutDestination{}, "not set"},
	} {
		t.Run(c.name, func(t *testing.T) {
			b, _ := json.Marshal(viewPayoutDestination(&c.d))
			var m map[string]any
			_ = json.Unmarshal(b, &m)
			if m["minimum"] != c.want {
				t.Errorf("minimum = %v, want %q (%s)", m["minimum"], c.want, b)
			}
		})
	}

	g := newPayoutGateway(t)
	g.dest["min_amount"], g.dest["no_minimum"] = nil, true
	out := captureStdout(t, func() {
		if err := payoutDestinationsGetCmd.RunE(payoutDestinationsGetCmd, []string{poDestID}); err != nil {
			t.Fatalf("get: %v", err)
		}
	})
	if !strings.Contains(out, `"minimum"`) || !strings.Contains(out, "none") {
		t.Errorf("get output does not carry the minimum:\n%s", out)
	}
}

// The minimum hints name the flags.
func TestPayoutMinimumHints(t *testing.T) {
	if h := hintFor("minimum_required", ""); !strings.Contains(h, "--no-minimum") || !strings.Contains(h, "--min-amount") {
		t.Errorf("minimum_required hint %q", h)
	}
	if h := hintFor("minimum_conflict", ""); !strings.Contains(h, "--no-minimum") {
		t.Errorf("minimum_conflict hint %q", h)
	}
	if hintFor("payout_destination_minimum_unset", "") != "" {
		t.Error("payout_destination_minimum_unset no longer exists: no hint")
	}
}

// A label-only update needs no attestation and reads nothing first; changing the
// minimum without consent is refused locally; re-sending the current minimum is not a
// change and sends no confirm_details.
func TestDestinationUpdateAsksOnlyWhenTheMinimumChanges(t *testing.T) {
	cmd := payoutDestinationsUpdateCmd
	t.Cleanup(func() {
		payoutDestUpdateLabel, payoutDestUpdateMin = "", ""
		cmd.Flags().Lookup("label").Changed = false
		cmd.Flags().Lookup("min-amount").Changed = false
	})

	t.Run("label only", func(t *testing.T) {
		g := newPayoutGateway(t)
		_ = cmd.Flags().Set("label", "Main")
		t.Cleanup(func() { cmd.Flags().Lookup("label").Changed = false })
		_ = captureStdout(t, func() {
			if err := cmd.RunE(cmd, []string{poDestID}); err != nil {
				t.Fatalf("update: %v", err)
			}
		})
		b := g.body("PATCH /payout_destinations/" + poDestID)
		if b["label"] != "Main" || b["confirm_details"] != nil {
			t.Errorf("body = %v, want the label and no confirm_details", b)
		}
		if g.sent("GET /payout_destinations/" + poDestID) {
			t.Error("a label change read the destination for nothing")
		}
	})

	t.Run("new minimum without consent", func(t *testing.T) {
		g := newPayoutGateway(t)
		_ = cmd.Flags().Set("min-amount", "25")
		t.Cleanup(func() { cmd.Flags().Lookup("min-amount").Changed = false })
		assumeYes = true // must not count
		err := cmd.RunE(cmd, []string{poDestID})
		if err == nil || !strings.Contains(err.Error(), "--confirm-details") {
			t.Fatalf("want a refusal, got %v", err)
		}
		if g.sent("PATCH /payout_destinations/" + poDestID) {
			t.Error("the minimum was changed without the details confirmation")
		}
	})

	t.Run("new minimum with consent", func(t *testing.T) {
		g := newPayoutGateway(t)
		_ = cmd.Flags().Set("min-amount", "25")
		t.Cleanup(func() { cmd.Flags().Lookup("min-amount").Changed = false })
		payoutConfirmDetails = true
		_ = captureStdout(t, func() {
			if err := cmd.RunE(cmd, []string{poDestID}); err != nil {
				t.Fatalf("update: %v", err)
			}
		})
		b := g.body("PATCH /payout_destinations/" + poDestID)
		if b["min_amount"] != "25000000" || b["confirm_details"] != true {
			t.Errorf("body = %v, want min_amount 25000000 with confirm_details", b)
		}
	})

	t.Run("same minimum", func(t *testing.T) {
		g := newPayoutGateway(t)
		_ = cmd.Flags().Set("min-amount", "10.00")
		t.Cleanup(func() { cmd.Flags().Lookup("min-amount").Changed = false })
		_ = captureStdout(t, func() {
			if err := cmd.RunE(cmd, []string{poDestID}); err != nil {
				t.Fatalf("update: %v", err)
			}
		})
		if b := g.body("PATCH /payout_destinations/" + poDestID); b["confirm_details"] != nil {
			t.Errorf("an unchanged minimum sent confirm_details: %v", b)
		}
	})
}

// ── send ────────────────────────────────────────────────────────────────────

// The happy path: the destination is read, the payout prepared, the token catalog
// consulted for the typed amount, the transfer verified and signed, then submitted.
func TestPayoutsSendSignsAndSubmitsTheVerifiedTransfer(t *testing.T) {
	g := newPayoutGateway(t)
	payoutSendDestination, payoutSendAmount, payoutSendKey = poDestID, "25", testKeyHex
	assumeYes = true

	out := captureStdout(t, func() {
		if err := payoutsSendCmd.RunE(payoutsSendCmd, nil); err != nil {
			t.Fatalf("send: %v", err)
		}
	})
	b := g.body("POST /payouts/prepare")
	if b["destination_id"] != poDestID || b["amount"] != "25" || b["purpose"] != "payout" {
		t.Errorf("prepare body = %v", b)
	}
	sub := g.body("POST /payouts/" + poID + "/submit")
	if s, _ := sub["signed_transaction"].(string); !strings.HasPrefix(s, "0x") {
		t.Errorf("submit carried no signed transaction: %v", sub)
	}
	if !strings.Contains(out, "submitted") {
		t.Errorf("output = %s, want the submitted payout", out)
	}
}

// A prepared transfer to another address is refused before the key signs it: no
// submit goes out. (The SDK's verifier; this pins that send goes through it.)
func TestPayoutsSendRefusesATamperedTransfer(t *testing.T) {
	g := newPayoutGateway(t)
	g.prepared["unsigned_transaction"] = payoutTransfer(t, addressForTestKey(t), "0x2222222222222222222222222222222222222222", "25000000")
	payoutSendDestination, payoutSendAmount, payoutSendKey = poDestID, "25", testKeyHex
	assumeYes = true

	err := payoutsSendCmd.RunE(payoutsSendCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "refusing to sign") || strings.HasPrefix(err.Error(), "rail0: ") {
		t.Fatalf("want the SDK refusal without its prefix, got %v", err)
	}
	if g.sent("POST /payouts/" + poID + "/submit") {
		t.Error("a tampered transfer was submitted")
	}
}

// Without a TTY and without --yes, send stops at the confirmation: nothing prepared.
func TestPayoutsSendNeedsConfirmation(t *testing.T) {
	g := newPayoutGateway(t)
	payoutSendDestination, payoutSendAmount, payoutSendKey = poDestID, "25", testKeyHex

	err := payoutsSendCmd.RunE(payoutsSendCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "refusing to run without confirmation") {
		t.Fatalf("want the confirmation gate, got %v", err)
	}
	if g.sent("POST /payouts/prepare") {
		t.Error("prepared before the confirmation")
	}
}

// An unverified destination takes the proof only: a payout is refused before the
// prompt, the proof goes through.
func TestPayoutsSendToAnUnverifiedDestination(t *testing.T) {
	g := newPayoutGateway(t)
	g.dest["status"] = "unverified"
	payoutSendDestination, payoutSendAmount, payoutSendKey = poDestID, "25", testKeyHex
	assumeYes = true
	if err := payoutsSendCmd.RunE(payoutsSendCmd, nil); err == nil || !strings.Contains(err.Error(), "--proof") {
		t.Fatalf("want a refusal pointing at --proof, got %v", err)
	}
	if g.sent("POST /payouts/prepare") {
		t.Error("prepared a payout to an unverified destination")
	}

	g.prepared["purpose"], g.prepared["amount"] = "proof", "10000000"
	g.prepared["unsigned_transaction"] = payoutTransfer(t, addressForTestKey(t), poDeposit, "10000000")
	payoutSendAmount, payoutSendProof = "", true
	_ = captureStdout(t, func() {
		if err := payoutsSendCmd.RunE(payoutsSendCmd, nil); err != nil {
			t.Fatalf("proof: %v", err)
		}
	})
	if b := g.body("POST /payouts/prepare"); b["purpose"] != "proof" || b["amount"] != nil {
		t.Errorf("proof prepare body = %v, want purpose proof and no amount", b)
	}
	if !g.sent("POST /payouts/" + poID + "/submit") {
		t.Error("the proof was not submitted")
	}
}

// The reserve opt-outs: sent only as false and only when asked for (unset = the
// gateway keeps both), and refused with --proof, which keeps nothing back.
func TestPayoutPrepareRequestReserveFlags(t *testing.T) {
	t.Cleanup(func() {
		payoutSendDestination, payoutSendAmount, payoutSendProof = "", "", false
		payoutSendNoRefund, payoutSendNoGas = false, false
	})
	payoutSendDestination, payoutSendAmount = poDestID, "all"
	req, err := payoutPrepareRequest()
	if err != nil || req.KeepRefundReserve != nil || req.KeepGasReserve != nil {
		t.Fatalf("no flags: req = %+v, err = %v, want both reserve keys unset", req, err)
	}
	payoutSendNoRefund = true
	req, err = payoutPrepareRequest()
	if err != nil || req.KeepRefundReserve == nil || *req.KeepRefundReserve || req.KeepGasReserve != nil {
		t.Fatalf("--no-refund-reserve: req = %+v, err = %v", req, err)
	}
	payoutSendNoRefund, payoutSendNoGas = false, true
	req, err = payoutPrepareRequest()
	if err != nil || req.KeepGasReserve == nil || *req.KeepGasReserve || req.KeepRefundReserve != nil {
		t.Fatalf("--no-gas-reserve: req = %+v, err = %v", req, err)
	}
	payoutSendAmount, payoutSendProof = "", true
	if _, err := payoutPrepareRequest(); err == nil || !strings.Contains(err.Error(), "not to the proof") {
		t.Fatalf("--proof with --no-gas-reserve: err = %v, want a usage error", err)
	}
}

// send --no-refund-reserve sends keep_refund_reserve: false and nothing about the gas
// reserve; the prompt says the refund reserve is skipped and what that risks, and
// "all" is computed without it.
func TestPayoutsSendWithoutTheRefundReserve(t *testing.T) {
	g := newPayoutGateway(t)
	payoutSendDestination, payoutSendAmount, payoutSendKey = poDestID, "25", testKeyHex
	payoutSendNoRefund, assumeYes = true, true
	_ = captureStdout(t, func() {
		if err := payoutsSendCmd.RunE(payoutsSendCmd, nil); err != nil {
			t.Fatalf("send: %v", err)
		}
	})
	b := g.body("POST /payouts/prepare")
	if v, present := b["keep_refund_reserve"]; !present || v != false {
		t.Errorf("prepare body = %v, want keep_refund_reserve false", b)
	}
	if _, present := b["keep_gas_reserve"]; present {
		t.Errorf("prepare body = %v, want no keep_gas_reserve", b)
	}

	res := &rail0.PayoutReserves{Balance: "100000000", RefundReserve: "5000000", RefundablePayments: 3,
		NativeGas: true, GasReserve: "150000", GasAllowance: "20000", Available: "94830000"}
	dest := &rail0.PayoutDestination{ID: poDestID, ChainID: poChain, Token: poToken, TokenSymbol: "USDC", Address: poDeposit}
	req, _ := payoutPrepareRequest()
	req.Amount = rail0.PayoutAmountAll
	prompt := payoutSendPrompt(t.Context(), dest, req, res, "0xsigner")
	for _, want := range []string{"about 99.83 USDC now", "refund reserve  SKIPPED (--no-refund-reserve)", "a later refund may fail", "gas reserve     0.15 USDC kept back"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt lacks %q:\n%s", want, prompt)
		}
	}
}

// Both reserves kept (the default): the prompt names them with the gateway's figures,
// "all" is the gateway's available, and off a native-gas chain there is no gas line.
func TestPayoutSendPromptShowsTheReserves(t *testing.T) {
	_ = newPayoutGateway(t)
	res := &rail0.PayoutReserves{Balance: "100000000", RefundReserve: "5000000", RefundablePayments: 1,
		NativeGas: true, GasReserve: "150000", GasAllowance: "20000", Available: "94830000"}
	dest := &rail0.PayoutDestination{ID: poDestID, ChainID: poChain, Token: poToken, TokenSymbol: "USDC", Address: poDeposit}
	req := rail0.PreparePayoutRequest{DestinationID: poDestID, Purpose: rail0.PayoutPurposePayout, Amount: rail0.PayoutAmountAll}
	prompt := payoutSendPrompt(t.Context(), dest, req, res, "0xsigner")
	for _, want := range []string{"about 94.83 USDC now", "refund reserve  5 USDC kept back (1 refundable payment)", "gas reserve     0.15 USDC kept back"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt lacks %q:\n%s", want, prompt)
		}
	}
	res.NativeGas, res.GasReserve, res.GasAllowance = false, "0", "0"
	if prompt := payoutSendPrompt(t.Context(), dest, req, res, "0xsigner"); strings.Contains(prompt, "gas reserve") {
		t.Errorf("a chain whose gas is not the token shows a gas reserve:\n%s", prompt)
	}
	req.Purpose, req.Amount = rail0.PayoutPurposeProof, ""
	if prompt := payoutSendPrompt(t.Context(), dest, req, nil, "0xsigner"); strings.Contains(prompt, "reserve") {
		t.Errorf("the proof keeps nothing back, prompt:\n%s", prompt)
	}
}

// submit shows the reserves the prepared payout recorded.
func TestPayoutSubmitPromptShowsTheRecordedReserves(t *testing.T) {
	_ = newPayoutGateway(t)
	p := &rail0.Payout{ID: poID, Purpose: "payout", ChainID: poChain, Token: poToken, TokenSymbol: "USDC",
		Address: poDeposit, Amount: "25000000", AmountFormatted: "25", RefundReserve: "0", GasReserve: "150000"}
	prompt := payoutSubmitPrompt(t.Context(), p, "0xsigner")
	for _, want := range []string{"refund reserve  0 USDC kept back", "gas reserve     0.15 USDC kept back"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt lacks %q:\n%s", want, prompt)
		}
	}
}

// ── destinations delete / reserves / --include-deleted ──────────────────────

// delete is gated like every destructive command: no TTY and no --yes refuses before
// any request; with --yes it sends DELETE and prints the deleted destination.
func TestDestinationDelete(t *testing.T) {
	g := newPayoutGateway(t)
	err := payoutDestinationsDeleteCmd.RunE(payoutDestinationsDeleteCmd, []string{poDestID})
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("want the confirmation gate, got %v", err)
	}
	if len(g.all()) > 0 {
		t.Fatalf("requests %v sent before the confirmation", g.all())
	}
	assumeYes = true
	out := captureStdout(t, func() {
		if err := payoutDestinationsDeleteCmd.RunE(payoutDestinationsDeleteCmd, []string{poDestID}); err != nil {
			t.Fatalf("delete: %v", err)
		}
	})
	if !g.sent("DELETE /payout_destinations/" + poDestID) {
		t.Error("no DELETE sent")
	}
	if !strings.Contains(out, "deleted_at") || !strings.Contains(out, "disabled") {
		t.Errorf("output = %s, want the deleted destination", out)
	}
}

// reserves reads GET …/reserves; machine output is the gateway's record unchanged,
// the terminal form puts every figure in token units with the counts beside them.
func TestDestinationReserves(t *testing.T) {
	g := newPayoutGateway(t)
	out := captureStdout(t, func() {
		if err := payoutDestinationsReservesCmd.RunE(payoutDestinationsReservesCmd, []string{poDestID}); err != nil {
			t.Fatalf("reserves: %v", err)
		}
	})
	if !g.sent("GET /payout_destinations/" + poDestID + "/reserves") {
		t.Error("no reserves read")
	}
	for _, want := range []string{`"available": "94830000"`, `"refundable_payments": 3`, `"gas_reserve": "150000"`} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %s:\n%s", want, out)
		}
	}

	r := &rail0.PayoutReserves{DestinationID: poDestID, ChainID: poChain, Token: poToken, TokenSymbol: "USDC",
		BalanceFormatted: "100", RefundReserveFormatted: "5", RefundablePayments: 1, NativeGas: true,
		GasReserveFormatted: "0.15", GasRefunds: 1, OpenAuthorizations: 2, BaseCaptures: 5,
		GasAllowanceFormatted: "0.02", AvailableFormatted: "94.83"}
	v := humanPayoutReserves(r)
	if v.Balance != "100 USDC" || v.RefundReserve != "5 USDC (1 refundable payment)" || v.Available != "94.83 USDC" ||
		v.GasReserve != "0.15 USDC (1 refund, 2 open authorizations + 5 new captures)" || v.GasAllowance != "0.02 USDC" {
		t.Errorf("human view = %+v", v)
	}
	r.NativeGas = false
	if v := humanPayoutReserves(r); v.GasReserve != "" || v.GasAllowance != "" {
		t.Errorf("gas lines off a native-gas chain: %+v", v)
	}
}

func TestDestinationListIncludeDeleted(t *testing.T) {
	g := newPayoutGateway(t)
	payoutDestListDeleted = true
	t.Cleanup(func() { payoutDestListDeleted = false })
	_ = captureStdout(t, func() {
		if err := payoutDestinationsListCmd.RunE(payoutDestinationsListCmd, nil); err != nil {
			t.Fatalf("list: %v", err)
		}
	})
	if q := g.query("GET /payout_destinations"); q != "include_deleted=true" {
		t.Errorf("query = %q, want include_deleted=true", q)
	}
	for _, c := range []*cobra.Command{payoutDestinationsAddCmd, payoutDestinationsUpdateCmd} {
		if c.Flags().Lookup("reserve") != nil {
			t.Errorf("%s still takes --reserve", c.CommandPath())
		}
	}
}

func TestPayoutPrepareRequestFlags(t *testing.T) {
	t.Cleanup(func() { payoutSendDestination, payoutSendAmount, payoutSendProof = "", "", false })
	for _, c := range []struct {
		dest, amount string
		proof        bool
		wantErr      string
		wantAmount   string
	}{
		{"", "1", false, "--destination", ""},
		{poDestID, "", false, "--amount is required", ""},
		{poDestID, "1", true, "drop --amount", ""},
		{poDestID, "-1", false, "invalid amount", ""},
		{poDestID, "ALL", false, "", "all"},
		{poDestID, "1.50", false, "", "1.50"},
		{poDestID, "", true, "", ""},
	} {
		payoutSendDestination, payoutSendAmount, payoutSendProof = c.dest, c.amount, c.proof
		req, err := payoutPrepareRequest()
		if c.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("%+v: err = %v, want %q", c, err, c.wantErr)
			}
			continue
		}
		if err != nil || req.Amount != c.wantAmount {
			t.Errorf("%+v: req = %+v, err = %v", c, req, err)
		}
	}
}

// ── submit / report-hash / list ─────────────────────────────────────────────

// A signed payout is resend's: submit refuses it before prompting or signing.
func TestPayoutsSubmitRefusesASignedPayout(t *testing.T) {
	g := newPayoutGateway(t)
	g.prepared["signed"] = true
	payoutSubmitKey = testKeyHex
	assumeYes = true
	err := payoutsSubmitCmd.RunE(payoutsSubmitCmd, []string{poID})
	if err == nil || !strings.Contains(err.Error(), "payouts resend") {
		t.Fatalf("want a pointer to resend, got %v", err)
	}
	if g.sent("POST /payouts/" + poID + "/submit") {
		t.Error("a signed payout was signed again")
	}
}

func TestPayoutsSubmitSignsAPendingPayout(t *testing.T) {
	g := newPayoutGateway(t)
	payoutSubmitKey = testKeyHex
	assumeYes = true
	_ = captureStdout(t, func() {
		if err := payoutsSubmitCmd.RunE(payoutsSubmitCmd, []string{poID}); err != nil {
			t.Fatalf("submit: %v", err)
		}
	})
	if !g.sent("GET /payout_destinations/"+poDestID) || !g.sent("POST /payouts/"+poID+"/submit") {
		t.Error("submit must verify against the destination, then submit")
	}
}

func TestPayoutsReportHashChecksTheHash(t *testing.T) {
	err := payoutsReportHashCmd.RunE(payoutsReportHashCmd, []string{poID, "0xabc"})
	if err == nil || !strings.Contains(err.Error(), "64 hex") {
		t.Errorf("want a malformed-hash refusal, got %v", err)
	}
}

func TestPayoutsListValidatesFiltersLocally(t *testing.T) {
	g := newPayoutGateway(t)
	payoutsListStatuses = []string{"submitted", "done"}
	t.Cleanup(func() { payoutsListStatuses, payoutsListPurpose = nil, "" })
	if err := payoutsListCmd.RunE(payoutsListCmd, nil); err == nil || !strings.Contains(err.Error(), `"done"`) {
		t.Errorf("want the bad status named, got %v", err)
	}
	payoutsListStatuses, payoutsListPurpose = []string{"submitted", "confirmed"}, "refund"
	if err := payoutsListCmd.RunE(payoutsListCmd, nil); err == nil || !strings.Contains(err.Error(), `"refund"`) {
		t.Errorf("want the bad purpose named, got %v", err)
	}
	if g.sent("GET /payouts") {
		t.Error("an invalid filter reached the gateway")
	}
}

// Every payout command is in the tree, the listing groups run bare, and the
// fund-moving / attesting ones take --yes.
func TestPayoutCommandTree(t *testing.T) {
	for _, path := range [][]string{
		{"payouts", "list"}, {"payouts", "get"}, {"payouts", "send"}, {"payouts", "prepare"},
		{"payouts", "submit"}, {"payouts", "resend"}, {"payouts", "cancel"}, {"payouts", "report-hash"},
		{"payouts", "events"}, {"payouts", "destinations", "list"}, {"payouts", "destinations", "add"},
		{"payouts", "destinations", "update"}, {"payouts", "destinations", "disable"},
		{"payouts", "destinations", "enable"}, {"payouts", "destinations", "confirm"},
		{"payouts", "destinations", "events"}, {"payouts", "destinations", "get"},
		{"payouts", "destinations", "delete"}, {"payouts", "destinations", "reserves"},
	} {
		if c, _, err := rootCmd.Find(path); err != nil || c.Name() != path[len(path)-1] {
			t.Errorf("rail0 %s: not found (%v)", strings.Join(path, " "), err)
		}
	}
	if payoutsCmd.RunE == nil || payoutDestinationsCmd.RunE == nil {
		t.Error("payouts and payouts destinations must list when run bare")
	}
	for _, c := range []*cobra.Command{payoutsSendCmd, payoutsSubmitCmd, payoutDestinationsConfirmCmd, payoutDestinationsDeleteCmd} {
		if c.Flags().Lookup("yes") == nil {
			t.Errorf("%s: missing --yes", c.CommandPath())
		}
	}
	for _, c := range []*cobra.Command{payoutDestinationsAddCmd, payoutDestinationsUpdateCmd} {
		if c.Flags().Lookup("confirm-details") == nil {
			t.Errorf("%s: missing --confirm-details", c.CommandPath())
		}
	}
	if !strings.Contains(rail0.PayoutDetailsConfirmationText, "permanently lost") {
		t.Error("unexpected details text")
	}
}

// ── --wait ───────────────────────────────────────────────────────────────────

// send --wait follows the payout past the submit: submitted → confirmed, and the
// result is the confirmed payout rather than the submit's answer.
func TestPayoutsSendWaitFollowsTheTransferToConfirmed(t *testing.T) {
	g := newPayoutGateway(t)
	fastPolls(t)
	g.pollsAfterSubmit([]string{"submitted", "submitted", "confirmed"}, map[string]any{"block_number": 42})
	payoutSendDestination, payoutSendAmount, payoutSendKey = poDestID, "25", testKeyHex
	assumeYes, waitFlag = true, true

	out := captureStdout(t, func() {
		if err := payoutsSendCmd.RunE(payoutsSendCmd, nil); err != nil {
			t.Fatalf("send --wait: %v", err)
		}
	})
	if !strings.Contains(out, `"status": "confirmed"`) || !strings.Contains(out, `"block_number": 42`) {
		t.Errorf("with --wait the result is the confirmed payout, got:\n%s", out)
	}
	if n := g.count("GET /payouts/" + poID); n != 3 {
		t.Errorf("polled %d times, want 3 (until confirmed)", n)
	}
}

// A payout that fails while waited on stops the wait at once with the gateway's
// detail; submission_timeout points at report-hash, the way back.
func TestPayoutsWaitStopsOnFailed(t *testing.T) {
	for code, want := range map[string]string{
		"reverted":           "transfer reverted",
		"submission_timeout": "payouts report-hash " + poID,
	} {
		t.Run(code, func(t *testing.T) {
			g := newPayoutGateway(t)
			fastPolls(t)
			g.pollsAfterSubmit([]string{"submitted", "failed"},
				map[string]any{"error_code": code, "error_detail": "transfer reverted"})
			payoutSubmitKey = testKeyHex
			assumeYes, waitFlag = true, true

			var err error
			_ = captureStdout(t, func() { err = payoutsSubmitCmd.RunE(payoutsSubmitCmd, []string{poID}) })
			if err == nil || !strings.Contains(err.Error(), "failed") || !strings.Contains(err.Error(), want) {
				t.Fatalf("err = %v, want the failure with %q", err, want)
			}
			if errors.Is(err, ErrWaitGaveUp) || exitCode(err) != 1 {
				t.Errorf("a failed payout is a failure (exit 1), not a timeout: exit %d", exitCode(err))
			}
		})
	}
}

// A wait that runs out says the transfer may still be in flight and exits 11, like a
// payment wait: never "failed", never "safe to retry".
func TestPayoutsResendWaitGivesUpInFlight(t *testing.T) {
	g := newPayoutGateway(t)
	fastPolls(t)
	g.pollsAfterSubmit([]string{"submitted"}, nil)
	waitFlag, waitTimeout, waitTimeoutExplicit = true, 20*time.Millisecond, true

	var err error
	_ = captureStdout(t, func() { err = payoutsResendCmd.RunE(payoutsResendCmd, []string{poID}) })
	if !errors.Is(err, ErrWaitGaveUp) || exitCode(err) != ExitCodeInFlight {
		t.Fatalf("err = %v (exit %d), want a give-up exiting %d", err, exitCode(err), ExitCodeInFlight)
	}
	if !strings.Contains(err.Error(), "rail0 payouts get "+poID) {
		t.Errorf("the give-up must name the check command: %v", err)
	}
	if !g.sent("POST /payouts/" + poID + "/resend") {
		t.Error("resend never went out")
	}
}

// Without --wait nothing is polled: the submit's answer is the result, as before.
func TestPayoutsSendWithoutWaitDoesNotPoll(t *testing.T) {
	g := newPayoutGateway(t)
	g.pollsAfterSubmit([]string{"confirmed"}, nil)
	payoutSendDestination, payoutSendAmount, payoutSendKey = poDestID, "25", testKeyHex
	assumeYes = true
	_ = captureStdout(t, func() {
		if err := payoutsSendCmd.RunE(payoutsSendCmd, nil); err != nil {
			t.Fatalf("send: %v", err)
		}
	})
	if n := g.count("GET /payouts/" + poID); n != 0 {
		t.Errorf("polled %d times without --wait", n)
	}
}

func TestPayoutSentHintAfterAWait(t *testing.T) {
	if h := payoutSentHint(&rail0.Payout{ID: poID, Purpose: "payout", Status: "confirmed"}); h != "" {
		t.Errorf("a confirmed payout needs no next step, got %q", h)
	}
	h := payoutSentHint(&rail0.Payout{ID: poID, DestinationID: poDestID, Purpose: "proof", Status: "confirmed"})
	if !strings.Contains(h, "destinations confirm "+poDestID) || strings.Contains(h, "payouts get") {
		t.Errorf("a confirmed proof leaves only the venue check: %q", h)
	}
}
