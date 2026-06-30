package cmd

import (
	"context"
	"fmt"
	"regexp"

	"github.com/rail0/cli/internal/token"
	rail0 "github.com/rail0/go-sdk"
)

// rail0IDPattern matches a logical rail0_id: a 0x-prefixed 32-byte hex hash.
// Anything else (e.g. a UUID) is treated as the technical primary key.
var rail0IDPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`)

// resolvePaymentID turns the identifier a user passes into the payment's
// technical UUID, which is what the gateway addresses path operations by.
//
//   - A UUID (or any non-rail0_id value) is returned unchanged and works with
//     or without a session — the gateway accepts it directly.
//   - A rail0_id (0x + 64 hex) is the logical id a user knows; it is resolved to
//     the UUID through the authenticated GET /payments?rail0_id= filter, which
//     is scoped to the caller's payer/payee. That only works when logged in, so
//     without a session we ask for the UUID (or a login) instead of letting the
//     call fail with an opaque 401.
func resolvePaymentID(ctx context.Context, client *rail0.Client, idOrRail0 string) (string, error) {
	if !rail0IDPattern.MatchString(idOrRail0) {
		return idOrRail0, nil
	}
	if token.Load() == "" {
		return "", fmt.Errorf("a rail0_id needs a logged-in session to resolve — run 'rail0 auth login', or pass the payment's UUID id instead")
	}
	res, err := client.Payments.List(ctx, rail0.ListPaymentsParams{Rail0ID: idOrRail0, PerPage: 1})
	if err != nil {
		return "", err
	}
	if len(res.Data) == 0 {
		return "", fmt.Errorf("no payment found for rail0_id %s (log in as its payer or payee)", idOrRail0)
	}
	return res.Data[0].ID, nil
}
