package cmd

import (
	"context"
	"slices"
	"strings"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// Every account's payouts and payout destinations, read-only. The gateway is
// non-custodial and every payout write is the destination wallet's own ('rail0 payouts'):
// an operator can look, not act. With the payouts feature off on the gateway these routes
// do not exist and every command here answers 404 ('rail0 admin config payouts' says
// whether it is on).

var (
	adminPoAccount     string
	adminPoWallet      string
	adminPoDestination string
	adminPoChainID     int
	adminPoStatuses    []string
	adminPoPurpose     string
	adminPoCreatedFrom string
	adminPoCreatedTo   string
	adminPoSort        string
	adminPoPage        int
	adminPoPerPage     int
	adminPoEventsPage  int
	adminPoEventsPer   int
)

const payoutsReadOnlyNote = `Read-only: every payout write is the destination wallet's own, and the gateway holds no
funds. With the payouts feature off on this gateway the command answers 404
('rail0 admin config payouts' says whether it is on).`

var adminPayoutsCmd = &cobra.Command{
	Use:   "payouts",
	Short: "Every account's payouts, read-only (lists when run bare)",
	Long: `List the payouts of every account, newest first: each with its account, destination,
amount, status and, when it failed, why (error_code, error_reason).

` + payoutsReadOnlyNote,
	Example: `  rail0 admin payouts --status failed --pretty
  rail0 admin payouts --account 019e748b-… --created-from 2026-11-01T00:00:00Z
  rail0 admin payouts get 01a3f0c2-…
  rail0 admin payouts events 01a3f0c2-…
  rail0 admin payouts destinations --status unverified`,
	Args: cobra.NoArgs,
	RunE: operatorOnly(func(_ *cobra.Command, _ []string) error {
		statuses, err := statusFilterValues("--status", adminPoStatuses, payoutStatuses)
		if err != nil {
			return err
		}
		if adminPoPurpose != "" && !slices.Contains(payoutPurposes, adminPoPurpose) {
			return usagef("--purpose must be payout or proof (got %q)", adminPoPurpose)
		}
		return ui.RunAPI(func() (any, error) {
			resp, err := newReadClient().Admin.Payouts(context.Background(), rail0.ListAdminPayoutsParams{
				AccountID: adminPoAccount, WalletID: adminPoWallet, DestinationID: adminPoDestination,
				ChainID: adminPoChainID, Statuses: statuses, Purpose: adminPoPurpose,
				CreatedFrom: adminPoCreatedFrom, CreatedTo: adminPoCreatedTo,
				Sort: adminPoSort, Page: adminPoPage, PerPage: adminPoPerPage,
			})
			if err != nil {
				return nil, err
			}
			return paged(table(resp.Data, operatorPayoutColumns), resp.Meta), nil
		})
	}),
}

var adminPayoutsGetCmd = &cobra.Command{
	Use:     "get <payout>",
	Short:   "One payout of any account: the whole record but its signed payload",
	Long:    payoutsReadOnlyNote,
	Example: `  rail0 admin payouts get 01a3f0c2-…`,
	Args:    cobra.ExactArgs(1),
	RunE: operatorOnly(func(_ *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newReadClient().Admin.Payout(context.Background(), args[0])
		})
	}),
}

var adminPayoutsEventsCmd = &cobra.Command{
	Use:     "events <payout>",
	Short:   "A payout's audit trail, newest first",
	Long:    payoutsReadOnlyNote,
	Example: `  rail0 admin payouts events 01a3f0c2-… --pretty`,
	Args:    cobra.ExactArgs(1),
	RunE: operatorOnly(func(_ *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			resp, err := newReadClient().Admin.PayoutEvents(context.Background(), args[0],
				rail0.ListPayoutEventsParams{Page: adminPoEventsPage, PerPage: adminPoEventsPer})
			if err != nil {
				return nil, err
			}
			return paged(resp.Data, resp.Meta), nil
		})
	}),
}

// ── admin payouts destinations ───────────────────────────────────────────────

var (
	adminPdAccount   string
	adminPdWallet    string
	adminPdChainID   int
	adminPdStatus    string
	adminPdSort      string
	adminPdPage      int
	adminPdPerPage   int
	adminPdEventsPg  int
	adminPdEventsPer int
)

var adminPayoutDestinationsCmd = &cobra.Command{
	Use:   "destinations",
	Short: "Every account's payout destinations, read-only (lists when run bare)",
	Long: `List the payout destinations of every account: each with its account, wallet, chain,
token, address, label and status (unverified, verified, disabled), who registered it and
who confirmed it.

` + payoutsReadOnlyNote,
	Example: `  rail0 admin payouts destinations --status unverified --pretty
  rail0 admin payouts destinations --account 019e748b-…
  rail0 admin payouts destinations get 01a3e9b1-…
  rail0 admin payouts destinations events 01a3e9b1-…`,
	Args: cobra.NoArgs,
	RunE: operatorOnly(func(_ *cobra.Command, _ []string) error {
		if adminPdStatus != "" && !slices.Contains(payoutDestinationStatuses, adminPdStatus) {
			return usagef("--status must be one of %s (got %q)", strings.Join(payoutDestinationStatuses, ", "), adminPdStatus)
		}
		return ui.RunAPI(func() (any, error) {
			resp, err := newReadClient().Admin.PayoutDestinations(context.Background(), rail0.ListAdminPayoutDestinationsParams{
				AccountID: adminPdAccount, WalletID: adminPdWallet, ChainID: adminPdChainID, Status: adminPdStatus,
				Sort: adminPdSort, Page: adminPdPage, PerPage: adminPdPerPage,
			})
			if err != nil {
				return nil, err
			}
			return paged(table(resp.Data, operatorPayoutDestinationColumns), resp.Meta), nil
		})
	}),
}

var adminPayoutDestinationsGetCmd = &cobra.Command{
	Use:     "get <destination>",
	Short:   "One payout destination of any account: the whole record",
	Long:    payoutsReadOnlyNote,
	Example: `  rail0 admin payouts destinations get 01a3e9b1-…`,
	Args:    cobra.ExactArgs(1),
	RunE: operatorOnly(func(_ *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newReadClient().Admin.PayoutDestination(context.Background(), args[0])
		})
	}),
}

var adminPayoutDestinationsEventsCmd = &cobra.Command{
	Use:     "events <destination>",
	Short:   "A destination's audit trail, newest first: its own events and its payouts'",
	Long:    payoutsReadOnlyNote,
	Example: `  rail0 admin payouts destinations events 01a3e9b1-… --pretty`,
	Args:    cobra.ExactArgs(1),
	RunE: operatorOnly(func(_ *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			resp, err := newReadClient().Admin.PayoutDestinationEvents(context.Background(), args[0],
				rail0.ListPayoutEventsParams{Page: adminPdEventsPg, PerPage: adminPdEventsPer})
			if err != nil {
				return nil, err
			}
			return paged(resp.Data, resp.Meta), nil
		})
	}),
}

// isAdminPayoutsCommand reports whether cmd is `rail0 admin payouts` or one of its
// descendants: the commands whose 404 may mean "the feature is off", not "wrong id".
func isAdminPayoutsCommand(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c == adminPayoutsCmd {
			return true
		}
	}
	return false
}

func init() {
	f := adminPayoutsCmd.Flags()
	f.StringVar(&adminPoAccount, "account", "", "Only this account's payouts (account id)")
	f.StringVar(&adminPoWallet, "wallet", "", "Only payouts from this wallet (wallet id)")
	f.StringVar(&adminPoDestination, "destination", "", "Only payouts to this destination (destination id)")
	addChainFlag(adminPayoutsCmd, &adminPoChainID, "", "Only payouts on this chain")
	f.StringSliceVar(&adminPoStatuses, "status", nil,
		"Filter by status ("+strings.Join(payoutStatuses, ", ")+"); repeat or comma-separate for any of several")
	f.StringVar(&adminPoPurpose, "purpose", "", "Filter by purpose: payout or proof")
	f.StringVar(&adminPoCreatedFrom, "created-from", "", "Only payouts created at/after this ISO-8601 time")
	f.StringVar(&adminPoCreatedTo, "created-to", "", "Only payouts created at/before this ISO-8601 time")
	f.StringVar(&adminPoSort, "sort", "", "Sort fields (e.g. -created_at)")
	f.IntVar(&adminPoPage, "page", 0, "Page number (1-based)")
	f.IntVar(&adminPoPerPage, "per-page", 0, "Payouts per page (max 100)")
	_ = adminPayoutsCmd.RegisterFlagCompletionFunc("status", fixedListCompletions(payoutStatuses...))
	_ = adminPayoutsCmd.RegisterFlagCompletionFunc("purpose", fixedCompletions(payoutPurposes...))
	adminPayoutsEventsCmd.Flags().IntVar(&adminPoEventsPage, "page", 0, "Page number (1-based)")
	adminPayoutsEventsCmd.Flags().IntVar(&adminPoEventsPer, "per-page", 0, "Events per page")

	d := adminPayoutDestinationsCmd.Flags()
	d.StringVar(&adminPdAccount, "account", "", "Only this account's destinations (account id)")
	d.StringVar(&adminPdWallet, "wallet", "", "Only this wallet's destinations (wallet id)")
	addChainFlag(adminPayoutDestinationsCmd, &adminPdChainID, "", "Only destinations on this chain")
	d.StringVar(&adminPdStatus, "status", "", "Filter by status: "+strings.Join(payoutDestinationStatuses, ", "))
	d.StringVar(&adminPdSort, "sort", "", "Sort by created_at, updated_at, label or status (prefix - for descending)")
	d.IntVar(&adminPdPage, "page", 0, "Page number (1-based)")
	d.IntVar(&adminPdPerPage, "per-page", 0, "Destinations per page")
	_ = adminPayoutDestinationsCmd.RegisterFlagCompletionFunc("status", fixedCompletions(payoutDestinationStatuses...))
	adminPayoutDestinationsEventsCmd.Flags().IntVar(&adminPdEventsPg, "page", 0, "Page number (1-based)")
	adminPayoutDestinationsEventsCmd.Flags().IntVar(&adminPdEventsPer, "per-page", 0, "Events per page")
}
