package cmd

import "testing"

func TestAddressFromKey(t *testing.T) {
	// Well-known test vector (Hardhat account #0).
	const key = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	const want = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"

	got, err := addressFromKey(key)
	if err != nil {
		t.Fatalf("addressFromKey: %v", err)
	}
	if got != want {
		t.Errorf("address: got %s, want %s", got, want)
	}

	// Without the 0x prefix it must still work.
	if got, err := addressFromKey(key[2:]); err != nil || got != want {
		t.Errorf("no-prefix: got %s err %v", got, err)
	}

	// Garbage is rejected.
	if _, err := addressFromKey("not-a-key"); err == nil {
		t.Error("invalid key: expected error, got nil")
	}
}
