package cmd

import (
	"strings"
	"testing"
)

func TestResolveAccountID(t *testing.T) {
	// An explicit --account wins and needs no session.
	if id, err := resolveAccountID("019e748b-acc"); err != nil || id != "019e748b-acc" {
		t.Fatalf("explicit: got %q err %v", id, err)
	}

	// No flag and no session → error, and it points at --account.
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", dir)
	_, err := resolveAccountID("")
	if err == nil {
		t.Fatal("no flag + no session must error")
	}
	if !strings.Contains(err.Error(), "--account") {
		t.Errorf("error should mention --account, got: %v", err)
	}
}
