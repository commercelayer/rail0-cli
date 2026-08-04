package token

import (
	"os"
	"path/filepath"
	"testing"
)

const gwA = "https://api.staging.rail0.xyz"
const gwB = "https://api.rail0.xyz"

// The cache file is the ONLY source of the session. A RAIL0_TOKEN env var used to take
// priority over it, which made `auth login` look broken: it wrote a fresh token,
// reported success, and every later command kept using the stale env value — an
// endless re-login loop with no explanation (#60).
//
// Asserted as "the env var is ignored" rather than by deleting the old test, so a
// future reintroduction of the override fails here.
func TestLoadIgnoresTheEnvironment(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("RAIL0_TOKEN", "stale-token-from-the-shell")

	// Nothing on disk → nothing loaded, despite the env var being set.
	if got := Load(gwA); got != "" {
		t.Errorf("Load() = %q, want empty: the environment must not be a token source", got)
	}

	// With a file present, the file wins — there is nothing to lose to.
	if err := Save(gwA, "fresh-token-from-login"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := Load(gwA); got != "fresh-token-from-login" {
		t.Errorf("Load() = %q, want the token just saved", got)
	}
}

// A JWT is minted by one gateway and meaningless to another. `rail0 env` switches
// base URL without touching the session, so a single global file let a staging
// token be presented to production, where `ensureSession` saw a live token with a
// matching `sub`, skipped the sign-in, and collected a 401. (#94)
func TestTokensAreScopedPerGateway(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", dir)

	if err := Save(gwA, "staging-jwt"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if got := Load(gwB); got != "" {
		t.Errorf("Load(%q) = %q, want empty: another gateway's token must not be offered", gwB, got)
	}
	if got := Load(gwA); got != "staging-jwt" {
		t.Errorf("Load(%q) = %q, want the staging token", gwA, got)
	}

	// Both sessions coexist, so switching back and forth does not force a re-login.
	if err := Save(gwB, "production-jwt"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := Load(gwA); got != "staging-jwt" {
		t.Errorf("Load(%q) = %q, want the staging token to have survived", gwA, got)
	}
	if got := Load(gwB); got != "production-jwt" {
		t.Errorf("Load(%q) = %q, want the production token", gwB, got)
	}

	// Deleting one leaves the other alone.
	if err := Delete(gwB); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got := Load(gwB); got != "" {
		t.Errorf("Load(%q) = %q after Delete, want empty", gwB, got)
	}
	if got := Load(gwA); got != "staging-jwt" {
		t.Errorf("Load(%q) = %q, want the staging token untouched by the other's delete", gwA, got)
	}
}

// A trailing slash or different casing in the base URL is the same gateway — the
// two forms must not become two sessions.
func TestBaseURLIsNormalised(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", dir)

	if err := Save("https://API.rail0.xyz/", "jwt"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := Load("https://api.rail0.xyz"); got != "jwt" {
		t.Errorf("Load() = %q, want the token saved under a differently-spelled same URL", got)
	}
}

// A pre-#94 token file records no issuer, so it must never be presented to a
// gateway — it is removed instead, which costs one re-login and says so.
func TestLegacyTokenFileIsDiscardedNotUsed(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", dir)

	base, err := tokenDir()
	if err != nil {
		t.Fatalf("tokenDir: %v", err)
	}
	legacy := filepath.Join(base, legacyFileName)
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(legacy, []byte("token-of-unknown-origin\n"), 0o600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	if got := Load(gwA); got != "" {
		t.Errorf("Load() = %q, want empty: a token of unknown origin must not be used", got)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("the legacy file should have been removed, stat err = %v", err)
	}
}

// `auth status` reports Path so an operator can find or remove one specific
// session; the value is only useful if it is the file Save actually writes, and
// if it answers for a gateway with no session at all (the "where would it be?"
// case, which is exactly when someone goes looking).
func TestPathIsWhereTheSessionIsWritten(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", dir)

	if got := Path(gwA); got == "" {
		t.Fatal("Path() = empty for a resolvable cache dir")
	}
	if got, want := Path(gwB), Path(gwA); got == want {
		t.Errorf("Path() returned the same file for two gateways: %q", got)
	}

	// Before any Save: a path, not an error, and nothing on disk yet.
	before := Path(gwA)
	if _, err := os.Stat(before); !os.IsNotExist(err) {
		t.Errorf("Path() named an existing file before Save, stat err = %v", err)
	}

	written, err := SaveAndReport(gwA, "jwt")
	if err != nil {
		t.Fatalf("SaveAndReport: %v", err)
	}
	if written != before {
		t.Errorf("Save wrote %q but Path reports %q", written, before)
	}
}
