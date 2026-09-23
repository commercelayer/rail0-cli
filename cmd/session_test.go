package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rail0/cli/internal/token"
)

// fakeGateway stands in for the RAIL0 gateway over the SIWE login flow plus one
// authenticated read, and records the Authorization header of every request it sees.
type fakeGateway struct {
	mu    sync.Mutex
	auths map[string]string // request path -> Authorization header as received
	jwt   string
}

func newFakeGateway(t *testing.T) (*fakeGateway, *httptest.Server) {
	t.Helper()
	g := &fakeGateway{
		auths: map[string]string{},
		jwt:   makeJWT(t, map[string]any{"sub": "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", "exp": time.Now().Add(time.Hour).Unix()}),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		g.auths[r.URL.Path] = r.Header.Get("Authorization")
		g.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/auth/nonces":
			_ = json.NewEncoder(w).Encode(map[string]string{"nonce": "abc123nonce", "expires_at": "2030-01-01T00:00:00Z"})
		case r.URL.Path == "/auth":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"token": g.jwt, "address": "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", "expires_at": "2030-01-01T00:00:00Z",
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "p_1", "status": "signed"})
		}
	}))
	t.Cleanup(srv.Close)
	return g, srv
}

func (g *fakeGateway) authFor(path string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.auths[path]
}

// isolateTokenStore points the token cache at a temp dir so a test never reads or
// clobbers the developer's real session.
func isolateTokenStore(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	// macOS resolves UserCacheDir to $HOME/Library/Caches, which must exist.
	if err := os.MkdirAll(filepath.Join(dir, "Library", "Caches"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// The regression in #59: on a fresh machine with no cached token, the client was
// built unauthenticated, ensureSession logged in on a THROWAWAY client, and the very
// next request went out with no Authorization header — a 401 immediately after
// "Signed in as 0x…".
func TestEnsureSessionAuthenticatesTheCallersClient(t *testing.T) {
	isolateTokenStore(t)
	g, srv := newFakeGateway(t)
	t.Setenv("RAIL0_BASE_URL", srv.URL)

	client := newClient() // no cached token: starts unauthenticated
	ctx := context.Background()

	if err := ensureSession(ctx, client, testKeyHex, addressForTestKey(t)); err != nil {
		t.Fatalf("ensureSession: %v", err)
	}

	// The read AFTER signing in must carry the freshly-issued JWT.
	if _, err := client.Payments.Get(ctx, "p_1"); err != nil {
		t.Fatalf("Payments.Get after ensureSession: %v", err)
	}
	got := g.authFor("/payments/p_1")
	if got == "" {
		t.Fatal("the request after ensureSession carried NO Authorization header — the session was lost (#59)")
	}
	if want := "Bearer " + g.jwt; got != want {
		t.Errorf("Authorization = %q, want the JWT issued by the login (%q)", got, want)
	}
}

// A stale cached token must be REPLACED, not kept alongside the new one. Seeding the
// JWT via ClientOptions.Token rather than a static header is what makes SetToken able
// to override it.
func TestEnsureSessionReplacesAStaleCachedToken(t *testing.T) {
	isolateTokenStore(t)
	g, srv := newFakeGateway(t)
	t.Setenv("RAIL0_BASE_URL", srv.URL)

	// A cached token for a DIFFERENT address: ensureSession must sign in again.
	stale := makeJWT(t, map[string]any{"sub": "0x9999999999999999999999999999999999999999", "exp": time.Now().Add(time.Hour).Unix()})
	if err := saveTokenForTest(stale); err != nil {
		t.Fatalf("seed stale token: %v", err)
	}

	client := newClient()
	ctx := context.Background()
	if err := ensureSession(ctx, client, testKeyHex, addressForTestKey(t)); err != nil {
		t.Fatalf("ensureSession: %v", err)
	}
	if _, err := client.Payments.Get(ctx, "p_1"); err != nil {
		t.Fatalf("Payments.Get: %v", err)
	}
	got := g.authFor("/payments/p_1")
	if strings.Contains(got, stale) {
		t.Errorf("the stale token was still sent: %q", got)
	}
	if want := "Bearer " + g.jwt; got != want {
		t.Errorf("Authorization = %q, want the re-issued JWT", got)
	}
}

// A cached token that already matches the address is reused without a new login, and
// the client built from it is authenticated — so the fast path still works.
func TestEnsureSessionReusesAMatchingCachedToken(t *testing.T) {
	isolateTokenStore(t)
	g, srv := newFakeGateway(t)
	t.Setenv("RAIL0_BASE_URL", srv.URL)

	addr := addressForTestKey(t)
	cached := makeJWT(t, map[string]any{"sub": addr, "exp": time.Now().Add(time.Hour).Unix()})
	if err := saveTokenForTest(cached); err != nil {
		t.Fatalf("seed cached token: %v", err)
	}

	client := newClient()
	ctx := context.Background()
	if err := ensureSession(ctx, client, testKeyHex, addr); err != nil {
		t.Fatalf("ensureSession: %v", err)
	}
	if a := g.authFor("/auth"); a != "" || g.authFor("/auth/nonces") != "" {
		t.Error("ensureSession logged in again despite a valid matching cached token")
	}
	if _, err := client.Payments.Get(ctx, "p_1"); err != nil {
		t.Fatalf("Payments.Get: %v", err)
	}
	if want := "Bearer " + cached; g.authFor("/payments/p_1") != want {
		t.Errorf("Authorization = %q, want the cached JWT", g.authFor("/payments/p_1"))
	}
}

// A matching session with seconds left is NOT reused: the 401 would land partway
// through prepare → sign → submit → wait, possibly after the broadcast. With --wait the
// session must also outlive the whole wait budget, since the polls use it.
func TestEnsureSessionSignsInAgainWhenTheCachedSessionIsAboutToExpire(t *testing.T) {
	for _, tc := range []struct {
		name    string
		left    time.Duration
		wait    bool
		reLogin bool
	}{
		{"20s left", 20 * time.Second, false, true},
		{"an hour left", time.Hour, false, false},
		{"10m left, --wait with a 15m budget", 10 * time.Minute, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateTokenStore(t)
			g, srv := newFakeGateway(t)
			t.Setenv("RAIL0_BASE_URL", srv.URL)
			waitFlag, waitTimeout = tc.wait, defaultWaitTimeout
			t.Cleanup(func() { waitFlag = false })

			addr := addressForTestKey(t)
			cached := makeJWT(t, map[string]any{"sub": addr, "exp": time.Now().Add(tc.left).Unix()})
			if err := saveTokenForTest(cached); err != nil {
				t.Fatalf("seed cached token: %v", err)
			}
			if err := ensureSession(context.Background(), newClient(), testKeyHex, addr); err != nil {
				t.Fatalf("ensureSession: %v", err)
			}
			g.mu.Lock()
			_, loggedIn := g.auths["/auth"]
			g.mu.Unlock()
			if loggedIn != tc.reLogin {
				t.Errorf("signed in again = %v, want %v", loggedIn, tc.reLogin)
			}
		})
	}
}

// addressForTestKey derives the address of testKeyHex, so the tests assert against
// the same value ensureSession compares the JWT's `sub` to.
func addressForTestKey(t *testing.T) string {
	t.Helper()
	addr, err := addressFromKey(testKeyHex)
	if err != nil {
		t.Fatalf("addressFromKey: %v", err)
	}
	return addr
}

// saveTokenForTest seeds the isolated token cache. Wraps token.Save so the import
// stays in one place.
func saveTokenForTest(jwt string) error { return token.Save(resolveBaseURL(), jwt) }
