package cmd

import "testing"

// A chain is labelled "Name (id)" when the catalog knows it, and degrades to
// "chain <id>" when it doesn't (unreachable gateway, or an id outside this
// environment's catalog).
func TestFormatChainLabel(t *testing.T) {
	cases := []struct {
		name string
		id   int
		want string
	}{
		{"Base Sepolia", 84532, "Base Sepolia (84532)"},
		{"", 84532, "chain 84532"},
	}
	for _, tc := range cases {
		if got := formatChainLabel(tc.name, tc.id); got != tc.want {
			t.Errorf("formatChainLabel(%q, %d) = %q, want %q", tc.name, tc.id, got, tc.want)
		}
	}
}

// `rail0 blockchains` is an alias of `rail0 chains`.
func TestChainsHasBlockchainsAlias(t *testing.T) {
	sub, _, err := rootCmd.Find([]string{"blockchains"})
	if err != nil || sub == nil || sub.Name() != "chains" {
		t.Fatalf("`blockchains` should resolve to the chains command, got %v (%v)", sub, err)
	}
}
