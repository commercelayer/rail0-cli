package cmd

import (
	"testing"

	"github.com/rail0/cli/internal/config"
)

func TestResolveAlias(t *testing.T) {
	// Point the config dir at a temp location and seed one alias.
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	cfg := &config.Config{Aliases: map[string]string{"payee": "0xAbC123"}}
	if err := cfg.Save(); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	// Literal address passes through unchanged.
	if got, err := resolveAlias("0xdeadbeef"); err != nil || got != "0xdeadbeef" {
		t.Errorf("literal: got %q err %v", got, err)
	}
	// Empty passes through (optional flags).
	if got, err := resolveAlias(""); err != nil || got != "" {
		t.Errorf("empty: got %q err %v", got, err)
	}
	// Known alias resolves.
	if got, err := resolveAlias("@payee"); err != nil || got != "0xAbC123" {
		t.Errorf("alias: got %q err %v", got, err)
	}
	// Unknown alias errors.
	if _, err := resolveAlias("@nope"); err == nil {
		t.Error("unknown alias: expected error, got nil")
	}
}

func TestAddAliasIsUniqueByName(t *testing.T) {
	// Point the config dir at a temp location.
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	const (
		addrA = "0xe3dd5ea618f74de4c95b0ff0ebf9921a06a694c4"
		addrB = "0x2d2b3d4e72370a7256633d426518a372a53557d5"
	)

	// First add succeeds and persists.
	if err := addAlias("payee", addrA); err != nil {
		t.Fatalf("first add: unexpected error %v", err)
	}
	if cfg, _ := config.Load(); cfg.Aliases["payee"] != addrA {
		t.Fatalf("alias not saved: %v", cfg.Aliases)
	}

	// Re-using the name is rejected (unique-by-name) and leaves the value intact.
	if err := addAlias("payee", addrB); err == nil {
		t.Error("duplicate name: expected error, got nil")
	}
	if cfg, _ := config.Load(); cfg.Aliases["payee"] != addrA {
		t.Errorf("duplicate add must not overwrite: %v", cfg.Aliases)
	}

	// The same address under a different name is allowed (uniqueness is name-only).
	if err := addAlias("merchant", addrA); err != nil {
		t.Errorf("same address, new name: unexpected error %v", err)
	}

	// A leading '@' in the name is rejected (sigil is reserved for referencing).
	if err := addAlias("@oops", addrA); err == nil {
		t.Error("'@'-prefixed name: expected error, got nil")
	}
}

func TestAddAliasValidatesAddress(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	// Malformed addresses are rejected and nothing is persisted.
	for _, bad := range []string{"0xAbC123", "not-an-address", "e3dd5ea618f74de4c95b0ff0ebf9921a06a694c4", "0xZZdd5ea618f74de4c95b0ff0ebf9921a06a694c4"} {
		if err := addAlias("buyer", bad); err == nil {
			t.Errorf("address %q: expected validation error, got nil", bad)
		}
	}
	if cfg, _ := config.Load(); len(cfg.Aliases) != 0 {
		t.Errorf("invalid adds must not persist: %v", cfg.Aliases)
	}

	// A well-formed 0x-prefixed 40-hex address is accepted.
	if err := addAlias("buyer", "0xe3dd5ea618f74de4c95b0ff0ebf9921a06a694c4"); err != nil {
		t.Errorf("valid address: unexpected error %v", err)
	}
}

func TestWalletsAliasCommand(t *testing.T) {
	// `wallets alias <address> <name>` is wired and takes exactly two args.
	sub, _, err := walletsCmd.Find([]string{"alias"})
	if err != nil || sub == nil || sub.Name() != "alias" {
		t.Fatalf("wallets alias: not found (%v)", err)
	}
	if got := sub.Use; got != "alias <address> <name>" {
		t.Errorf("wallets alias arg order: got %q", got)
	}
}

func TestAliasesCommandTree(t *testing.T) {
	// Bare `aliases` lists (has its own RunE) and takes no args.
	if aliasesCmd.RunE == nil {
		t.Error("aliases should list when run bare (nil RunE)")
	}
	for _, verb := range []string{"add", "rm"} {
		if sub, _, err := aliasesCmd.Find([]string{verb}); err != nil || sub == nil || sub.Name() != verb {
			t.Errorf("aliases %s: not found (%v)", verb, err)
		}
	}
	// No 'ls' verb — the bare command is the list.
	if sub, _, _ := aliasesCmd.Find([]string{"ls"}); sub != nil && sub.Name() == "ls" {
		t.Error("aliases ls should not exist; bare 'aliases' lists")
	}
}
