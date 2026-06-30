package cmd

import (
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
	if keysCmd.RunE == nil {
		t.Error("keys should list when run bare (nil RunE)")
	}
	for _, verb := range []string{"add", "rm", "address"} {
		if sub, _, err := keysCmd.Find([]string{verb}); err != nil || sub == nil || sub.Name() != verb {
			t.Errorf("keys %s: not found (%v)", verb, err)
		}
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
