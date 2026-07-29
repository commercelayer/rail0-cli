package token

import "testing"

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
	if got := Load(); got != "" {
		t.Errorf("Load() = %q, want empty: the environment must not be a token source", got)
	}

	// With a file present, the file wins — there is nothing to lose to.
	if err := Save("fresh-token-from-login"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := Load(); got != "fresh-token-from-login" {
		t.Errorf("Load() = %q, want the token just saved", got)
	}
}
