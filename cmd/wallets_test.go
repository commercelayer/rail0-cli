package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestWalletsCommandTree(t *testing.T) {
	// `wallets` is top-level; bare it lists (account from the session), verbs
	// hang off it and take no account_id.
	if walletsCmd.RunE == nil {
		t.Error("wallets should list when run bare (nil RunE)")
	}
	if err := walletsCmd.Args(walletsCmd, []string{}); err != nil {
		t.Errorf("wallets list should take no args, got %v", err)
	}

	for _, verb := range []string{"get", "create", "update", "delete", "balances"} {
		sub, _, err := walletsCmd.Find([]string{verb})
		if err != nil || sub == nil || sub.Name() != verb {
			t.Errorf("wallets %s: not found (%v)", verb, err)
		}
	}

	// get/update/delete/balances take exactly one positional (the wallet),
	// no account id.
	for _, verb := range []string{"get", "update", "delete", "balances"} {
		sub, _, _ := walletsCmd.Find([]string{verb})
		if err := sub.Args(sub, []string{"0xabc"}); err != nil {
			t.Errorf("wallets %s should accept 1 arg, got %v", verb, err)
		}
		if err := sub.Args(sub, []string{"a", "b"}); err == nil {
			t.Errorf("wallets %s should reject 2 args", verb)
		}
	}

	// `accounts` is gone entirely.
	if sub, _, _ := rootCmd.Find([]string{"accounts"}); sub != nil && sub.Name() == "accounts" {
		t.Error("top-level 'accounts' should no longer exist; use 'wallets'")
	}
}

func TestWalletsSymbolHasTokenAlias(t *testing.T) {
	// --token is a hidden alias of --symbol on the list and balances commands.
	if walletsCmd.Flags().Lookup("token") == nil {
		t.Error("wallets: --token should alias --symbol")
	}
	balances, _, _ := walletsCmd.Find([]string{"balances"})
	if balances == nil || balances.Flags().Lookup("token") == nil {
		t.Error("wallets balances: --token should alias --symbol")
	}
}

func TestWalletsCreateHasSigningKeyFlags(t *testing.T) {
	// Adding a wallet now needs the key OF the address being added to build the
	// SIWE proof-of-ownership, resolved like every signing command: --private-key
	// (-p), a hidden --pk alias, and --address (required, accepts an @alias).
	create, _, err := walletsCmd.Find([]string{"create"})
	if err != nil || create == nil {
		t.Fatalf("wallets create: not found (%v)", err)
	}
	pk := create.Flags().Lookup("private-key")
	if pk == nil {
		t.Fatal("wallets create: --private-key flag missing")
	}
	if pk.Shorthand != "p" {
		t.Errorf("wallets create: --private-key shorthand = %q, want p", pk.Shorthand)
	}
	if create.Flags().Lookup("pk") == nil {
		t.Error("wallets create: --pk should alias --private-key")
	}
	addr := create.Flags().Lookup("address")
	if addr == nil {
		t.Fatal("wallets create: --address flag missing")
	}
	if ann := addr.Annotations[cobra.BashCompOneRequiredFlag]; len(ann) != 1 || ann[0] != "true" {
		t.Error("wallets create: --address should be required")
	}
}
