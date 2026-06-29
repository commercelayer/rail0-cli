package cmd

import "testing"

// `rail0 blockchains` is an alias of `rail0 chains`.
func TestChainsHasBlockchainsAlias(t *testing.T) {
	sub, _, err := rootCmd.Find([]string{"blockchains"})
	if err != nil || sub == nil || sub.Name() != "chains" {
		t.Fatalf("`blockchains` should resolve to the chains command, got %v (%v)", sub, err)
	}
}
