package cmd

import (
	"strings"
	"testing"

	"github.com/rail0/cli/internal/config"
	"github.com/zalando/go-keyring"
)

// A throwaway test key (well-known Hardhat account #0) and its address.
const (
	testKeyHex  = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	testKeyAddr = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
)

func TestKeysCommandTree(t *testing.T) {
	for _, verb := range []string{"add", "rm", "address"} {
		if sub, _, err := keysCmd.Find([]string{verb}); err != nil || sub == nil || sub.Name() != verb {
			t.Errorf("keys %s: not found (%v)", verb, err)
		}
	}
}

// `keys` must not list. The listing it used to print came from the name index
// in the config file, which is a copy the keychain never confirms — so it
// cheerfully reported keys that had been removed elsewhere, and hid keys the
// keychain held after the index was lost. Replacing it with nothing is the
// point: `keys address <name>` reads the keychain and cannot be stale. A nil
// RunE makes cobra print the help instead, so the bare command still answers
// "what can I do here?" — it just no longer answers "what do I have?", which
// is the question it could not answer honestly. (#139)
func TestKeysDoesNotListWhenRunBare(t *testing.T) {
	if keysCmd.RunE != nil || keysCmd.Run != nil {
		t.Error("keys must have no bare action: a listing can only replay a stale index")
	}
}

func TestLoadKeyAndResolveSigningKey(t *testing.T) {
	keyring.MockInit() // in-memory keychain — never touches the real OS store
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("RAIL0_PRIVATE_KEY", "")

	// Seed a key directly in the (mock) keychain.
	if err := keyring.Set(keyringService, "payer", testKeyHex); err != nil {
		t.Fatalf("seed keychain: %v", err)
	}

	// loadKey returns the stored secret; "@payer" on --private-key resolves to it.
	if got, err := loadKey("payer"); err != nil || got != testKeyHex {
		t.Errorf("loadKey: got %q err %v", got, err)
	}
	if got, err := resolveSigningKey("@payer"); err != nil || got != testKeyHex {
		t.Errorf("resolveSigningKey(@payer): got %q err %v", got, err)
	}

	// A raw key passes through unchanged; an unknown "@name" errors.
	if got, err := resolveSigningKey(testKeyHex); err != nil || got != testKeyHex {
		t.Errorf("resolveSigningKey(raw): got %q err %v", got, err)
	}
	if _, err := resolveSigningKey("@nope"); err == nil {
		t.Error("resolveSigningKey(@nope): expected error, got nil")
	}
}

func TestKeysAddressDerivation(t *testing.T) {
	// addressFromKey underpins `keys add` validation and `keys address`.
	addr, err := addressFromKey(testKeyHex)
	if err != nil || addr != testKeyAddr {
		t.Errorf("addressFromKey: got %q err %v, want %s", addr, err, testKeyAddr)
	}
	if _, err := addressFromKey("not-a-key"); err == nil {
		t.Error("addressFromKey(invalid): expected error, got nil")
	}
}

func TestConfigKeysIndexPersists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	cfg := &config.Config{Keys: []string{"payer", "payee"}}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := config.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got.Keys) != 2 || got.Keys[0] != "payer" || got.Keys[1] != "payee" {
		t.Errorf("keys index not persisted: %v", got.Keys)
	}
}

// The keychain and the local index can disagree: delete config.yaml to "start
// clean", or copy a config between machines, and the index no longer lists an entry
// the keychain still holds. `keys add` used to write unconditionally, so adding
// under that name destroyed the orphaned secret with no warning (#70). A private key
// cannot be re-derived, so these two tests pin both halves: refuse the overwrite,
// AND leave the orphan removable — refusing without a way out would make the name
// permanently unusable.
func TestKeysAddRefusesToOverwriteAnOrphanedKeychainEntry(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	// The keychain holds "payer"; the index does NOT (a deleted config.yaml).
	if err := keyring.Set(keyringService, "payer", testKeyHex); err != nil {
		t.Fatalf("seed keychain: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if len(cfg.Keys) != 0 {
		t.Fatalf("precondition: index should be empty, got %v", cfg.Keys)
	}

	err = keysAddCmd.RunE(keysAddCmd, []string{"payer"})
	if err == nil {
		t.Fatal("keys add: expected a refusal for an orphaned keychain entry, got nil")
	}
	// Assert it is the GUARD refusing, not the secret prompt failing for want of a
	// TTY: without this the test passes against the unfixed code, since RunE errors
	// either way. The guard runs before promptSecret, so its message is deterministic.
	if !strings.Contains(err.Error(), "keychain already holds") {
		t.Fatalf("keys add failed for the wrong reason (want the overwrite guard): %v", err)
	}

	// The original secret must be untouched — the whole point.
	if got, gerr := keyring.Get(keyringService, "payer"); gerr != nil || got != testKeyHex {
		t.Errorf("the orphaned key was modified: got %q err %v", got, gerr)
	}
}

func TestKeysRemoveClearsAnOrphanedKeychainEntry(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	if err := keyring.Set(keyringService, "payer", testKeyHex); err != nil {
		t.Fatalf("seed keychain: %v", err)
	}

	// --yes bypasses the interactive confirmation (there is no TTY under test).
	if err := keysRemoveCmd.Flags().Set("yes", "true"); err != nil {
		t.Fatalf("set --yes: %v", err)
	}
	t.Cleanup(func() { _ = keysRemoveCmd.Flags().Set("yes", "false") })

	if err := keysRemoveCmd.RunE(keysRemoveCmd, []string{"payer"}); err != nil {
		t.Fatalf("keys rm on an orphaned entry: %v", err)
	}

	if _, err := keyring.Get(keyringService, "payer"); err == nil {
		t.Error("keys rm: the orphaned keychain entry is still present")
	}
}

// A name in neither place is still just unknown — the fallback must not turn every
// typo into a keychain probe that reports success.
func TestKeysRemoveRejectsAnUnknownName(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	if err := keysRemoveCmd.RunE(keysRemoveCmd, []string{"nope"}); err == nil {
		t.Error("keys rm nope: expected an error, got nil")
	}
}
