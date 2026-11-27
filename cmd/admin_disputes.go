package cmd

import (
	"context"
	"slices"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// Disputes across accounts. `rail0 disputes` is the list of one wallet's payments; the
// operator's question is platform-wide: what is open, since when, on which chain, with
// which reason. Read-only: a dispute is opened and closed by the payer, on chain.

var (
	adminDispStatus  string
	adminDispPayment string
	adminDispChainID int
	adminDispReason  string
	adminDispFrom    string
	adminDispTo      string
	adminDispSort    string
	adminDispPage    int
	adminDispPerPage int
)

var disputeStatuses = []string{"open", "closed"}

var adminDisputesCmd = &cobra.Command{
	Use:   "disputes",
	Short: "Every dispute across accounts, open and closed",
	Long: `List the disputes of every account, newest first, each with the payment it is about
(ids, chain, payer, payee, amount) and both reasons read from the dictionary.

--status open is what is being contested right now. --reason takes one of the codes a
payer can open a dispute with ('rail0 disputes reasons'); --from and --to bound when the
dispute was opened.

Read-only: a dispute is opened and closed by the payer, on chain, and a full refund
closes it. 'rail0 admin trace <payment>' shows one payment's disputes among everything
else that happened to it.`,
	Example: `  rail0 admin disputes --status open
  rail0 admin disputes --status open --chain arc --pretty
  rail0 admin disputes --reason not_received --from 2026-11-01T00:00:00Z
  rail0 admin disputes --payment 0xrail0Id…`,
	Args: cobra.NoArgs,
	RunE: operatorOnly(func(_ *cobra.Command, _ []string) error {
		if adminDispStatus != "" && !slices.Contains(disputeStatuses, adminDispStatus) {
			return usagef("--status must be open or closed (got %q)", adminDispStatus)
		}
		if adminDispPayment != "" && !paymentIDPattern.MatchString(adminDispPayment) {
			return usagef("--payment %q is neither a payment UUID nor a rail0_id (0x followed by 64 hex characters)", adminDispPayment)
		}
		reason, err := openDisputeReasons.resolve(adminDispReason)
		if err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			resp, err := newReadClient().Admin.Disputes(context.Background(), rail0.ListAdminDisputesParams{
				Status: adminDispStatus, PaymentID: adminDispPayment, ChainID: adminDispChainID,
				ReasonCode: string(reason), From: adminDispFrom, To: adminDispTo,
				Sort: adminDispSort, Page: adminDispPage, PerPage: adminDispPerPage,
			})
			if err != nil {
				return nil, err
			}
			return paged(table(resp.Data, operatorDisputeColumns), resp.Meta), nil
		})
	}),
}

func init() {
	f := adminDisputesCmd.Flags()
	f.StringVar(&adminDispStatus, "status", "", "Only disputes that are open or closed")
	f.StringVar(&adminDispPayment, "payment", "", "Only this payment's disputes: its id (UUID) or its rail0_id (0x…)")
	addChainFlag(adminDisputesCmd, &adminDispChainID, "", "Only disputes on payments of this chain")
	f.StringVar(&adminDispReason, "reason", "", "Only disputes opened with this reason code (see 'rail0 disputes reasons')")
	f.StringVar(&adminDispFrom, "from", "", "Only disputes opened at/after this ISO-8601 time")
	f.StringVar(&adminDispTo, "to", "", "Only disputes opened at/before this ISO-8601 time")
	f.StringVar(&adminDispSort, "sort", "", "Sort by opened_at or closed_at (prefix - for descending; default: newest first)")
	f.IntVar(&adminDispPage, "page", 0, "Page number (1-based)")
	f.IntVar(&adminDispPerPage, "per-page", 0, "Results per page")
	_ = adminDisputesCmd.RegisterFlagCompletionFunc("status", fixedCompletions(disputeStatuses...))
	_ = adminDisputesCmd.RegisterFlagCompletionFunc("reason", fixedCompletions(openDisputeReasons.codes()...))
}
