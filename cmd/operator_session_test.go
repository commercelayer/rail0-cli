package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/token"
)

// signInGateway answers the sign-in flow, saying whether the account is an operator's,
// and records every request with the Authorization header it carried.
type signInGateway struct {
	mu       sync.Mutex
	operator bool
	jwt      string
	seen     []string // "METHOD path|Authorization"
}

func newSignInGateway(t *testing.T, operator bool) *signInGateway {
	t.Helper()
	isolate(t)
	isolateTokenStore(t)
	g := &signInGateway{operator: operator}
	g.jwt = makeJWT(t, map[string]any{"sub": addressForTestKey(t), "account_id": "acc-1", "exp": time.Now().Add(time.Hour).Unix()})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		g.seen = append(g.seen, r.Method+" "+r.URL.Path+"|"+r.Header.Get("Authorization"))
		g.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/nonces":
			_ = json.NewEncoder(w).Encode(map[string]string{"nonce": "abc123nonce", "expires_at": "2030-01-01T00:00:00Z"})
		case "/auth":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token": g.jwt, "address": addressForTestKey(t), "expires_at": "2030-01-01T00:00:00Z", "admin": g.operator,
			})
		case "/auth/logout":
			_ = json.NewEncoder(w).Encode(map[string]any{"revoked": true})
		default:
			if r.Header.Get("Authorization") == "" {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"code": "missing_authorization", "title": "No session", "detail": "Sign in."})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "p_1", "status": "signed"})
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RAIL0_BASE_URL", srv.URL)
	t.Setenv("RAIL0_PRIVATE_KEY", "")
	return g
}

// sent returns the Authorization header of the last request to "METHOD path", and
// whether there was one.
func (g *signInGateway) sent(what string) (string, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i := len(g.seen) - 1; i >= 0; i-- {
		if req, auth, _ := strings.Cut(g.seen[i], "|"); req == what {
			return auth, true
		}
	}
	return "", false
}

func isOperatorSessionError(err error) bool {
	var se *sessionError
	return errors.As(err, &se) && se.Code == "operator_session" && exitCode(err) == 4
}

// A person's sign-in is stored with what the gateway said about it: an operator's is
// marked, and a later sign-in that is not an operator's removes the mark.
func TestSignInRemembersWhetherTheSessionIsAnOperators(t *testing.T) {
	g := newSignInGateway(t, true)
	base := resolveBaseURL()
	ctx := context.Background()
	if err := ensureSession(ctx, newClient(), testKeyHex, addressForTestKey(t)); err != nil {
		t.Fatalf("ensureSession: %v", err)
	}
	if token.Load(base) != g.jwt || !token.IsOperator(base) {
		t.Fatal("an operator's session must be stored and marked")
	}
	// A person is not refused anything: the client carries the session.
	if got := bearerToken(base); got != g.jwt {
		t.Errorf("outside agent mode the client carries %q, want the session", got)
	}

	g.operator = false
	g.jwt = makeJWT(t, map[string]any{"sub": addressForTestKey(t), "exp": time.Now().Add(2 * time.Hour).Unix()})
	_ = token.Delete(base)
	_ = token.MarkOperator(base, "a marker left behind")
	if err := ensureSession(ctx, newClient(), testKeyHex, addressForTestKey(t)); err != nil {
		t.Fatalf("ensureSession: %v", err)
	}
	if token.IsOperator(base) {
		t.Error("a session that is not an operator's reads as one")
	}
}

// With an operator's session stored, agent mode does not carry it: the request goes out
// with no Authorization, and whatever needs a session says why.
func TestAgentModeDoesNotCarryAStoredOperatorSession(t *testing.T) {
	g := newSignInGateway(t, true)
	base := resolveBaseURL()
	if err := saveTokenForTest(g.jwt); err != nil {
		t.Fatal(err)
	}
	rememberSessionRole(base, g.jwt, true)

	withAgentMode(t)
	if !operatorSessionWithheld(base) {
		t.Fatal("the stored operator session is not withheld in agent mode")
	}
	if got := bearerToken(base); got != "" {
		t.Errorf("agent mode carries %q", got)
	}
	_, err := newReadClient().Payments.Get(context.Background(), "p_1")
	if auth, ok := g.sent("GET /payments/p_1"); !ok || auth != "" {
		t.Errorf("the request carried Authorization %q (sent: %v)", auth, ok)
	}
	// The gateway's 401 is explained, not passed on as "sign in".
	if explained := explainWithheldSession(err); !isOperatorSessionError(explained) {
		t.Errorf("401 with the session withheld = %v, want operator_session", explained)
	}
	if _, err := currentAccountID(); !isOperatorSessionError(err) {
		t.Errorf("currentAccountID = %v, want operator_session", err)
	}
	if _, err := sessionAddress(); !isOperatorSessionError(err) {
		t.Errorf("sessionAddress = %v, want operator_session", err)
	}
	// A command signing in with that same key gets the same answer, without asking the
	// gateway again.
	if err := ensureSession(context.Background(), newClient(), testKeyHex, addressForTestKey(t)); !isOperatorSessionError(err) {
		t.Errorf("ensureSession with the operator's key = %v, want operator_session", err)
	}
	if _, asked := g.sent("POST /auth"); asked {
		t.Error("a sign-in was attempted with a key already known to be an operator's")
	}
	if token.Load(base) != g.jwt {
		t.Error("the stored session was touched: it is the person's, and stays")
	}
}

// Other failures are not rewritten, and a 401 is not either when nothing was withheld.
func TestOnlyAWithheldSessionExplainsA401(t *testing.T) {
	newSignInGateway(t, false)
	unauthorized := &rail0.APIError{Status: 401, Code: "token_expired"}
	withAgentMode(t)
	if got := explainWithheldSession(unauthorized); got != error(unauthorized) {
		t.Errorf("no operator session stored: 401 became %v", got)
	}
	if err := saveTokenForTest("some-jwt"); err != nil {
		t.Fatal(err)
	}
	rememberSessionRole(resolveBaseURL(), "some-jwt", true)
	forbidden := &rail0.APIError{Status: 403, Code: "forbidden"}
	if got := explainWithheldSession(forbidden); got != error(forbidden) {
		t.Errorf("a 403 became %v", got)
	}
	if got := explainWithheldSession(unauthorized); !isOperatorSessionError(got) {
		t.Errorf("401 with an operator session withheld = %v", got)
	}
}

// A sign-in made in agent mode that turns out to be an operator's: revoked on the
// gateway, taken off the client, never stored.
func TestAgentModeRefusesAnOperatorSignIn(t *testing.T) {
	g := newSignInGateway(t, true)
	base := resolveBaseURL()
	withAgentMode(t)
	client := newClient()
	ctx := context.Background()
	if err := ensureSession(ctx, client, testKeyHex, addressForTestKey(t)); !isOperatorSessionError(err) {
		t.Fatalf("ensureSession = %v, want operator_session", err)
	}
	if auth, ok := g.sent("POST /auth/logout"); !ok || auth != "Bearer "+g.jwt {
		t.Errorf("the session was not revoked with its own token (sent %v, Authorization %q)", ok, auth)
	}
	if token.Load(base) != "" {
		t.Error("an operator's session was stored by agent mode")
	}
	_, _ = client.Payments.Get(ctx, "p_1")
	if auth, _ := g.sent("GET /payments/p_1"); auth != "" {
		t.Errorf("the client still carries the refused session: %q", auth)
	}
}

// The same key's sign-in when the account is not an operator's works as it always did.
func TestAgentModeSignsInWithAnOrdinaryKey(t *testing.T) {
	g := newSignInGateway(t, false)
	withAgentMode(t)
	client := newClient()
	if err := ensureSession(context.Background(), client, testKeyHex, addressForTestKey(t)); err != nil {
		t.Fatalf("ensureSession: %v", err)
	}
	if token.Load(resolveBaseURL()) != g.jwt || token.IsOperator(resolveBaseURL()) {
		t.Error("an ordinary session must be stored, unmarked")
	}
	if _, revoked := g.sent("POST /auth/logout"); revoked {
		t.Error("an ordinary session was revoked")
	}
	if _, err := client.Payments.Get(context.Background(), "p_1"); err != nil {
		t.Errorf("the session is not usable: %v", err)
	}
}

// 'auth login' in agent mode with an operator's key: refused, and the session a person
// stored before is left exactly as it was, with its write-mode token.
func TestAuthLoginInAgentModeRefusesAnOperatorKey(t *testing.T) {
	g := newSignInGateway(t, true)
	base := resolveBaseURL()
	merchant := makeJWT(t, map[string]any{"sub": "0x9999999999999999999999999999999999999999", "exp": time.Now().Add(time.Hour).Unix()})
	if err := saveTokenForTest(merchant); err != nil {
		t.Fatal(err)
	}
	loginPrivateKey = testKeyHex
	t.Cleanup(func() { loginPrivateKey = "" })
	withAgentMode(t)

	err := authLoginCmd.RunE(authLoginCmd, nil)
	if !isOperatorSessionError(err) {
		t.Fatalf("auth login = %v, want operator_session", err)
	}
	if _, revoked := g.sent("POST /auth/logout"); !revoked {
		t.Error("the operator session opened by the login was not revoked")
	}
	if token.Load(base) != merchant || token.IsOperator(base) {
		t.Error("the session stored before the refused login was replaced or marked")
	}
}

// A person's 'auth login' with an operator's key stores it, marks it, and 'auth status'
// says so; in agent mode the status also says the session is not used there.
func TestAuthLoginAndStatusForAnOperator(t *testing.T) {
	g := newSignInGateway(t, true)
	base := resolveBaseURL()
	loginPrivateKey = testKeyHex
	t.Cleanup(func() { loginPrivateKey = "" })
	var err error
	captureStderr(t, func() { captureStdout(t, func() { err = authLoginCmd.RunE(authLoginCmd, nil) }) })
	if err != nil {
		t.Fatalf("auth login: %v", err)
	}
	if token.Load(base) != g.jwt || !token.IsOperator(base) {
		t.Fatal("a person's operator sign-in must be stored and marked")
	}
	out := captureStdout(t, func() { err = authStatusCmd.RunE(authStatusCmd, nil) })
	if err != nil || !strings.Contains(out, `"operator": true`) || strings.Contains(out, "usable_in_agent_mode") {
		t.Errorf("auth status: %v\n%s", err, out)
	}
	withAgentMode(t)
	out = captureStdout(t, func() { err = authStatusCmd.RunE(authStatusCmd, nil) })
	if err != nil || !strings.Contains(out, `"operator":true`) || !strings.Contains(out, `"usable_in_agent_mode":false`) {
		t.Errorf("auth status in agent mode: %v\n%s", err, out)
	}
}
