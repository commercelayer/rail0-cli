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
