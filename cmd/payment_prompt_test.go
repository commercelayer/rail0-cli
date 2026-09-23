package cmd

import (
	"slices"
	"sort"
	"testing"
)

// opFilters exists to mirror the gateway's state guards, so the two must not drift.
// Pinned as a test rather than trusted to a comment because the drift is silent: a
// filter narrower than the gateway hides an actionable payment from the picker, and
// one wider offers a payment the gateway will reject with a 422.
//
// `partially_refunded` is the case that prompted this (#52): no new payment reaches
// it, which reads as dead — but the gateway keeps it in CAPTURABLE_FROM /
// REFUNDABLE_FROM for historical rows, so the picker must keep offering it too.
func TestOpFiltersMirrorTheGatewayStateGuards(t *testing.T) {
	// Mirrors rail0-gateway app/models/payment.rb CAPTURABLE_FROM / REFUNDABLE_FROM,
	// and the AASM `void` event's sources.
	gatewayCapturableFrom := []string{"authorized", "expired", "partially_captured", "partially_refunded"}
	gatewayVoidableFrom := []string{"authorized", "expired"}
	gatewayRefundableFrom := []string{"charged", "captured", "partially_captured", "partially_refunded"}

	for _, tc := range []struct {
		op       string
		expected []string
	}{
		{"capture", gatewayCapturableFrom},
		// release acts on the capturable escrow: expired is the status it exists for.
		{"release", gatewayCapturableFrom},
		{"void", gatewayVoidableFrom},
		{"refund", gatewayRefundableFrom},
		// dispute acts on the refundable set too, additionally requiring no open dispute.
		{"dispute", gatewayRefundableFrom},
	} {
		got := append([]string(nil), opFilters[tc.op].statuses...)
		want := append([]string(nil), tc.expected...)
		sort.Strings(got)
		sort.Strings(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s: filter %v does not mirror the gateway's %v", tc.op, got, want)
		}
	}
}
