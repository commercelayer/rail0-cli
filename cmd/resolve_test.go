package cmd

import (
	"context"
	"strings"
	"testing"
)

func TestResolvePaymentID(t *testing.T) {
	ctx := context.Background()

	// Force "not logged in": no RAIL0_TOKEN and an empty cache dir so no token
	// file is found.
	dir := t.TempDir()
	t.Setenv("RAIL0_TOKEN", "")
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", dir)

	// A UUID passes through unchanged — works with or without a session, and
	// needs no network (client is never touched).
	uuid := "019f17da-ce2c-7247-bad4-7bc515defd7f"
	if got, err := resolvePaymentID(ctx, nil, uuid); err != nil || got != uuid {
		t.Errorf("uuid passthrough: got %q err %v", got, err)
	}

	// A rail0_id without a session is rejected up front with a clear message —
	// no network call is attempted.
	rail0id := "0x" + strings.Repeat("a", 64)
	if _, err := resolvePaymentID(ctx, nil, rail0id); err == nil || !strings.Contains(err.Error(), "logged-in") {
		t.Errorf("rail0_id without login: expected a login error, got %v", err)
	}
}
