package cmd

import (
	"context"
	"fmt"

	"github.com/rail0/cli/internal/ui"
	rail0 "github.com/rail0/go-sdk"
	"github.com/spf13/cobra"
)

// `rail0 analytics` — the account's sales KPIs, which the gateway has served all
// along and the CLI never exposed: rail0-admin reads /analytics/* and the CLI had zero
// occurrences of the SDK's AnalyticsService, so the numbers a merchant reports on were
// dashboard-only. (#110, CLAUDE.md rule 11)
//
// Three verbs, matching the three endpoints: headline figures, a time series, and a
// grouping. The filters are identical across all three, so they are registered from
// one place.

var (
	analyticsMode     string
	analyticsStatus   string
	analyticsToken    string
	analyticsChainID  int
	analyticsFrom     string
	analyticsTo       string
	analyticsInterval string
	analyticsBy       string
)

var analyticsCmd = &cobra.Command{
	Use:   "analytics",
	Short: "Account sales analytics: summary, timeseries, breakdown (requires login)",
	Long: `Read the account's sales analytics.

  summary     — headline figures: order count, disputed count, refund and dispute
                rates, per-status counts, and per-token volume
  timeseries  — order counts bucketed by day, week or month
  breakdown   — the same counts grouped by token, chain, mode or status

All three take the same filters (--mode, --status, --token, --chain-id, --from,
--to). Requires an account: a wallet-only session gets 403 account_required, since
these are the MERCHANT's figures, not an address's.`,
	Example: `  rail0 analytics summary
  rail0 analytics summary --from 2026-01-01T00:00:00Z
  rail0 analytics timeseries --interval week
  rail0 analytics breakdown --by token`,
}

// analyticsFilters collects the shared filter flags into the SDK's struct.
func analyticsFiltersFromFlags() rail0.AnalyticsFilters {
	return rail0.AnalyticsFilters{
		Mode:    analyticsMode,
		Status:  analyticsStatus,
		Token:   analyticsToken,
		ChainID: analyticsChainID,
		From:    analyticsFrom,
		To:      analyticsTo,
	}
}

var analyticsSummaryCmd = &cobra.Command{
	Use:     "summary",
	Short:   "Headline figures: orders, disputes, refund/dispute rates, volume",
	Example: `  rail0 analytics summary --chain-id 5042002`,
	Args:    cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return ui.RunAPI(func() (any, error) {
			s, err := newReadClient().Analytics.Summary(context.Background(), analyticsFiltersFromFlags())
			if err != nil {
				return nil, err
			}
			return summaryView(s), nil
		})
	},
}

var analyticsTimeseriesCmd = &cobra.Command{
	Use:   "timeseries",
	Short: "Order counts bucketed by day, week or month",
	Long: `Order counts over time, one row per bucket.

Each row carries a volume only when a SINGLE token and chain are filtered: summing
different tokens' base units would be meaningless, so the gateway returns null
instead of a number that looks addable.`,
	Example: `  rail0 analytics timeseries
  rail0 analytics timeseries --interval month --chain-id 5042002 --token 0x3600…`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return ui.RunAPI(func() (any, error) {
			return newReadClient().Analytics.Timeseries(context.Background(), rail0.TimeseriesParams{
				AnalyticsFilters: analyticsFiltersFromFlags(),
				Interval:         analyticsInterval,
			})
		})
	},
}

var analyticsBreakdownCmd = &cobra.Command{
	Use:   "breakdown",
	Short: "Group the counts by token, chain, mode or status (--by is required)",
	Long: `Group the account's orders by one dimension.

--by is required: token, chain, mode or status. token and chain rows carry volume
(they identify a scale); mode and status rows are counts only, because a group
spanning several tokens has no single unit to add up in.`,
	Example: `  rail0 analytics breakdown --by token
  rail0 analytics breakdown --by status --mode charge`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		if analyticsBy == "" {
			return fmt.Errorf("--by is required: token, chain, mode or status")
		}
		return ui.RunAPI(func() (any, error) {
			rows, err := newReadClient().Analytics.Breakdown(context.Background(), rail0.BreakdownParams{
				AnalyticsFilters: analyticsFiltersFromFlags(),
				By:               analyticsBy,
			})
			if err != nil {
				return nil, err
			}
			return breakdownView(rows), nil
		})
	},
}

// ── human companions ─────────────────────────────────────────────────────────
//
// The analytics rows carry their own `decimals`, so unlike a payment they need no
// token-catalog lookup to be rendered: the scale travels with the figure. Amounts get
// a *_human companion beside the raw base units for the reason the project rule
// exists — a bare 4050000 reads as four million, not 4.05 — and the raw fields stay
// exactly as the gateway sent them.
//
// A nil `decimals` (a token no longer in the catalog) leaves the companion out rather
// than guessing a scale.

type volumeView struct {
	rail0.AnalyticsVolume
	GrossHuman    string `json:"gross_human,omitempty"`
	CapturedHuman string `json:"captured_human,omitempty"`
	RefundedHuman string `json:"refunded_human,omitempty"`
}

type summaryOut struct {
	*rail0.AnalyticsSummary
	Volume []volumeView `json:"volume"`
}

// formatVolume renders a base-unit figure with the decimals the row carries, or ""
// when either is missing.
func formatVolume(base string, decimals *int) string {
	if base == "" || decimals == nil {
		return ""
	}
	return rail0.FormatAmount(base, *decimals)
}

func summaryView(s *rail0.AnalyticsSummary) any {
	if s == nil {
		return nil
	}
	vols := make([]volumeView, 0, len(s.Volume))
	for _, v := range s.Volume {
		vols = append(vols, volumeView{
			AnalyticsVolume: v,
			GrossHuman:      formatVolume(v.Gross, v.Decimals),
			CapturedHuman:   formatVolume(v.Captured, v.Decimals),
			RefundedHuman:   formatVolume(v.Refunded, v.Decimals),
		})
	}
	return summaryOut{AnalyticsSummary: s, Volume: vols}
}

type rowView struct {
	rail0.AnalyticsRow
	VolumeHuman string `json:"volume_human,omitempty"`
}

func breakdownView(rows []rail0.AnalyticsRow) []rowView {
	out := make([]rowView, 0, len(rows))
	for _, r := range rows {
		human := ""
		if r.Volume != nil {
			human = formatVolume(*r.Volume, r.Decimals)
		}
		out = append(out, rowView{AnalyticsRow: r, VolumeHuman: human})
	}
	return out
}

func init() {
	// One registration for the filters shared by all three verbs.
	for _, c := range []*cobra.Command{analyticsSummaryCmd, analyticsTimeseriesCmd, analyticsBreakdownCmd} {
		c.Flags().StringVar(&analyticsMode, "mode", "", "Filter by mode: authorize or charge")
		c.Flags().StringVar(&analyticsStatus, "status", "", "Filter by payment status (e.g. captured)")
		c.Flags().StringVar(&analyticsToken, "token", "", "Filter by token address (0x…)")
		c.Flags().IntVar(&analyticsChainID, "chain-id", 0, "Filter by EVM chain ID")
		c.Flags().StringVar(&analyticsFrom, "from", "", "Only payments created at/after this ISO-8601 time")
		c.Flags().StringVar(&analyticsTo, "to", "", "Only payments created at/before this ISO-8601 time")

		_ = c.RegisterFlagCompletionFunc("mode", fixedCompletions("authorize", "charge"))
		_ = c.RegisterFlagCompletionFunc("status", fixedCompletions(paymentStatuses...))
	}

	analyticsTimeseriesCmd.Flags().StringVar(&analyticsInterval, "interval", "", "Bucket width: day (default), week or month")
	_ = analyticsTimeseriesCmd.RegisterFlagCompletionFunc("interval", fixedCompletions("day", "week", "month"))

	analyticsBreakdownCmd.Flags().StringVar(&analyticsBy, "by", "", "Group by: token, chain, mode or status (required)")
	_ = analyticsBreakdownCmd.RegisterFlagCompletionFunc("by", fixedCompletions("token", "chain", "mode", "status"))

	analyticsCmd.AddCommand(analyticsSummaryCmd, analyticsTimeseriesCmd, analyticsBreakdownCmd)
}
