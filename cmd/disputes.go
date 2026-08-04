package cmd

import (
	"context"

	"github.com/rail0/cli/internal/ui"
	rail0 "github.com/rail0/go-sdk"
	"github.com/spf13/cobra"
)

var (
	accountDisputesStatus  string
	accountDisputesSort    string
	accountDisputesPage    int
	accountDisputesPerPage int
)

// `rail0 disputes` — the ACCOUNT-level list, distinct from `rail0 payments disputes
// <rail0Id>`, which is one payment's open/close history. The gateway has had both for
// a while and the CLI exposed only the second, so the question a merchant actually
// asks — "is anything disputed right now?" — had no answer short of walking every
// payment. (#110)
//
// Bare = list, following the noun→verb shape of `wallets`, `aliases` and `keys`.
var disputesCmd = &cobra.Command{
	Use:   "disputes",
	Short: "List every dispute on your payments (requires login)",
	Long: `List the disputes across the payments your wallet is party to, as payer or
payee, each with its parent payment embedded.

Open AND closed: a closed dispute stops setting the 'disputed' flag on
'payments list' (which reports current state) but is still listed here, so the
history stays visible. Filter with --status to see only one of the two.

Distinct from 'rail0 payments disputes <rail0Id>', which is the open/close history
of ONE payment.`,
	Example: `  rail0 disputes
  rail0 disputes --status open
  rail0 disputes --sort -opened_at --per-page 50`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return ui.RunAPI(func() (any, error) {
			ctx := context.Background()
			resp, err := newReadClient().Disputes.List(ctx, rail0.ListDisputesParams{
				Status:  accountDisputesStatus,
				Sort:    accountDisputesSort,
				Page:    accountDisputesPage,
				PerPage: accountDisputesPerPage,
			})
			if err != nil {
				return nil, err
			}
			return resp.Data, nil // bare array; pagination is in headers
		})
	},
}

func init() {
	disputesCmd.Flags().StringVar(&accountDisputesStatus, "status", "", "Filter by dispute status: open or closed")
	disputesCmd.Flags().StringVar(&accountDisputesSort, "sort", "", "Sort fields (e.g. -opened_at)")
	disputesCmd.Flags().IntVar(&accountDisputesPage, "page", 0, "Page number (1-based)")
	disputesCmd.Flags().IntVar(&accountDisputesPerPage, "per-page", 0, "Results per page")

	// A finite set, so it completes: `--status ope<TAB>` beats reading --help, and a
	// typo here costs a round-trip to find out.
	_ = disputesCmd.RegisterFlagCompletionFunc("status",
		fixedCompletions("open", "closed"))
}
