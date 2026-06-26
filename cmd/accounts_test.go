package cmd

import "testing"

func TestBalancesCommandRegistered(t *testing.T) {
	sub, _, err := accountsCmd.Find([]string{"balances"})
	if err != nil || sub == nil || sub.Name() != "balances" {
		t.Fatalf("accounts balances: command not found (%v)", err)
	}
	for _, flag := range []string{"chain-id", "symbol"} {
		if sub.Flags().Lookup(flag) == nil {
			t.Errorf("accounts balances: --%s flag missing", flag)
		}
	}
	if err := sub.Args(sub, []string{"acct_1"}); err == nil {
		t.Error("accounts balances: expected 2 args to be required")
	}
	if err := sub.Args(sub, []string{"acct_1", "wlt_1"}); err != nil {
		t.Errorf("accounts balances: 2 args should be accepted, got %v", err)
	}
}
