package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rail0/cli/internal/token"
	"github.com/spf13/cobra"
)

// operatorGateway stands in for the gateway's operator routes. It records every request
// with the token it carried, and answers from respond (an empty list by default) with
// the paging headers a collection has.
type operatorGateway struct {
	mu      sync.Mutex
	reqs    []operatorRequest
	respond func(r *http.Request) (status int, body any)
	total   int // X-Total-Count of every answer

	session string // the stored session token
}

type operatorRequest struct {
	method string
	path   string
	query  url.Values
	token  string // the bearer token, without "Bearer "
	body   []byte
}

// json decodes the request's body, which must be a JSON object.
func (r operatorRequest) json(t *testing.T) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(r.body, &out); err != nil {
		t.Fatalf("%s %s: body %q is not a JSON object: %v", r.method, r.path, r.body, err)
	}
	return out
}

func newOperatorGateway(t *testing.T) *operatorGateway {
	t.Helper()
	isolate(t)
	isolateTokenStore(t)
	g := &operatorGateway{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		g.mu.Lock()
		g.reqs = append(g.reqs, operatorRequest{
			method: r.Method, path: r.URL.Path, query: r.URL.Query(),
			token: strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "),
			body:  raw,
		})
		total := g.total
		g.mu.Unlock()
		status, body := http.StatusOK, any([]any{})
		if g.respond != nil {
			if s, b := g.respond(r); s != 0 {
				status, body = s, b
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", strconv.Itoa(total))
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RAIL0_BASE_URL", srv.URL)
	g.session = makeJWT(t, map[string]any{"sub": addressForTestKey(t), "account_id": "acc-1", "kind": "session", "exp": time.Now().Add(time.Hour).Unix()})
	if err := saveTokenForTest(g.session); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	return g
}

func (g *operatorGateway) requests() []operatorRequest {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]operatorRequest(nil), g.reqs...)
}

func (g *operatorGateway) last(t *testing.T, method, path string) operatorRequest {
	t.Helper()
	reqs := g.requests()
	for i := len(reqs) - 1; i >= 0; i-- {
		if reqs[i].method == method && reqs[i].path == path {
			return reqs[i]
		}
	}
	t.Fatalf("no %s %s was sent (got %v)", method, path, reqs)
	return operatorRequest{}
}

func (g *operatorGateway) sent(method, path string) bool {
	for _, r := range g.requests() {
		if r.method == method && r.path == path {
			return true
		}
	}
	return false
}

// writeModeOn stores a write-mode token of the session's address and returns it.
func writeModeOn(t *testing.T) string {
	t.Helper()
	wt := makeJWT(t, map[string]any{"sub": addressForTestKey(t), "admin_write": true, "exp": time.Now().Add(time.Hour).Unix()})
	if err := token.SaveWriteMode(resolveBaseURL(), wt); err != nil {
		t.Fatal(err)
	}
	return wt
}

// runE runs a command the way cobra would after parsing, and returns its stdout and error.
func runE(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	var err error
	out := captureStdout(t, func() { err = cmd.RunE(cmd, args) })
	return out, err
}

// Every command under `rail0 admin` refuses in agent mode, before anything is sent, and
// is listed as such in the help data — from the tree, so a new one cannot be forgotten.
func TestOperatorCommandsAreNotForAgents(t *testing.T) {
	g := newOperatorGateway(t)
	cmds := operatorCommands()
	if len(cmds) < 8 {
		t.Fatalf("found %d operator commands, the walk is broken", len(cmds))
	}
	withAgentMode(t)
	for _, c := range cmds {
		if why := agentUnavailable[c.CommandPath()]; why != operatorCommandsReason {
			t.Errorf("%s: not listed as unavailable in agent mode", c.CommandPath())
		}
		err := c.RunE(c, []string{"0x" + strings.Repeat("ab", 32)})
		if code := agentErrorCode(err, exitCode(err)); err == nil || code != "unavailable_in_agent_mode" || exitCode(err) != ExitCodeUsage {
			t.Errorf("%s: got %v (%s), want unavailable_in_agent_mode", c.CommandPath(), err, code)
		}
	}
	if n := len(g.requests()); n != 0 {
		t.Errorf("%d requests were sent in agent mode, want none", n)
	}
}

// While write mode is on, every client carries the write-mode token — and never in
// agent mode, whatever is stored.
func TestBearerTokenCarriesWriteModeOnlyForAPerson(t *testing.T) {
	g := newOperatorGateway(t)
	base := resolveBaseURL()
	if got := bearerToken(base); got != g.session {
		t.Errorf("write mode off: carries %q, want the session token", got)
	}
	wt := writeModeOn(t)
	if got := bearerToken(base); got != wt {
		t.Errorf("write mode on: carries %q, want the write-mode token", got)
	}
	func() {
		withAgentMode(t)
		if got := bearerToken(base); got != g.session {
			t.Errorf("agent mode: carries %q, want the session token", got)
		}
	}()
}

// A write-mode token that cannot be this session's is dropped, not carried: expired,
// issued to another address, or left behind with no session at all.
func TestStaleWriteModeTokenIsDropped(t *testing.T) {
	cases := map[string]func(t *testing.T) string{
		"expired": func(t *testing.T) string {
			return makeJWT(t, map[string]any{"sub": addressForTestKey(t), "exp": time.Now().Add(-time.Minute).Unix()})
		},
		"another address": func(t *testing.T) string {
			return makeJWT(t, map[string]any{"sub": "0x000000000000000000000000000000000000dEaD", "exp": time.Now().Add(time.Hour).Unix()})
		},
		"not a token": func(*testing.T) string { return "garbage" },
	}
	for name, mint := range cases {
		t.Run(name, func(t *testing.T) {
			g := newOperatorGateway(t)
			base := resolveBaseURL()
			if err := token.SaveWriteMode(base, mint(t)); err != nil {
				t.Fatal(err)
			}
			if got := bearerToken(base); got != g.session {
				t.Errorf("carries %q, want the session token", got)
			}
			if token.LoadWriteMode(base) != "" {
				t.Error("the stale write-mode token was left on disk")
			}
		})
	}
	t.Run("no session", func(t *testing.T) {
		newOperatorGateway(t)
		base := resolveBaseURL()
		writeModeOn(t)
		if err := token.Delete(base); err != nil {
			t.Fatal(err)
		}
		if got := bearerToken(base); got != "" {
			t.Errorf("carries %q with no session stored", got)
		}
	})
}

// 'write-mode on' asks with the session token, stores what it gets, and prints the
// expiry — never the token.
func TestWriteModeOnStoresTheTokenAndNeverPrintsIt(t *testing.T) {
	g := newOperatorGateway(t)
	issued := makeJWT(t, map[string]any{"sub": addressForTestKey(t), "admin_write": true, "exp": time.Now().Add(time.Hour).Unix()})
	g.respond = func(r *http.Request) (int, any) {
		if r.Method == http.MethodPost && r.URL.Path == "/admin/write_mode" {
			return http.StatusCreated, map[string]any{"token": issued, "expires_at": "2030-01-01T00:00:00Z"}
		}
		return 0, nil
	}
	var out string
	var err error
	stderr := captureStderr(t, func() { out, err = runE(t, adminWriteModeOnCmd) })
	if err != nil {
		t.Fatalf("write-mode on: %v", err)
	}
	if req := g.last(t, http.MethodPost, "/admin/write_mode"); req.token != g.session {
		t.Errorf("asked with %q, want the session token", req.token)
	}
	if token.LoadWriteMode(resolveBaseURL()) != issued {
		t.Error("the issued token was not stored")
	}
	if strings.Contains(out, issued) || strings.Contains(stderr, issued) {
		t.Errorf("the write-mode token was printed:\n%s\n%s", out, stderr)
	}
	var info map[string]any
	if json.Unmarshal([]byte(out), &info) != nil || info["active"] != true || info["expires_at"] == nil {
		t.Errorf("output = %s", out)
	}
	if !strings.Contains(stderr, "Write mode is on until") {
		t.Errorf("stderr = %q", stderr)
	}
}

// 'write-mode off' revokes the stored token on the gateway, presenting THAT token, and
// removes it; with nothing stored it sends nothing.
func TestWriteModeOff(t *testing.T) {
	g := newOperatorGateway(t)
	g.respond = func(r *http.Request) (int, any) {
		if r.Method == http.MethodDelete && r.URL.Path == "/admin/write_mode" {
			return http.StatusOK, map[string]any{"revoked": true}
		}
		return 0, nil
	}
	wt := writeModeOn(t)
	out, err := runE(t, adminWriteModeOffCmd)
	if err != nil {
		t.Fatalf("write-mode off: %v", err)
	}
	if req := g.last(t, http.MethodDelete, "/admin/write_mode"); req.token != wt {
		t.Errorf("revoked presenting %q, want the write-mode token", req.token)
	}
	if token.LoadWriteMode(resolveBaseURL()) != "" {
		t.Error("the write-mode token is still stored")
	}
	if !strings.Contains(out, `"revoked": true`) || !strings.Contains(out, `"was_active": true`) {
		t.Errorf("output = %s", out)
	}

	before := len(g.requests())
	out, err = runE(t, adminWriteModeOffCmd)
	if err != nil || !strings.Contains(out, `"was_active": false`) || len(g.requests()) != before {
		t.Errorf("off with nothing stored: %v, %s, %d new requests", err, out, len(g.requests())-before)
	}
}

// Signing out ends write mode first, and then signs out the SESSION: the logout must not
// present the write-mode token, or the gateway would revoke that one and leave the
// session alive.
func TestLogoutEndsWriteModeAndSignsOutTheSession(t *testing.T) {
	g := newOperatorGateway(t)
	g.respond = func(r *http.Request) (int, any) {
		return http.StatusOK, map[string]any{"revoked": true}
	}
	wt := writeModeOn(t)
	res := revokeSession(resolveBaseURL())
	if !res.Had || !res.Revoked {
		t.Errorf("revocation = %+v", res)
	}
	if req := g.last(t, http.MethodDelete, "/admin/write_mode"); req.token != wt {
		t.Errorf("write mode ended presenting %q, want the write-mode token", req.token)
	}
	if req := g.last(t, http.MethodPost, "/auth/logout"); req.token != g.session {
		t.Errorf("signed out presenting %q, want the session token", req.token)
	}
	if token.LoadWriteMode(resolveBaseURL()) != "" || token.Load(resolveBaseURL()) != "" {
		t.Error("a token is still stored after the sign-out")
	}
}

// The verdict decides the exit code, after the report is printed.
func TestAdminHealthFailsOnAVerdictThatIsNotOK(t *testing.T) {
	g := newOperatorGateway(t)
	status := "degraded"
	g.respond = func(r *http.Request) (int, any) {
		switch r.URL.Path {
		case "/admin/health":
			return http.StatusOK, map[string]any{"status": status, "checks": map[string]any{
				"workers":        map[string]any{"status": "ok"},
				"sync_callbacks": map[string]any{"status": "degraded", "unapplied": 827},
				"broadcasts":     map[string]any{"status": "error"},
			}}
		case "/admin/rpc_health":
			return http.StatusOK, map[string]any{"status": "error", "chains": []any{
				map[string]any{"chain_id": 5042002, "name": "Arc Testnet", "status": "error"},
				map[string]any{"chain_id": 1, "name": "Ethereum", "status": "ok"},
			}}
		}
		return 0, nil
	}
	out, err := runE(t, adminHealthCmd)
	if err == nil || exitCode(err) != 1 {
		t.Fatalf("degraded: err = %v (exit %d), want exit 1", err, exitCode(err))
	}
	if msg := err.Error(); !strings.Contains(msg, `"degraded"`) || !strings.Contains(msg, "broadcasts (error), sync_callbacks (degraded)") {
		t.Errorf("message = %q", msg)
	}
	if !strings.Contains(out, `"unapplied": 827`) {
		t.Errorf("the report must still be printed:\n%s", out)
	}

	adminHealthRPC = true
	t.Cleanup(func() { adminHealthRPC = false })
	out, err = runE(t, adminHealthCmd)
	if err == nil || !strings.Contains(err.Error(), "Arc Testnet (error)") || strings.Contains(err.Error(), "Ethereum") {
		t.Errorf("--rpc: err = %v", err)
	}
	if !g.sent(http.MethodGet, "/admin/rpc_health") || !strings.Contains(out, "Arc Testnet") {
		t.Errorf("--rpc did not read the RPC probe:\n%s", out)
	}

	adminHealthRPC = false
	status = "ok"
	g.respond = func(r *http.Request) (int, any) {
		return http.StatusOK, map[string]any{"status": "ok", "checks": map[string]any{"workers": map[string]any{"status": "ok"}}}
	}
	if _, err := runE(t, adminHealthCmd); err != nil {
		t.Errorf("ok: err = %v", err)
	}
}

func TestAdminTransactionsFiltersAndAmount(t *testing.T) {
	g := newOperatorGateway(t)
	g.respond = func(r *http.Request) (int, any) {
		if r.URL.Path == "/admin/transactions" {
			return http.StatusOK, []any{map[string]any{
				"id": "tx1", "operation": "capture", "status": "failed", "amount": "2500000", "error_reason": "nonce too low",
				"payment": map[string]any{"id": "p1", "decimals": 6, "chain_id": 5042002},
			}}
		}
		return 0, nil
	}
	adminTxStuck, adminTxStatuses, adminTxAccount, adminTxHash = "unconfirmed", []string{"failed", "pending"}, "acc-9", "0xfeed"
	t.Cleanup(func() { adminTxStuck, adminTxStatuses, adminTxAccount, adminTxHash = "", nil, "", "" })

	out, err := runE(t, adminTransactionsCmd)
	if err != nil {
		t.Fatalf("admin transactions: %v", err)
	}
	q := g.last(t, http.MethodGet, "/admin/transactions").query
	if q.Get("stuck") != "unconfirmed" || q.Get("status") != "failed,pending" || q.Get("account_id") != "acc-9" || q.Get("hash") != "0xfeed" {
		t.Errorf("query = %v", q)
	}
	if !strings.Contains(out, `"amount_human": "2.5"`) || !strings.Contains(out, `"error_reason": "nonce too low"`) {
		t.Errorf("output = %s", out)
	}

	adminTxStuck = "sometimes"
	if _, err := runE(t, adminTransactionsCmd); err == nil || exitCode(err) != ExitCodeUsage {
		t.Errorf("--stuck sometimes: err = %v, want a usage error", err)
	}
}

func resetSyncErrorFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		syncErrReason, syncErrOutcome, syncErrTxHash, syncErrPayment = "", "", "", ""
		syncErrAll, assumeYes = false, false
	})
}

// What stands between the command and the DELETE: a filter (or an explicit --all, never
// both), a session, write mode, and a confirmation.
func TestSyncErrorsClearGuards(t *testing.T) {
	g := newOperatorGateway(t)
	g.total = 12
	resetSyncErrorFlags(t)

	_, err := runE(t, adminSyncErrorsClearCmd)
	if err == nil || exitCode(err) != ExitCodeUsage {
		t.Errorf("no filter: err = %v, want a usage error", err)
	}

	syncErrReason, syncErrAll = "not_found", true
	_, err = runE(t, adminSyncErrorsClearCmd)
	if err == nil || agentErrorCode(err, exitCode(err)) != "flag_conflict" {
		t.Errorf("filter with --all: err = %v, want flag_conflict", err)
	}

	syncErrAll = false
	syncErrReason = "made_up"
	if _, err = runE(t, adminSyncErrorsClearCmd); err == nil || exitCode(err) != ExitCodeUsage {
		t.Errorf("unknown reason: err = %v, want a usage error", err)
	}

	syncErrReason = "not_found"
	_, err = runE(t, adminSyncErrorsClearCmd)
	if err == nil || exitCode(err) != 4 || agentErrorCode(err, exitCode(err)) != "write_mode_required" {
		t.Errorf("write mode off: err = %v (exit %d), want write_mode_required, exit 4", err, exitCode(err))
	}

	writeModeOn(t)
	_, err = runE(t, adminSyncErrorsClearCmd) // not a terminal, no --yes
	if err == nil || agentErrorCode(err, exitCode(err)) != "confirmation_required" {
		t.Errorf("no confirmation: err = %v, want confirmation_required", err)
	}

	if g.sent(http.MethodDelete, "/admin/sync_errors") {
		t.Error("a DELETE was sent by a refused command")
	}
}

// The clear counts with the filter, then deletes with the same filter, in write mode.
func TestSyncErrorsClearDeletesWhatTheFilterLists(t *testing.T) {
	g := newOperatorGateway(t)
	g.total = 12
	g.respond = func(r *http.Request) (int, any) {
		if r.Method == http.MethodDelete {
			return http.StatusOK, map[string]any{"deleted": 12}
		}
		return 0, nil
	}
	resetSyncErrorFlags(t)
	wt := writeModeOn(t)
	rail0ID := "0x" + strings.Repeat("ab", 32)
	syncErrReason, syncErrPayment, assumeYes = "not_found", rail0ID, true

	out, err := runE(t, adminSyncErrorsClearCmd)
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	count := g.last(t, http.MethodGet, "/admin/sync_errors")
	del := g.last(t, http.MethodDelete, "/admin/sync_errors")
	for _, req := range []operatorRequest{count, del} {
		if req.query.Get("reason") != "not_found" || req.query.Get("payment_id") != rail0ID {
			t.Errorf("%s query = %v, want the filter", req.method, req.query)
		}
	}
	if del.query.Get("per_page") != "" || len(del.query) != 2 {
		t.Errorf("DELETE query = %v, want the filter alone", del.query)
	}
	if del.token != wt {
		t.Errorf("DELETE carried %q, want the write-mode token", del.token)
	}
	if !strings.Contains(out, `"deleted": 12`) {
		t.Errorf("output = %s", out)
	}
}

// Nothing to delete: say so, and send no DELETE.
func TestSyncErrorsClearWithNothingMatching(t *testing.T) {
	g := newOperatorGateway(t)
	resetSyncErrorFlags(t)
	writeModeOn(t)
	syncErrReason = "unapplicable"
	out, err := runE(t, adminSyncErrorsClearCmd)
	if err != nil || !strings.Contains(out, `"deleted": 0`) {
		t.Errorf("err = %v, output = %s", err, out)
	}
	if g.sent(http.MethodDelete, "/admin/sync_errors") {
		t.Error("a DELETE was sent for an empty selection")
	}
}

// --all is the one way to send an unfiltered DELETE.
func TestSyncErrorsClearAll(t *testing.T) {
	g := newOperatorGateway(t)
	g.total = 3
	g.respond = func(r *http.Request) (int, any) {
		if r.Method == http.MethodDelete {
			return http.StatusOK, map[string]any{"deleted": 3}
		}
		return 0, nil
	}
	resetSyncErrorFlags(t)
	writeModeOn(t)
	syncErrAll, assumeYes = true, true
	if _, err := runE(t, adminSyncErrorsClearCmd); err != nil {
		t.Fatalf("clear --all: %v", err)
	}
	if q := g.last(t, http.MethodDelete, "/admin/sync_errors").query; len(q) != 0 {
		t.Errorf("DELETE query = %v, want none", q)
	}
}

// traceGateway answers the five reads of a trace for one payment.
func traceGateway(t *testing.T) (*operatorGateway, string) {
	t.Helper()
	g := newOperatorGateway(t)
	const uuid = "01a16403-398f-7f15-a8e5-8a9b78900e1a"
	g.respond = func(r *http.Request) (int, any) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/payments/"):
			return http.StatusOK, map[string]any{
				"id": uuid, "rail0_id": "0x" + strings.Repeat("95", 32), "status": "captured", "mode": "authorize",
				"amount": "2500000", "decimals": 6, "created_at": "2026-10-22T14:28:15.364Z", "signed_at": "2026-10-22T14:28:15.574Z",
				"transactions": []any{map[string]any{
					"id": "tx1", "operation": "capture", "status": "confirmed", "amount": "1000000",
					"confirmed_at": "2026-10-22T14:37:06.552Z", "transaction_hash": "0x" + strings.Repeat("f8", 32),
				}},
			}
		case r.URL.Path == "/admin/sync_errors":
			return http.StatusOK, []any{map[string]any{
				"id": "e1", "reason": "payment_mismatch", "outcome": "confirmed", "event_type": "captured",
				"transaction_hash": "0xbad", "created_at": "2026-10-22T14:40:00.000Z",
			}}
		case r.URL.Path == "/admin/event_callbacks":
			return http.StatusOK, []any{map[string]any{
				"id": "cb1", "topic": "payments.captured", "status": "failed", "response_code": 500,
				"payload": `{"large":"body"}`, "created_at": "2026-10-22T14:37:10.000Z",
				"webhook": map[string]any{"id": "wh1", "name": "Shop", "account_id": "acc-2"},
			}}
		case r.URL.Path == "/admin/disputes":
			return http.StatusOK, []any{map[string]any{
				"id": "d1", "status": "closed", "reason_code": "not_received", "opened_at": "2026-10-23T09:00:00.000Z",
				"closed_at": "2026-10-24T09:00:00.000Z", "close_reason_code": "withdrawn",
			}}
		case r.URL.Path == "/admin/actions":
			return http.StatusOK, []any{map[string]any{
				"id": "a1", "action": "transaction.redrive", "actor_address": "0x134a", "created_at": "2026-10-22T14:36:00.000Z",
			}}
		}
		return 0, nil
	}
	return g, uuid
}

// The trace reads the payment first and asks the four listings by its UUID — also when
// it was given the rail0_id, which the audit log does not take.
func TestAdminTraceSections(t *testing.T) {
	g, uuid := traceGateway(t)
	rail0ID := "0x" + strings.Repeat("95", 32)
	out, err := runE(t, adminTraceCmd, rail0ID)
	if err != nil {
		t.Fatalf("trace: %v", err)
	}
	if !g.sent(http.MethodGet, "/payments/"+rail0ID) {
		t.Errorf("the payment was not read by the id given: %v", g.requests())
	}
	for _, path := range []string{"/admin/sync_errors", "/admin/event_callbacks", "/admin/disputes", "/admin/actions"} {
		if q := g.last(t, http.MethodGet, path).query; q.Get("payment_id") != uuid {
			t.Errorf("%s asked with payment_id=%q, want the UUID", path, q.Get("payment_id"))
		}
	}
	var trace map[string]any
	if err := json.Unmarshal([]byte(out), &trace); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	for _, k := range []string{"payment", "sync_errors", "deliveries", "disputes", "actions", "totals"} {
		if trace[k] == nil {
			t.Errorf("section %q is missing", k)
		}
	}
	if p, _ := trace["payment"].(map[string]any); p["amount_human"] != "2.5" {
		t.Errorf("payment = %v", trace["payment"])
	}
	// The delivery keeps who it went to and loses its request body.
	if strings.Contains(out, "large") || !strings.Contains(out, `"account_id": "acc-2"`) {
		t.Errorf("deliveries = %v", trace["deliveries"])
	}
	if totals, _ := trace["totals"].(map[string]any); totals["transactions"] != float64(1) || totals["deliveries"] != float64(1) {
		t.Errorf("totals = %v", trace["totals"])
	}
}

// --timeline is the same facts in the order they happened.
func TestAdminTraceTimeline(t *testing.T) {
	_, uuid := traceGateway(t)
	adminTraceTimeline = true
	t.Cleanup(func() { adminTraceTimeline = false })
	out, err := runE(t, adminTraceCmd, uuid)
	if err != nil {
		t.Fatalf("trace --timeline: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("output is not a JSON list: %v\n%s", err, out)
	}
	var got []string
	for _, r := range rows {
		got = append(got, r["kind"].(string)+":"+r["event"].(string))
	}
	want := []string{
		"payment:created", "payment:signed", "operator:transaction.redrive", "transaction:capture",
		"delivery:payments.captured", "sync_error:captured", "dispute:opened", "dispute:closed",
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("timeline =\n  %v\nwant\n  %v", got, want)
	}
	for _, r := range rows {
		if r["kind"] == "delivery" && (r["code"] != float64(500) || r["detail"] != "Shop") {
			t.Errorf("delivery row = %v", r)
		}
	}
}

func TestAdminTraceRefusesWhatIsNotAPaymentID(t *testing.T) {
	g := newOperatorGateway(t)
	if _, err := runE(t, adminTraceCmd, "nope"); err == nil || exitCode(err) != ExitCodeUsage {
		t.Errorf("err = %v, want a usage error", err)
	}
	if len(g.requests()) != 0 {
		t.Error("a request was sent for an id that is not one")
	}
}
