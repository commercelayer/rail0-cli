package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/spf13/cobra"
)

func TestParseBoolFlag(t *testing.T) {
	cases := []struct {
		in      string
		want    *bool
		wantErr bool
	}{
		{"", nil, false},
		{"true", boolPtr(true), false},
		{"false", boolPtr(false), false},
		{"yes", nil, true},
	}
	for _, tc := range cases {
		got, err := parseBoolFlag(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseBoolFlag(%q): expected error, got nil", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseBoolFlag(%q): unexpected error %v", tc.in, err)
			continue
		}
		if (got == nil) != (tc.want == nil) {
			t.Errorf("parseBoolFlag(%q): nil mismatch got %v want %v", tc.in, got, tc.want)
			continue
		}
		if got != nil && *got != *tc.want {
			t.Errorf("parseBoolFlag(%q): got %v want %v", tc.in, *got, *tc.want)
		}
	}
}

func boolPtr(b bool) *bool { return &b }

// `webhooks delete` and `webhooks rotate-secret` had no confirmation gate while
// every sibling destructive command did (#67). These tests cover the gate itself,
// which nothing did before — hence the sweep below rather than two isolated cases.

// The whole destructive surface, so a command added later without a gate fails here
// instead of shipping. `rotate-secret` counts: the previous secret is gone and every
// consumer still verifying with it starts rejecting deliveries.
//
// The fund-moving payment ops are ALL here — capture, charge and the dispute
// pair used to broadcast without any confirmation while void/release/refund
// were gated, and this sweep only listed the commands already marked, which is
// exactly how the gap survived (#104).
func TestEveryDestructiveCommandHasTheYesFlag(t *testing.T) {
	destructive := map[*cobra.Command][]string{
		paymentsChargeCmd:       {"payments", "charge"},
		paymentsCaptureCmd:      {"payments", "capture"},
		paymentsVoidCmd:         {"payments", "void"},
		paymentsReleaseCmd:      {"payments", "release"},
		paymentsRefundCmd:       {"payments", "refund"},
		paymentsDisputeCmd:      {"payments", "dispute"},
		paymentsDisputeCloseCmd: {"payments", "dispute", "close"},
		webhooksDeleteCmd:       {"webhooks", "delete"},
		webhooksRotateSecretCmd: {"webhooks", "rotate-secret"},
		walletsDeleteCmd:        {"wallets", "delete"},
		walletTokensDisableCmd:  {"wallets", "tokens", "disable"},
		keysRemoveCmd:           {"keys", "rm"},
	}
	for cmd, path := range destructive {
		if cmd.Flags().Lookup("yes") == nil {
			t.Errorf("%s: missing -y/--yes, so it cannot be confirmed or run non-interactively",
				strings.Join(path, " "))
		}
	}
}

// The gate must run BEFORE the API call: under test there is no TTY, so
// confirmDestructive refuses and the command must fail with that refusal rather
// than by attempting a request. Asserting the message distinguishes the two — a
// network error here would mean the gate was bypassed.
func TestDestructiveWebhookCommandsRefuseWithoutATTY(t *testing.T) {
	// assumeYes is a package-level var shared by every --yes flag, so a previous
	// test (or command) setting it would leak in.
	assumeYes = false
	t.Cleanup(func() { assumeYes = false })

	for _, tc := range []struct {
		name string
		cmd  *cobra.Command
	}{
		{"webhooks delete", webhooksDeleteCmd},
		{"webhooks rotate-secret", webhooksRotateSecretCmd},
	} {
		err := tc.cmd.RunE(tc.cmd, []string{"whk_test123"})
		if err == nil {
			t.Errorf("%s: expected a refusal without a TTY, got nil", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), "refusing to run without confirmation") {
			t.Errorf("%s: failed for the wrong reason (want the confirmation gate): %v", tc.name, err)
		}
	}
}

// A delivery row stores the payment's UUID, so the 0x rail0 id every other command
// takes must be resolved before it reaches the filter — sent as-is it failed the whole
// query. A UUID passes through without a lookup.
func TestEventCallbackPaymentIDResolvesARail0ID(t *testing.T) {
	var lookups int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lookups++
		if r.URL.Path != "/payments/0xabc" {
			t.Errorf("unexpected request %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "7f1c0c8e-uuid", "rail0_id": "0xabc"})
	}))
	defer srv.Close()
	client := rail0.NewClient(rail0.ClientOptions{BaseURL: srv.URL})

	got, err := eventCallbackPaymentID(context.Background(), client, "0xabc")
	if err != nil || got != "7f1c0c8e-uuid" {
		t.Fatalf("0x id: got %q err %v, want the payment's UUID", got, err)
	}
	for _, passthrough := range []string{"", "7f1c0c8e-uuid"} {
		if got, err := eventCallbackPaymentID(context.Background(), client, passthrough); err != nil || got != passthrough {
			t.Errorf("%q: got %q err %v, want it untouched", passthrough, got, err)
		}
	}
	if lookups != 1 {
		t.Errorf("lookups = %d, want 1 (only the 0x id needs one)", lookups)
	}
}
