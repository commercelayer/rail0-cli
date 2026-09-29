package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
		authRevokeAllCmd:        {"auth", "revoke-all"},
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

// --payment-id reaches the gateway exactly as typed — it resolves both the UUID and
// the 0x rail0_id itself. The CLI used to resolve a 0x id with Payments.Get, which is
// participant-only (so another wallet's payment of the same account failed) and cost
// an extra request: exactly one request now, the listing, carrying the value as is.
func TestEventCallbacksPassesThePaymentIDThrough(t *testing.T) {
	const rail0ID = "0xa1b2c3d40000000000000000000000000000000000000000000000000000beef"
	for _, id := range []string{rail0ID, "7f1c0c8e-1d2b-4c3a-9e8f-0123456789ab"} {
		t.Run(id, func(t *testing.T) {
			isolate(t)
			isolateTokenStore(t)
			var paths []string
			var got string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				got = r.URL.Query().Get("payment_id")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`[]`))
			}))
			defer srv.Close()
			t.Setenv("RAIL0_BASE_URL", srv.URL)
			webhooksEventsPaymentID = id
			t.Cleanup(func() { webhooksEventsPaymentID = "" })

			_ = captureStdout(t, func() {
				if err := webhooksEventCallbacksCmd.RunE(webhooksEventCallbacksCmd, []string{"whk_1"}); err != nil {
					t.Fatalf("event-callbacks: %v", err)
				}
			})
			if len(paths) != 1 || paths[0] != "/webhooks/whk_1/event_callbacks" {
				t.Errorf("requests = %v, want only the listing", paths)
			}
			if got != id {
				t.Errorf("payment_id = %q, want %q unchanged", got, id)
			}
		})
	}
}

// A value that is neither form is refused locally, before any request, naming both.
func TestValidateEventCallbackPaymentID(t *testing.T) {
	for _, ok := range []string{
		"",
		"7f1c0c8e-1d2b-4c3a-9e8f-0123456789ab",
		"7F1C0C8E-1D2B-4C3A-9E8F-0123456789AB",
		"0x" + strings.Repeat("aB", 32),
	} {
		if err := validateEventCallbackPaymentID(ok); err != nil {
			t.Errorf("%q: %v, want accepted", ok, err)
		}
	}
	for _, bad := range []string{
		"0xabc",                            // too short
		"0X" + strings.Repeat("ab", 32),    // the gateway requires a lowercase 0x
		"0x" + strings.Repeat("ab", 33),    // too long
		strings.Repeat("ab", 32),           // no 0x
		"7f1c0c8e-uuid",                    // not a UUID
		"7f1c0c8e1d2b4c3a9e8f0123456789ab", // UUID without dashes
	} {
		err := validateEventCallbackPaymentID(bad)
		if err == nil || !strings.Contains(err.Error(), "UUID") || !strings.Contains(err.Error(), "rail0_id") {
			t.Errorf("%q: got %v, want a refusal naming both forms", bad, err)
		}
	}
}

// event-callbacks used to stop, silently, at the gateway's default page, and could
// not ask for one response code: both now reach the query.
func TestEventCallbacksSendsPagingAndResponseCode(t *testing.T) {
	isolate(t)
	isolateTokenStore(t)
	var q map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = map[string]string{}
		for k := range r.URL.Query() {
			q[k] = r.URL.Query().Get(k)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	t.Setenv("RAIL0_BASE_URL", srv.URL)
	webhooksEventsResponseCode, webhooksEventsPage, webhooksEventsPerPage = "500", 2, 50
	t.Cleanup(func() { webhooksEventsResponseCode, webhooksEventsPage, webhooksEventsPerPage = "", 0, 0 })

	_ = captureStdout(t, func() {
		if err := webhooksEventCallbacksCmd.RunE(webhooksEventCallbacksCmd, []string{"wh-1"}); err != nil {
			t.Errorf("event-callbacks: %v", err)
		}
	})
	for k, want := range map[string]string{"response_code": "500", "page": "2", "per_page": "50"} {
		if q[k] != want {
			t.Errorf("%s = %q, want %q (query %v)", k, q[k], want, q)
		}
	}
}

// redeliver is ONE POST to the callback's redeliver route, and prints the gateway's
// queued acknowledgement (the 202 means accepted, not delivered).
func TestWebhooksRedeliverPostsToTheCallbackRoute(t *testing.T) {
	isolate(t)
	isolateTokenStore(t)
	const cb = "7f1c0c8e-1d2b-4c3a-9e8f-0123456789ab"
	var method, path string
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		method, path = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"queued"}`))
	}))
	defer srv.Close()
	t.Setenv("RAIL0_BASE_URL", srv.URL)

	out := captureStdout(t, func() {
		if err := webhooksRedeliverCmd.RunE(webhooksRedeliverCmd, []string{"whk_1", cb}); err != nil {
			t.Fatalf("redeliver: %v", err)
		}
	})
	if calls != 1 || method != http.MethodPost || path != "/webhooks/whk_1/event_callbacks/"+cb+"/redeliver" {
		t.Errorf("request = %d × %s %s, want one POST to the redeliver route", calls, method, path)
	}
	if !strings.Contains(out, "queued") {
		t.Errorf("output %q does not show the queued acknowledgement", out)
	}
}

// Both ids are required: a missing callback id is a usage error, not a request.
func TestWebhooksRedeliverNeedsBothIDs(t *testing.T) {
	if err := webhooksRedeliverCmd.Args(webhooksRedeliverCmd, []string{"whk_1"}); err == nil {
		t.Error("redeliver with only the webhook id: expected an argument error")
	}
	if err := webhooksRedeliverCmd.Args(webhooksRedeliverCmd, []string{"whk_1", "cb"}); err != nil {
		t.Errorf("redeliver with both ids: %v", err)
	}
}
