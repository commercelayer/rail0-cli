package cmd

import (
	"context"

	rail0 "github.com/rail0/go-sdk"
)

// resolvePaymentID returns the identifier the gateway should address the payment
// by. The gateway resolves EITHER form in the path — the payment's UUID id or its
// logical rail0_id (0x + 64 hex) — so the value is passed through unchanged and
// works with or without a session. (Historically the CLI pre-resolved a rail0_id
// to its UUID via an authenticated list filter; that is no longer needed.)
//
// The ctx/client parameters are retained so callers need no change and a future
// client-side step can be reintroduced without touching every command.
func resolvePaymentID(_ context.Context, _ *rail0.Client, idOrRail0 string) (string, error) {
	return idOrRail0, nil
}
