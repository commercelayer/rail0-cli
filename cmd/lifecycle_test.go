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
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	rail0 "github.com/commercelayer/rail0-go"
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
// the signer, calldata calling op's RAIL0 function on this payment's id (with
// amountUnits as capture/refund's amount).
func unsignedFor(t *testing.T, op, amountUnits string) string {
	t.Helper()
	b, _ := json.Marshal(map[string]any{
		"chain_id": lcChainID, "from": addressForTestKey(t),
		"to": "0x4CCC4DdeBB8A63B9186936A8C0FA404910A4311b", "value": 0,
		"data":  calldataFor(t, op, lcRail0ID, amountUnits),
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
	seedDecimals(t, lcChainID, lcToken, 6)
	g.prepare["capture"] = []any{map[string]any{"id": "tx-1", "operation": "capture", "status": "pending",
		"unsigned_transaction": unsignedFor(t, "capture", "1000000")}}
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
	seedDecimals(t, lcChainID, lcToken, 6)
	g.prepare["refund"] = []any{map[string]any{"id": "tx-r", "operation": "refund", "status": "pending",
		"unsigned_transaction": unsignedFor(t, "refund", "500000")}}
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

// The capture prepare answered with capture calldata for MORE than the user typed:
// the right function on the right payment, so only the amount word gives it away.
// Nothing may be signed or submitted.
func TestCaptureRefusesCalldataForAnotherAmount(t *testing.T) {
	g := newLifecycleGateway(t)
	seedDecimals(t, lcChainID, lcToken, 6)
	g.prepare["capture"] = []any{map[string]any{"id": "tx-1", "operation": "capture", "status": "pending",
		"unsigned_transaction": unsignedFor(t, "capture", "5000000")}}
	captureAmount, capturePrivateKey = "1.00", testKeyHex
	t.Cleanup(func() { captureAmount, capturePrivateKey = "", "" })

	var err error
	_ = captureStdout(t, func() { err = paymentsCaptureCmd.RunE(paymentsCaptureCmd, []string{lcRail0ID}) })
	if err == nil || !strings.Contains(err.Error(), "5000000") {
		t.Fatalf("capture calldata for another amount must be refused, got %v", err)
	}
	if n := g.sent("POST /payments/" + lcRail0ID + "/capture"); n != 0 {
		t.Errorf("submitted %d time(s) after a refusal", n)
	}
}

// A void prepare answered with capture calldata — settling the escrow to the merchant
// instead of returning it — is refused on the selector.
func TestVoidRefusesCaptureCalldata(t *testing.T) {
	g := newLifecycleGateway(t)
	g.prepare["void"] = []any{map[string]any{"id": "tx-v", "operation": "void", "status": "pending",
		"unsigned_transaction": unsignedFor(t, "capture", "1000000")}}
	voidPrivateKey = testKeyHex
	t.Cleanup(func() { voidPrivateKey = "" })

	var err error
	_ = captureStdout(t, func() { err = paymentsVoidCmd.RunE(paymentsVoidCmd, []string{lcRail0ID}) })
	if err == nil || !strings.Contains(err.Error(), "capture()") {
		t.Fatalf("capture calldata on a void must be refused, got %v", err)
	}
	if n := g.sent("POST /payments/" + lcRail0ID + "/void"); n != 0 {
		t.Errorf("submitted %d time(s) after a refusal", n)
	}
}

// ── payments create ──────────────────────────────────────────────────────────

const (
	lcCreatePayee    = "0x7777777777777777777777777777777777777777"
	lcCreateContract = "0x4CCC4DdeBB8A63B9186936A8C0FA404910A4311b"
)

// createGateway is a fake gateway for `payments create`: POST /payments answers with
// `created` (the test edits it to play a hostile or replaying gateway), and PUT
// …/sign records that a signature was deposited.
type createGateway struct {
	mu       sync.Mutex
	status   int
	created  map[string]any
	requests []string
}

func (g *createGateway) sent(methodPath string) int {
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

// newCreateGateway serves a well-formed create for 1.00 of lcToken (6 decimals) from
// the test key to lcCreatePayee, with a signing payload verifySigningPayload accepts,
// and points `payments create` at it through a payment file.
func newCreateGateway(t *testing.T) *createGateway {
	t.Helper()
	isolate(t)
	isolateTokenStore(t)
	t.Setenv("RAIL0_PRIVATE_KEY", "")
	seedDecimals(t, lcChainID, lcToken, 6)
	payer := addressForTestKey(t)
	sp := payload(rail0.PrimaryTypeReceive)
	sp.Domain.ChainId = lcChainID
	sp.Domain.VerifyingContract = rail0.Address(lcToken)
	sp.Message.From = rail0.Address(payer)
	sp.Message.To = rail0.Address(lcCreateContract)
	sp.Message.Value = "1000000"
	g := &createGateway{status: http.StatusCreated, created: map[string]any{
		"id": "pay-new", "rail0_id": lcRail0ID, "status": "unsigned", "chain_id": lcChainID,
		"mode": "authorize", "token": lcToken, "amount": "1000000", "payer": payer,
		"payee": lcCreatePayee, "rail0_contract": lcCreateContract, "signing_payload": sp,
	}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		g.requests = append(g.requests, r.Method+" "+r.URL.Path)
		g.mu.Unlock()
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/payments":
			g.mu.Lock()
			body := g.created
			g.mu.Unlock()
			w.WriteHeader(g.status)
			_ = json.NewEncoder(w).Encode(body)
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/sign"):
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "pay-new", "rail0_id": lcRail0ID, "status": "signed"})
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RAIL0_BASE_URL", srv.URL)
	if err := saveTokenForTest(makeJWT(t, map[string]any{"sub": payer, "exp": time.Now().Add(time.Hour).Unix()})); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	file := filepath.Join(t.TempDir(), "payment.json")
	body, _ := json.Marshal(map[string]any{
		"chain_id": lcChainID, "mode": "authorize", "amount": "1.00", "token": lcToken,
		"payer": payer, "payee": lcCreatePayee, "description": "order 42",
	})
	if err := os.WriteFile(file, body, 0o600); err != nil {
		t.Fatal(err)
	}
	prev := signCreateFlags
	signCreateFlags = createPaymentFlags{paymentFile: file}
	signPrivateKey = testKeyHex
	t.Cleanup(func() { signCreateFlags, signPrivateKey, signCreateIdempotencyKey = prev, "", "" })
	return g
}

// The happy path is undisturbed: created, checked, signed, deposited once.
func TestCreateSignsAPaymentMatchingTheRequest(t *testing.T) {
	g := newCreateGateway(t)
	_ = captureStdout(t, func() {
		if err := paymentsCreateCmd.RunE(paymentsCreateCmd, nil); err != nil {
			t.Fatalf("create: %v", err)
		}
	})
	if n := g.sent("PUT /payments/pay-new/sign"); n != 1 {
		t.Errorf("signature deposited %d time(s), want 1", n)
	}
}

// A gateway that records a DIFFERENT payment than the one asked for — another payee,
// another mode, a larger amount — must not get the payer's signature for it.
func TestCreateRefusesAPaymentOtherThanRequested(t *testing.T) {
	for field, value := range map[string]any{
		"payee":    testAttacker,
		"payer":    testAttacker,
		"mode":     "charge",
		"amount":   "9000000",
		"token":    testPayloadToken,
		"chain_id": 1,
	} {
		t.Run(field, func(t *testing.T) {
			g := newCreateGateway(t)
			g.created[field] = value
			var err error
			_ = captureStdout(t, func() { err = paymentsCreateCmd.RunE(paymentsCreateCmd, nil) })
			if err == nil || !strings.Contains(err.Error(), "refusing to sign") || !strings.Contains(err.Error(), field) {
				t.Fatalf("a created payment with another %s must be refused by name, got %v", field, err)
			}
			if n := g.sent("PUT /payments/pay-new/sign"); n != 0 {
				t.Errorf("signed %d time(s) after a refusal", n)
			}
		})
	}
}

// A re-run with the same --idempotency-key after a create that went through: the
// gateway replays the already-signed payment, with no signing_payload. That is the
// retry succeeding — it must exit 0 with the payment, and deposit nothing.
func TestCreateReRunWithAKeyReturnsTheSignedPayment(t *testing.T) {
	g := newCreateGateway(t)
	g.status = http.StatusOK
	g.created["status"] = "signed"
	delete(g.created, "signing_payload")
	signCreateIdempotencyKey = "order-42"

	var err error
	out := captureStdout(t, func() { err = paymentsCreateCmd.RunE(paymentsCreateCmd, nil) })
	if err != nil {
		t.Fatalf("a replayed, already-signed create must succeed, got %v", err)
	}
	if !strings.Contains(out, `"signed"`) || !strings.Contains(out, lcRail0ID) {
		t.Errorf("the result must be the replayed payment, got:\n%s", out)
	}
	if n := g.sent("PUT /payments/pay-new/sign"); n != 0 {
		t.Errorf("deposited a signature %d time(s) on an already-signed payment", n)
	}
}

// Still `unsigned` with no payload is NOT a replay to report as done: the payment
// needs a signature the gateway could not say what to sign over.
func TestCreateWithoutAPayloadOnAnUnsignedPaymentFails(t *testing.T) {
	g := newCreateGateway(t)
	delete(g.created, "signing_payload")

	var err error
	_ = captureStdout(t, func() { err = paymentsCreateCmd.RunE(paymentsCreateCmd, nil) })
	if err == nil || !strings.Contains(err.Error(), "signing_payload") || !strings.Contains(err.Error(), "payments sign") {
		t.Fatalf("an unsigned payment with no payload must fail and point at 'payments sign', got %v", err)
	}
	if n := g.sent("PUT /payments/pay-new/sign"); n != 0 {
		t.Errorf("deposited a signature %d time(s) with nothing to sign", n)
	}
}

// A keyed re-run whose original transaction FAILED: the replay is final, so it must
// exit non-zero with the decoded reason — and say a retry needs a new key, since
// this one can only ever bring the failed row back. Nothing is submitted.
func TestCaptureReRunOfAFailedTransactionFails(t *testing.T) {
	for _, wait := range []bool{false, true} {
		g := newLifecycleGateway(t)
		g.prepare["capture"] = []any{map[string]any{"id": "tx-1", "operation": "capture", "status": "failed",
			"error_code": "authorization_expired", "error_detail": "the authorization window closed"}}
		idempotencyKey, captureAmount, capturePrivateKey, waitFlag = "order-42", "1.00", testKeyHex, wait
		t.Cleanup(func() { captureAmount, capturePrivateKey = "", "" })

		var err error
		_ = captureStdout(t, func() { err = paymentsCaptureCmd.RunE(paymentsCaptureCmd, []string{lcRail0ID}) })
		if err == nil {
			t.Fatalf("wait=%v: a replayed failed capture exited 0", wait)
		}
		for _, want := range []string{"capture failed on-chain", "the authorization window closed", "new --idempotency-key"} {
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(want)) {
				t.Errorf("wait=%v: error %q is missing %q", wait, err, want)
			}
		}
		if exitCode(err) == 0 {
			t.Errorf("wait=%v: exit code 0 for a failed operation", wait)
		}
		if n := g.sent("POST /payments/" + lcRail0ID + "/capture"); n != 0 {
			t.Errorf("wait=%v: the replayed capture was submitted %d time(s)", wait, n)
		}
	}
}
