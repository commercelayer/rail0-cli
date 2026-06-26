package cmd

import "testing"

func TestRequirePrivateKey(t *testing.T) {
	if err := requirePrivateKey(""); err == nil {
		t.Error("requirePrivateKey(\"\"): expected error, got nil")
	}
	if err := requirePrivateKey("0xabc"); err != nil {
		t.Errorf("requirePrivateKey(key): unexpected error %v", err)
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
