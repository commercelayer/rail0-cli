package cmd

import (
	"context"
	"fmt"
	"regexp"

	rail0 "github.com/rail0/go-sdk"
)

// rail0IDPattern matches a logical rail0_id: a 0x-prefixed 32-byte hex hash.
// Anything else (e.g. a UUID) is treated as the technical primary key.
var rail0IDPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`)

// resolvePaymentID turns the identifier a user passes — the logical rail0_id
// they know — into the payment's technical UUID, which is what the gateway (and
// the SDK, mirroring it) address path operations by. A rail0_id is resolved
// through the authenticated GET /payments?rail0_id= filter; a value that is not
// a rail0_id (already a UUID) is returned unchanged. Resolving a rail0_id
// therefore requires a logged-in session whose wallet is the payment's
// payer/payee.
func resolvePaymentID(ctx context.Context, client *rail0.Client, idOrRail0 string) (string, error) {
	if !rail0IDPattern.MatchString(idOrRail0) {
		return idOrRail0, nil
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
