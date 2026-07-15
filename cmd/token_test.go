package cmd

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	rail0 "github.com/rail0/go-sdk"
)

// makeJWT builds a display-only JWT (unsigned) whose payload carries the given
// claims. parseJWTClaims only base64url-decodes the payload segment and checks
// exp, so a real signature is unnecessary here.
func makeJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	seg := base64.RawURLEncoding.EncodeToString(payload)
	return "header." + seg + ".sig"
}

func TestHumanUntil(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{2 * time.Hour, "2h 0m"},
		{90 * time.Minute, "1h 30m"},
		{45 * time.Minute, "45m"},
		{30 * time.Second, "1m"},      // rounds up to the nearest minute
		{20 * time.Second, "expired"}, // rounds down to 0m → expired
		{0, "expired"},
		{-5 * time.Minute, "expired"},
		{25 * time.Hour, "25h 0m"},
	}
	for _, tc := range cases {
		if got := humanUntil(tc.in); got != tc.want {
			t.Errorf("humanUntil(%s) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTokenExpiry(t *testing.T) {
	exp := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	claims := map[string]any{"exp": float64(exp.Unix())}

	expiresAt, expiresIn := tokenExpiry(claims)
	if want := exp.UTC().Format(time.RFC3339); expiresAt != want {
		t.Errorf("expires_at = %q, want %q", expiresAt, want)
	}
	// ~2h out, rounded to the minute — allow either boundary for scheduling jitter.
	if expiresIn != "2h 0m" && expiresIn != "1h 59m" {
		t.Errorf("expires_in = %q, want ~2h", expiresIn)
	}
}

func TestTokenSession_NotLoggedIn(t *testing.T) {
	info, hint := tokenSession("")
	if info != nil {
		t.Errorf("info: got %v, want nil", info)
	}
	if hint == "" {
		t.Fatal("expected a login hint for an empty token")
	}
}

func TestTokenSession_InvalidOrExpired(t *testing.T) {
	// A well-formed JWT whose exp is in the past is treated as no session.
	expired := makeJWT(t, map[string]any{"exp": float64(time.Now().Add(-time.Minute).Unix())})
	for name, jwt := range map[string]string{
		"garbage": "not-a-jwt",
		"expired": expired,
	} {
		info, hint := tokenSession(jwt)
		if info != nil || hint == "" {
			t.Errorf("%s: got info=%v hint=%q, want nil info + a hint", name, info, hint)
		}
	}
}

func TestTokenSession_Valid(t *testing.T) {
	jwt := makeJWT(t, map[string]any{
		"sub": "0xabc",
		"exp": float64(time.Now().Add(time.Hour).Unix()),
	})
	info, hint := tokenSession(jwt)
	if hint != "" {
		t.Fatalf("unexpected hint: %q", hint)
	}
	if info["token"] != jwt {
		t.Errorf("token: got %v, want the JWT", info["token"])
	}
	if info["expires_at"] == "" || info["expires_in"] == "" {
		t.Errorf("expected expiry fields, got %v", info)
	}
}

func TestLoginSummary_HidesTokenByDefault(t *testing.T) {
	a := &rail0.AuthResponse{
		Token:     makeJWT(t, map[string]any{"exp": float64(time.Now().Add(time.Hour).Unix())}),
		Address:   "0xabc",
		AccountID: "acc_1",
		ExpiresAt: "2026-07-16T00:00:00Z",
	}

	// Default: no raw token, but the non-secret session facts are present.
	def := loginSummary(a, false)
	if _, ok := def["token"]; ok {
		t.Error("default summary must not include the raw token")
	}
	if def["address"] != "0xabc" || def["account_id"] != "acc_1" {
		t.Errorf("summary missing session fields: %v", def)
	}
	if def["expires_in"] == nil {
		t.Error("expected expires_in in the summary")
	}

	// --token: the raw token is included.
	withTok := loginSummary(a, true)
	if withTok["token"] != a.Token {
		t.Errorf("token: got %v, want the JWT", withTok["token"])
	}
}

func TestLoginSummary_OmitsAccountIDWhenAccountLess(t *testing.T) {
	a := &rail0.AuthResponse{
		Token:     makeJWT(t, map[string]any{"exp": float64(time.Now().Add(time.Hour).Unix())}),
		Address:   "0xbuyer",
		AccountID: "", // account-less (buyer) session
		ExpiresAt: "2026-07-16T00:00:00Z",
	}
	info := loginSummary(a, false)
	if _, ok := info["account_id"]; ok {
		t.Error("account_id must be omitted for an account-less session")
	}
	if info["address"] != "0xbuyer" {
		t.Errorf("address: got %v", info["address"])
	}
}
