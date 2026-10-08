package cmd

import (
	"net/http"
	"strings"
	"testing"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/config"
	"github.com/spf13/cobra"
)

// The two account-level surfaces the SDK exposed and the CLI did not (#110). Asserted
// as command tree + flags rather than by driving HTTP: what was missing was the
// commands existing at all.
func TestDisputesAndAnalyticsAreRegistered(t *testing.T) {
	for _, name := range []string{"disputes", "analytics"} {
		if cmd, _, err := rootCmd.Find([]string{name}); err != nil || cmd == nil || cmd.Name() != name {
			t.Errorf("rail0 %s is not registered (%v)", name, err)
		}
	}
	for _, verb := range []string{"summary", "timeseries", "breakdown"} {
		if cmd, _, err := rootCmd.Find([]string{"analytics", verb}); err != nil || cmd == nil || cmd.Name() != verb {
			t.Errorf("rail0 analytics %s is not registered (%v)", verb, err)
		}
	}
}

// `rail0 disputes` is the ACCOUNT list; `rail0 payments disputes <id>` is one
// payment's history. Both must exist — the second is not a replacement for the first,
// which is the confusion that left the account list unexposed.
func TestBothDisputeSurfacesExist(t *testing.T) {
	account, _, err := rootCmd.Find([]string{"disputes"})
	if err != nil || account.Args == nil {
		t.Fatalf("account-level disputes: %v", err)
	}
	if err := account.Args(account, []string{"0xsomething"}); err == nil {
		t.Error("rail0 disputes takes no positional argument — it is the account list")
	}
	perPayment, _, err := rootCmd.Find([]string{"payments", "disputes"})
	if err != nil || perPayment.Name() != "disputes" {
		t.Fatalf("per-payment disputes: %v", err)
	}
	if err := perPayment.Args(perPayment, []string{}); err == nil {
		t.Error("rail0 payments disputes requires a rail0Id")
	}
}

// --by is what makes a breakdown a breakdown; without it the gateway answers 400, so
// the check is local.
func TestBreakdownRequiresBy(t *testing.T) {
	analyticsBy = ""
	if err := analyticsBreakdownCmd.RunE(analyticsBreakdownCmd, nil); err == nil {
		t.Error("breakdown without --by must fail locally")
	}
}

func TestAnalyticsFilterFlagsExistOnEveryVerb(t *testing.T) {
	for _, c := range []*cobra.Command{analyticsSummaryCmd, analyticsTimeseriesCmd, analyticsBreakdownCmd} {
		for _, f := range []string{"mode", "status", "token", "chain-id", "from", "to", "payee"} {
			if c.Flags().Lookup(f) == nil {
				t.Errorf("analytics %s is missing --%s", c.Name(), f)
			}
		}
	}
	if analyticsTimeseriesCmd.Flags().Lookup("interval") == nil {
		t.Error("timeseries is missing --interval")
	}
	if analyticsBreakdownCmd.Flags().Lookup("by") == nil {
		t.Error("breakdown is missing --by")
	}
}

// The rows carry their own decimals, so the scale travels with the figure and no
// catalog lookup is needed. A nil decimals leaves the companion out rather than
// guessing.
func TestVolumeCompanions(t *testing.T) {
	six := 6
	if got := formatVolume("4050000", &six); got != "4.05" {
		t.Errorf("formatVolume = %q, want 4.05", got)
	}
	if got := formatVolume("4050000", nil); got != "" {
		t.Errorf("formatVolume with unknown decimals = %q, want empty", got)
	}
	if got := formatVolume("", &six); got != "" {
		t.Errorf("formatVolume of an empty amount = %q, want empty", got)
	}

	vol := rail0.AnalyticsVolume{
		Gross: "4050000", Settled: "3000000", Escrowed: "1050000",
		Captured: "1000000", Refunded: "", Decimals: &six,
	}
	out, ok := summaryView(&rail0.AnalyticsSummary{Volume: []rail0.AnalyticsVolume{vol}}).(summaryOut)
	if !ok {
		t.Fatal("summaryView did not return a summaryOut")
	}
	if len(out.Volume) != 1 {
		t.Fatalf("volume rows = %d", len(out.Volume))
	}
	v := out.Volume[0]
	if v.Gross != "4050000" {
		t.Errorf("raw gross = %q, want the base units untouched", v.Gross)
	}
	if v.GrossHuman != "4.05" || v.CapturedHuman != "1" {
		t.Errorf("companions = (%q, %q)", v.GrossHuman, v.CapturedHuman)
	}
	if v.SettledHuman != "3" || v.EscrowedHuman != "1.05" {
		t.Errorf("settled/escrowed companions = (%q, %q), want (3, 1.05)", v.SettledHuman, v.EscrowedHuman)
	}
	if v.RefundedHuman != "" {
		t.Errorf("refunded_human = %q, want empty for an empty raw value", v.RefundedHuman)
	}
}

func TestSummaryViewNilIsNil(t *testing.T) {
	if summaryView(nil) != nil {
		t.Error("summaryView(nil) must be nil")
	}
}

// Map iteration is randomised, so the same address book used to print in a different
// order every run.
func TestAliasCompletionIsSorted(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Aliases = map[string]string{
		"zebra":  "0x0000000000000000000000000000000000000001",
		"alpha":  "0x0000000000000000000000000000000000000002",
		"middle": "0x0000000000000000000000000000000000000003",
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}
	got, _ := completeAliasNames(nil, nil, "")
	want := []string{"@alpha", "@middle", "@zebra"}
	if len(got) != len(want) {
		t.Fatalf("completions = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("completions = %v, want %v (sorted)", got, want)
		}
	}
}

// --payee narrows every analytics verb to one of the account's wallets, and accepts an
// @alias through the same resolver as the other address flags. Dropping it on any verb
// would silently report the whole account while the merchant believes it sees one wallet,
// and leaving it unset must keep it off the wire — absent is what means "every wallet".
func TestAnalyticsPayeeIsResolvedAndSentOnEveryVerb(t *testing.T) {
	g := newRecordingGateway(t)
	g.respond = func(r *http.Request) any {
		if r.URL.Path == "/analytics/summary" {
			return map[string]any{"orders": 0}
		}
		return nil
	}
	const shop = "0x00000000000000000000000000000000000000aa"
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Aliases = map[string]string{"shop": shop}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}
	t.Cleanup(func() { analyticsPayee, analyticsBy = "", "" })

	analyticsPayee, analyticsBy = "@shop", "token"
	for _, c := range []*cobra.Command{analyticsSummaryCmd, analyticsTimeseriesCmd, analyticsBreakdownCmd} {
		run(t, c)
		q := g.last(t, http.MethodGet, "/analytics/"+c.Name()).query
		if got := q.Get("payee"); got != shop {
			t.Errorf("analytics %s payee = %q, want the @shop alias resolved to %s", c.Name(), got, shop)
		}
	}

	// A literal address passes through untouched.
	analyticsPayee = "0x00000000000000000000000000000000000000bb"
	run(t, analyticsSummaryCmd)
	if got := g.last(t, http.MethodGet, "/analytics/summary").query.Get("payee"); got != analyticsPayee {
		t.Errorf("payee = %q, want the literal address", got)
	}

	// Unset: no payee param, so the gateway keeps the whole-account scope.
	analyticsPayee = ""
	run(t, analyticsSummaryCmd)
	if _, present := g.last(t, http.MethodGet, "/analytics/summary").query["payee"]; present {
		t.Error("payee sent without --payee, want it absent")
	}

	// An unknown alias fails locally, before any request is made.
	analyticsPayee = "@nope"
	before := len(g.reqs)
	err = analyticsSummaryCmd.RunE(analyticsSummaryCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "unknown alias") {
		t.Errorf("unknown alias error = %v, want an 'unknown alias' error", err)
	}
	if len(g.reqs) != before {
		t.Error("an unknown --payee alias still reached the gateway")
	}
}

// --reason is checked against the SDK's dictionaries (rail0-gateway#381) with the
// gateway's own rule — a code, or exactly its bytes32 in any hex case — and sends the
// CODE. The two lists are not interchangeable.
func TestDisputeReasonResolve(t *testing.T) {
	notReceived := rail0.DisputeOpenReasons[0]
	cases := []struct {
		name    string
		list    disputeReasonList
		in      string
		want    rail0.DisputeReason
		wantErr string
	}{
		{"open code", openDisputeReasons, "not_received", rail0.DisputeOpenReasonNotReceived, ""},
		{"bytes32 without 0x", openDisputeReasons, notReceived.Bytes32[2:], "", "unknown open reason"},
		{"bytes32 in upper hex sends the code", openDisputeReasons, "0x" + strings.ToUpper(notReceived.Bytes32[2:]), rail0.DisputeOpenReasonNotReceived, ""},
		{"open bytes32", openDisputeReasons, notReceived.Bytes32, rail0.DisputeOpenReasonNotReceived, ""},
		{"close code on the open list", openDisputeReasons, "withdrawn", "", "use one of: not_received"},
		{"close code", closeDisputeReasons, "withdrawn", rail0.DisputeCloseReasonWithdrawn, ""},
		{"open code on the close list", closeDisputeReasons, "duplicate", "", "unknown close reason"},
		{"system reason", closeDisputeReasons, "full_refund", "", "recorded by the protocol"},
		{"old-style bytes32", closeDisputeReasons, "0x01", "", "rail0 disputes reasons"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.list.resolve(c.in)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Errorf("err = %v, want it to contain %q", err, c.wantErr)
				}
				return
			}
			if err != nil || got != c.want {
				t.Errorf("got (%q, %v), want %q", got, err, c.want)
			}
		})
	}
}

// Without a terminal there is no one to pick for: a missing reason fails with the
// list rather than reaching the gateway's 400.
func TestDisputeReasonIsRequiredWithoutATerminal(t *testing.T) {
	_, err := closeDisputeReasons.orPick("")
	if err == nil || !strings.Contains(err.Error(), "--reason is required") || !strings.Contains(err.Error(), "withdrawn") {
		t.Errorf("err = %v, want the required error listing the close codes", err)
	}
}

// A bad --reason is refused before the signing key, the picker or any request.
func TestDisputeCommandsRejectAnUnknownReasonLocally(t *testing.T) {
	t.Cleanup(func() { disputeReason, closeDisputeReason = "", "" })
	disputeReason, closeDisputeReason = "not_a_reason", "not_received"
	if err := paymentsDisputeCmd.RunE(paymentsDisputeCmd, []string{"0xabc"}); err == nil || !strings.Contains(err.Error(), "unknown open reason") {
		t.Errorf("dispute: err = %v", err)
	}
	if err := paymentsDisputeCloseCmd.RunE(paymentsDisputeCloseCmd, []string{"0xabc"}); err == nil || !strings.Contains(err.Error(), "unknown close reason") {
		t.Errorf("dispute close: err = %v", err)
	}
	for _, c := range []*cobra.Command{paymentsDisputeCmd, paymentsDisputeCloseCmd} {
		if c.Flags().Lookup("reason") == nil {
			t.Errorf("%s has no --reason", c.CommandPath())
		}
	}
}

func TestDisputesReasonsRows(t *testing.T) {
	if cmd, _, err := rootCmd.Find([]string{"disputes", "reasons"}); err != nil || cmd.Name() != "reasons" {
		t.Fatalf("rail0 disputes reasons is not registered (%v)", err)
	}

	all, err := disputeReasonRows("", false)
	if err != nil {
		t.Fatal(err)
	}
	want := len(rail0.DisputeOpenReasons) + len(rail0.DisputeCloseReasons) + len(rail0.DisputeSystemCloseReasons)
	if len(all) != want {
		t.Fatalf("rows = %d, want %d", len(all), want)
	}
	if all[0].Kind != "open" || all[len(all)-1].Code != "full_refund" || all[0].Bytes32 != "" {
		t.Errorf("first %+v, last %+v", all[0], all[len(all)-1])
	}

	closeRows, err := disputeReasonRows("close", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(closeRows) != len(rail0.DisputeCloseReasons) || closeRows[0].Bytes32 != rail0.DisputeCloseReasons[0].Bytes32 {
		t.Errorf("close rows: %+v", closeRows)
	}

	if _, err := disputeReasonRows("opened", false); err == nil {
		t.Error("an unknown --kind must fail")
	}
}

func TestUnknownDisputeReasonHasAHint(t *testing.T) {
	if h := hintFor("unknown_dispute_reason", ""); !strings.Contains(h, "rail0 disputes reasons") {
		t.Errorf("hint = %q", h)
	}
}
