package cmd

import (
	"context"
	"strings"
	"testing"
)

func TestResolvePaymentID(t *testing.T) {
	ctx := context.Background()

	// Both identifier forms pass through unchanged — the gateway resolves either a
	// UUID or a rail0_id in the path, with no network call from the CLI (client is
	// never touched, so a nil client is fine).
	uuid := "019f17da-ce2c-7247-bad4-7bc515defd7f"
	if got, err := resolvePaymentID(ctx, nil, uuid); err != nil || got != uuid {
		t.Errorf("uuid passthrough: got %q err %v", got, err)
	}

	rail0id := "0x" + strings.Repeat("a", 64)
	if got, err := resolvePaymentID(ctx, nil, rail0id); err != nil || got != rail0id {
		t.Errorf("rail0_id passthrough: got %q err %v", got, err)
	}
}
