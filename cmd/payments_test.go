package cmd

import "testing"

func TestResolveSigningKey(t *testing.T) {
	t.Setenv("RAIL0_PRIVATE_KEY", "")

	// Explicit key wins.
	if got, err := resolveSigningKey("0xexplicit"); err != nil || got != "0xexplicit" {
		t.Errorf("explicit: got %q err %v", got, err)
	}
	// No key anywhere → error.
	if _, err := resolveSigningKey(""); err == nil {
		t.Error("no key: expected error, got nil")
	}
	// RAIL0_PRIVATE_KEY fallback (CI).
	t.Setenv("RAIL0_PRIVATE_KEY", "0xenvkey")
	if got, err := resolveSigningKey(""); err != nil || got != "0xenvkey" {
		t.Errorf("env fallback: got %q err %v", got, err)
	}
}

// The atomic operations no longer expose --prepare: it should be absent from
// every signing command so the help can't promise a flow the CLI can't complete.
func TestSigningCommandsHaveNoPrepareFlag(t *testing.T) {
	cmds := []string{"authorize", "charge", "capture", "void", "release", "refund", "dispute"}
	for _, name := range cmds {
		sub, _, err := paymentsCmd.Find([]string{name})
		if err != nil || sub == nil {
			t.Fatalf("payments %s: command not found (%v)", name, err)
		}
		if sub.Flags().Lookup("prepare") != nil {
			t.Errorf("payments %s: --prepare should have been removed", name)
		}
		if sub.Flags().Lookup("private-key") == nil {
			t.Errorf("payments %s: --private-key flag missing", name)
		}
	}
}
