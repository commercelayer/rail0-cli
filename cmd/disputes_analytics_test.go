package cmd

import (
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
		for _, f := range []string{"mode", "status", "token", "chain-id", "from", "to"} {
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

	vol := rail0.AnalyticsVolume{Gross: "4050000", Captured: "1000000", Refunded: "", Decimals: &six}
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
