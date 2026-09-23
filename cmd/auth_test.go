package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rail0/cli/internal/token"
)

func TestSIWEHost(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://api.rail0.xyz/", "api.rail0.xyz"},
		{"https://api.rail0.xyz", "api.rail0.xyz"},
		// Host-only: the port must be stripped so it matches the gateway's
		// expected SIWE domain ("localhost" in dev), not "localhost:9292".
		{"http://localhost:9292", "localhost"},
		{"http://localhost:9292/", "localhost"},
		{"http://127.0.0.1:9292", "127.0.0.1"},
		{"localhost:9292", "localhost"}, // scheme-less fallback
	}
	for _, tc := range cases {
		if got := siweHost(tc.in); got != tc.want {
			t.Errorf("siweHost(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// A JWT-shaped value, used to prove it never reaches the notice or the summary.
const fakeJWT = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIweGFh" +
	"YWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWEiLCJleHAiOjk5OTk5OTk5OTl9.c2lnbmF0dXJl"

// `auth login` used to print `Set RAIL0_TOKEN=<jwt>` when the token could not be
// saved, putting a live bearer credential into terminal scrollback, CI logs and any
// `2>` redirect (#64). The trigger is mundane — a read-only filesystem, a sandboxed
// HOME — so this is the path most likely to leak, and the least likely to be noticed.
func TestTokenSaveFailureNoticeNeverContainsTheToken(t *testing.T) {
	notice := tokenSaveFailureNotice(errors.New("open /nonexistent/token: read-only file system"))

	if strings.Contains(notice, fakeJWT) {
		t.Error("the notice must never carry the raw token")
	}
	// Nothing JWT-shaped at all: the check above only catches this exact fixture,
	// while a future edit might interpolate a different value.
	if regexp.MustCompile(`eyJ[A-Za-z0-9_-]{8,}`).MatchString(notice) {
		t.Errorf("the notice contains something JWT-shaped: %q", notice)
	}
	// It must still be actionable — the failure and the way out. The way out is no
	// longer "--token, then export RAIL0_TOKEN": that variable was removed (#60), so
	// printing the token buys nothing and the notice points at the cache directory
	// instead. Asserting the recovery route here, not just the absence of a secret,
	// is what caught the two changes drifting apart during the rebase.
	for _, want := range []string{"could not save token", "read-only file system", "sign in again"} {
		if !strings.Contains(notice, want) {
			t.Errorf("notice is missing %q: %q", want, notice)
		}
	}
}

// `auth logout` reports its outcome as a result, not as text on stdout, so --json and
// -q callers can read it — with the same none/revoked/abandoned vocabulary `env`
// uses for previous_session.
func TestLogoutResult(t *testing.T) {
	cases := []struct {
		revoked sessionRevocation
		want    string
	}{
		{sessionRevocation{}, "none"},
		{sessionRevocation{Had: true, Revoked: true}, "revoked"},
		{sessionRevocation{Had: true, Why: "gateway unreachable"}, "abandoned"},
	}
	for _, c := range cases {
		got := logoutResult("https://gw.test", c.revoked)
		if got["session"] != c.want {
			t.Errorf("%+v: session = %v, want %s", c.revoked, got["session"], c.want)
		}
		if got["base_url"] != "https://gw.test" {
			t.Errorf("base_url = %v", got["base_url"])
		}
	}
	if d := logoutResult("x", sessionRevocation{Had: true, Why: "gateway unreachable"})["session_detail"]; d != "gateway unreachable" {
		t.Errorf("an abandoned session must say why, got %v", d)
	}
}

// revoke-all signs a proof with the key it is given (the SDK carries the statement)
// and then drops this machine's session when — and only when — it belonged to that
// address: the cutoff killed it, while another address's session is still valid.
func TestRevokeAllDropsOnlyTheRevokedAddressSession(t *testing.T) {
	for _, tc := range []struct {
		name, sub, want string
	}{
		{"own session", "", "removed"},
		{"another address", "0x9999999999999999999999999999999999999999", "kept"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolate(t)
			isolateTokenStore(t)
			var revoked bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/auth/nonces":
					_ = json.NewEncoder(w).Encode(map[string]string{"nonce": "abc123nonce9", "expires_at": "2030-01-01T00:00:00Z"})
				case "/auth/revoke_all":
					revoked = true
					_ = json.NewEncoder(w).Encode(map[string]any{"revoked_all": true, "cutoff_at": "2026-09-23T12:00:00Z"})
				}
			}))
			defer srv.Close()
			t.Setenv("RAIL0_BASE_URL", srv.URL)
			t.Setenv("RAIL0_PRIVATE_KEY", "")

			sub := tc.sub
			if sub == "" {
				sub = addressForTestKey(t)
			}
			if err := saveTokenForTest(makeJWT(t, map[string]any{"sub": sub, "exp": time.Now().Add(time.Hour).Unix()})); err != nil {
				t.Fatal(err)
			}
			assumeYes, revokeAllPrivateKey = true, testKeyHex
			t.Cleanup(func() { assumeYes, revokeAllPrivateKey = false, "" })

			out := captureStdout(t, func() {
				if err := authRevokeAllCmd.RunE(authRevokeAllCmd, nil); err != nil {
					t.Errorf("revoke-all: %v", err)
				}
			})
			if !revoked {
				t.Fatal("POST /auth/revoke_all was never sent")
			}
			if !strings.Contains(out, `"local_session": "`+tc.want+`"`) || !strings.Contains(out, "2026-09-23T12:00:00Z") {
				t.Errorf("result should report local_session %q and the cutoff, got:\n%s", tc.want, out)
			}
			if stored := token.Load(srv.URL) != ""; stored != (tc.want == "kept") {
				t.Errorf("stored session present = %v, want %v", stored, tc.want == "kept")
			}
		})
	}
}
