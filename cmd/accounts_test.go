package cmd

import "testing"

func TestWalletsCommandTree(t *testing.T) {
	// Bare `accounts wallets <account_id>` lists; verbs hang off it.
	wallets, _, err := accountsCmd.Find([]string{"wallets"})
	if err != nil || wallets == nil || wallets.Name() != "wallets" {
		t.Fatalf("accounts wallets: not found (%v)", err)
	}
	if err := wallets.Args(wallets, []string{"acct_1"}); err != nil {
		t.Errorf("wallets list should accept 1 arg, got %v", err)
	}

	for _, verb := range []string{"get", "create", "update", "delete", "balances"} {
		sub, _, err := accountsCmd.Find([]string{"wallets", verb})
		if err != nil || sub == nil || sub.Name() != verb {
			t.Errorf("accounts wallets %s: not found (%v)", verb, err)
		}
	}

	// The old hyphenated verbs must be gone.
	for _, gone := range []string{"create-wallet", "update-wallet", "delete-wallet", "wallet"} {
		if sub, _, _ := accountsCmd.Find([]string{gone}); sub != nil && sub.Name() == gone {
			t.Errorf("accounts %s should no longer exist", gone)
		}
	}
}
