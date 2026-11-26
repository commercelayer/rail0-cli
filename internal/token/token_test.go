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
func TestBaseURLIsNormalized(t *testing.T) {
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

// The modes are ENFORCED on every save: a tokens directory or file that already
// existed world-readable (an older build, a restored backup) used to stay that way,
// because MkdirAll and WriteFile apply their mode only when they create the path.
// The replace is atomic, so it leaves no temp file behind.
func TestSaveTightensExistingPermissions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", dir)

	path := Path(gwA)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Save(gwA, "new-token"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if info, _ := os.Stat(filepath.Dir(path)); info.Mode().Perm() != 0o700 {
		t.Errorf("tokens dir mode = %o, want 700", info.Mode().Perm())
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Errorf("token file mode = %o, want 600", info.Mode().Perm())
	}
	if got := Load(gwA); got != "new-token" {
		t.Errorf("Load = %q, want the new token", got)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("tokens dir holds %d entries, want only the token (no temp file left)", len(entries))
	}
}

// The write-mode token lives beside the session's, per gateway, in a file of its own:
// ending write mode removes it and leaves the session, and one gateway's does not show
// up on another.
func TestWriteModeTokenIsStoredBesideTheSession(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "Library", "Caches"), 0o755); err != nil {
		t.Fatal(err)
	}

	if got := LoadWriteMode(gwA); got != "" {
		t.Errorf("nothing stored: LoadWriteMode = %q", got)
	}
	if err := DeleteWriteMode(gwA); err != nil {
		t.Errorf("deleting what is not there must not fail: %v", err)
	}
	if err := Save(gwA, "session-a"); err != nil {
		t.Fatal(err)
	}
	if err := SaveWriteMode(gwA, "write-a"); err != nil {
		t.Fatal(err)
	}
	if LoadWriteMode(gwA) != "write-a" || Load(gwA) != "session-a" {
		t.Errorf("write = %q, session = %q", LoadWriteMode(gwA), Load(gwA))
	}
	if got := LoadWriteMode(gwB); got != "" {
		t.Errorf("another gateway sees %q", got)
	}

	// Same directory as the session, a name of its own, readable by the owner only.
	path := Path(gwA) + ".write"
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("the write-mode token is not at %s: %v", path, err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("permissions = %o, want 600", perm)
	}

	if err := DeleteWriteMode(gwA); err != nil {
		t.Fatal(err)
	}
	if LoadWriteMode(gwA) != "" || Load(gwA) != "session-a" {
		t.Errorf("after delete: write = %q, session = %q", LoadWriteMode(gwA), Load(gwA))
	}

	// Signing out removes the session file only; the caller ends write mode first.
	if err := SaveWriteMode(gwA, "write-a"); err != nil {
		t.Fatal(err)
	}
	if err := Delete(gwA); err != nil {
		t.Fatal(err)
	}
	if Load(gwA) != "" || LoadWriteMode(gwA) != "write-a" {
		t.Errorf("Delete touched the wrong file: session = %q, write = %q", Load(gwA), LoadWriteMode(gwA))
	}
}

// The operator marker describes ONE token: the one stored when it was written. A marker
// left by an earlier session must never answer for the session stored now.
func TestOperatorMarkerBelongsToItsToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "Library", "Caches"), 0o755); err != nil {
		t.Fatal(err)
	}

	if IsOperator(gwA) {
		t.Fatal("no session, no marker: must read as not an operator's")
	}
	if err := Save(gwA, "operator-jwt"); err != nil {
		t.Fatal(err)
	}
	if IsOperator(gwA) {
		t.Fatal("a session with no marker must read as not an operator's")
	}
	if err := MarkOperator(gwA, "operator-jwt"); err != nil {
		t.Fatal(err)
	}
	if !IsOperator(gwA) {
		t.Fatal("the marked session must read as an operator's")
	}
	if IsOperator(gwB) {
		t.Error("the marker leaked to another gateway")
	}

	// The marker file is as private as the token's, and holds no token.
	path := Path(gwA) + operatorSuffix
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("marker file: %v, mode %v, want 0600", err, info.Mode().Perm())
	}
	if b, _ := os.ReadFile(path); len(b) == 0 || string(b) == "operator-jwt\n" {
		t.Errorf("the marker must hold a fingerprint, not the token: %q", b)
	}

	// Another session stored over it: the old marker says nothing about the new token.
	if err := Save(gwA, "merchant-jwt"); err != nil {
		t.Fatal(err)
	}
	if IsOperator(gwA) {
		t.Error("a marker of the previous session answered for the new one")
	}

	// Removing the session removes what was known about it.
	if err := MarkOperator(gwA, "merchant-jwt"); err != nil {
		t.Fatal(err)
	}
	if err := Delete(gwA); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the marker outlived its session: %v", err)
	}
	if err := UnmarkOperator(gwA); err != nil {
		t.Errorf("removing a marker that is not there: %v", err)
	}
}
