package cmd

// End-to-end runs of the signing commands against a fake gateway: session read
// from the cache, payment fetched, prepare → verify → sign → submit, exactly as a
// real run does. The unit tests pin each helper; these pin that the command wires
// them together — which is where the --idempotency-key re-run bug lived.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	lcRail0ID = "0xa1b2c3d40000000000000000000000000000000000000000000000000000beef"
	lcToken   = "0x3600000000000000000000000000000000000000"
	lcChainID = 5042002
)

// lifecycleGateway serves one payment whose payee is the test key, answers each
// prepare with whatever the test put in `prepare` (keyed by operation), and records
// every request so a test can assert what was — and was NOT — sent.
type lifecycleGateway struct {
	mu       sync.Mutex
	prepare  map[string][]any // op → successive prepare responses
	requests []string         // "METHOD path"
	headers  []http.Header
}

func (g *lifecycleGateway) sent(methodPath string) int {
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

// unsignedFor is a transaction verify() accepts: value 0, the payment's chain, from
// the signer, calldata carrying this payment's id.
func unsignedFor(t *testing.T) string {
	t.Helper()
	b, _ := json.Marshal(map[string]any{
		"chain_id": lcChainID, "from": addressForTestKey(t),
		"to": "0x4CCC4DdeBB8A63B9186936A8C0FA404910A4311b", "value": 0,
		"data":  "0xdeadbeef" + strings.TrimPrefix(lcRail0ID, "0x"),
		"nonce": 1, "gas_limit": 200000, "max_priority_fee_per_gas": 1, "max_fee_per_gas": 2,
	})
	return string(b)
}

// newLifecycleGateway starts the fake gateway, points the CLI at it, and seeds a live
// session for the test key so no sign-in runs.
func newLifecycleGateway(t *testing.T) *lifecycleGateway {
	t.Helper()
	isolate(t)
	isolateTokenStore(t)
	t.Setenv("RAIL0_PRIVATE_KEY", "")
	signer := addressForTestKey(t)
	g := &lifecycleGateway{prepare: map[string][]any{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		g.requests = append(g.requests, r.Method+" "+r.URL.Path)
		g.headers = append(g.headers, r.Header.Clone())
		g.mu.Unlock()
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		base := "/payments/" + lcRail0ID
		switch {
		case r.Method == http.MethodGet && r.URL.Path == base:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "pay-1", "rail0_id": lcRail0ID, "status": "captured", "chain_id": lcChainID,
				"token": lcToken, "amount": "1000000", "payer": "0x9999999999999999999999999999999999999999",
				"payee": signer,
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/prepare"):
			op := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, base+"/"), "/prepare")
			g.mu.Lock()
			queue := g.prepare[op]
			var resp any = map[string]any{}
			if len(queue) > 0 {
				resp, g.prepare[op] = queue[0], queue[1:]
			}
			g.mu.Unlock()
			_ = json.NewEncoder(w).Encode(resp)
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "tx-new", "status": "submitting"})
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RAIL0_BASE_URL", srv.URL)
	if err := saveTokenForTest(makeJWT(t, map[string]any{"sub": signer, "exp": time.Now().Add(time.Hour).Unix()})); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	assumeYes = true
	t.Cleanup(func() { assumeYes, idempotencyKey, waitFlag = false, "", false })
	return g
}

// captureStdout runs fn with os.Stdout redirected and returns what it printed.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fn()
	_ = w.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The re-run the help recommends after a timed-out --wait: the keyed prepare replays
// the ORIGINAL transaction, already confirmed. Nothing may be submitted — submit
// accepts only pending rows, which is the 404 every such re-run used to die on.
func TestCaptureReRunWithAKeyFollowsTheReplayedTransaction(t *testing.T) {
	g := newLifecycleGateway(t)
	g.prepare["capture"] = []any{map[string]any{"id": "tx-1", "operation": "capture", "status": "confirmed"}}
	idempotencyKey, captureAmount, capturePrivateKey = "order-42", "1.00", testKeyHex
	t.Cleanup(func() { captureAmount, capturePrivateKey = "", "" })

	out := captureStdout(t, func() {
		if err := paymentsCaptureCmd.RunE(paymentsCaptureCmd, []string{lcRail0ID}); err != nil {
			t.Fatalf("capture re-run: %v", err)
		}
	})
	if n := g.sent("POST /payments/" + lcRail0ID + "/capture"); n != 0 {
		t.Errorf("the replayed capture was submitted %d time(s); it must not be", n)
	}
	if !strings.Contains(out, `"tx-1"`) || !strings.Contains(out, `"confirmed"`) {
		t.Errorf("the result must be the original transaction, got:\n%s", out)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for i, r := range g.requests {
		if strings.HasSuffix(r, "/capture/prepare") && g.headers[i].Get("Idempotency-Key") != "order-42" {
			t.Errorf("the prepare must carry the key, got %q", g.headers[i].Get("Idempotency-Key"))
		}
	}
}

// A first capture (no replay) still goes all the way: prepare, sign, submit once.
func TestCaptureWithoutAReplaySubmitsOnce(t *testing.T) {
	g := newLifecycleGateway(t)
	g.prepare["capture"] = []any{map[string]any{"id": "tx-1", "operation": "capture", "status": "pending",
		"unsigned_transaction": unsignedFor(t)}}
	idempotencyKey, captureAmount, capturePrivateKey = "order-42", "1.00", testKeyHex
	t.Cleanup(func() { captureAmount, capturePrivateKey = "", "" })

	_ = captureStdout(t, func() {
		if err := paymentsCaptureCmd.RunE(paymentsCaptureCmd, []string{lcRail0ID}); err != nil {
			t.Fatalf("capture: %v", err)
		}
	})
	if n := g.sent("POST /payments/" + lcRail0ID + "/capture"); n != 1 {
		t.Errorf("submitted %d time(s), want exactly 1", n)
	}
}

// Refund sends the key from phase 1, so a re-run whose refund already went out gets
// the original row back THERE: no signing payload is signed, nothing is submitted.
func TestRefundReRunStopsAtPhaseOne(t *testing.T) {
	g := newLifecycleGateway(t)
	g.prepare["refund"] = []any{map[string]any{"id": "tx-r", "operation": "refund", "status": "submitted"}}
	idempotencyKey, refundAmount, refundPrivateKey = "rf-1", "0.50", testKeyHex
	t.Cleanup(func() { refundAmount, refundPrivateKey = "", "" })

	out := captureStdout(t, func() {
		if err := paymentsRefundCmd.RunE(paymentsRefundCmd, []string{lcRail0ID}); err != nil {
			t.Fatalf("refund re-run: %v", err)
		}
	})
	if n := g.sent("POST /payments/" + lcRail0ID + "/refund/prepare"); n != 1 {
		t.Errorf("prepare sent %d time(s), want 1 (phase 1 answered with the replay)", n)
	}
	if n := g.sent("POST /payments/" + lcRail0ID + "/refund"); n != 0 {
		t.Errorf("the replayed refund was submitted %d time(s)", n)
	}
	if !strings.Contains(out, `"tx-r"`) {
		t.Errorf("the result must be the original transaction, got:\n%s", out)
	}
}

// A replayed refund still PENDING already carries the unsigned transaction with the
// authorization embedded: it is signed and submitted as is, without asking the payee
// for a fresh authorization signature (no second prepare).
func TestRefundReRunFinishesAPendingReplay(t *testing.T) {
	g := newLifecycleGateway(t)
	g.prepare["refund"] = []any{map[string]any{"id": "tx-r", "operation": "refund", "status": "pending",
		"unsigned_transaction": unsignedFor(t)}}
	idempotencyKey, refundAmount, refundPrivateKey = "rf-1", "0.50", testKeyHex
	t.Cleanup(func() { refundAmount, refundPrivateKey = "", "" })

	_ = captureStdout(t, func() {
		if err := paymentsRefundCmd.RunE(paymentsRefundCmd, []string{lcRail0ID}); err != nil {
			t.Fatalf("refund re-run: %v", err)
		}
	})
	if n := g.sent("POST /payments/" + lcRail0ID + "/refund/prepare"); n != 1 {
		t.Errorf("prepare sent %d time(s), want 1", n)
	}
	if n := g.sent("POST /payments/" + lcRail0ID + "/refund"); n != 1 {
		t.Errorf("submitted %d time(s), want exactly 1", n)
	}
}

// The refund authorization's amount check fails CLOSED: an amount finer than the
// token's decimals used to skip the value check and sign whatever came back. Now the
// command refuses after phase 1, and nothing further is prepared or submitted.
func TestRefundRefusesAnAmountItCannotCheck(t *testing.T) {
	g := newLifecycleGateway(t)
	seedDecimals(t, lcChainID, lcToken, 6)
	g.prepare["refund"] = []any{map[string]any{"signing_payload": map[string]any{
		"primaryType": "ReceiveWithAuthorization",
		"domain":      map[string]any{"chainId": lcChainID, "verifyingContract": lcToken},
		"message":     map[string]any{"from": addressForTestKey(t), "value": "999999999"},
	}}}
	refundAmount, refundPrivateKey = "0.0000001", testKeyHex
	t.Cleanup(func() { refundAmount, refundPrivateKey = "", "" })

	var err error
	_ = captureStdout(t, func() { err = paymentsRefundCmd.RunE(paymentsRefundCmd, []string{lcRail0ID}) })
	if err == nil {
		t.Fatal("a refund amount finer than the token's decimals must be refused")
	}
	if n := g.sent("POST /payments/" + lcRail0ID + "/refund/prepare"); n != 1 {
		t.Errorf("prepare sent %d time(s), want only phase 1", n)
	}
	if n := g.sent("POST /payments/" + lcRail0ID + "/refund"); n != 0 {
		t.Errorf("submitted %d time(s) after a refusal", n)
	}
}
