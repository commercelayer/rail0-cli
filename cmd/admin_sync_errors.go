package cmd

import (
	"context"
	"fmt"
	"slices"
	"strings"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// The sync-error ledger: every indexer callback the gateway could not apply, and why.
// The gateway never prunes it, so an operator reads it and, once a backlog is understood,
// clears it. The clear takes the same filters as the listing and removes exactly what
// they list.

var (
	syncErrReason  string
	syncErrOutcome string
	syncErrTxHash  string
	syncErrPayment string
	syncErrSort    string
	syncErrPage    int
	syncErrPerPage int
	syncErrAll     bool
)

// syncErrorReasons are the reasons the gateway records (its own CHECK constraint).
var syncErrorReasons = []string{
	"not_found", "external_operation", "chain_mismatch", "payment_mismatch",
	"operation_mismatch", "unapplicable", "balance_divergence", "impossible_balances",
}

var syncErrorOutcomes = []string{"confirmed", "failed"}

// syncErrorFilter reads and checks the filter flags the listing and the clear share.
func syncErrorFilter() (rail0.SyncErrorFilter, error) {
	if syncErrReason != "" && !slices.Contains(syncErrorReasons, syncErrReason) {
		return rail0.SyncErrorFilter{}, usagef("--reason must be one of %s (got %q)", strings.Join(syncErrorReasons, ", "), syncErrReason)
	}
	if syncErrOutcome != "" && !slices.Contains(syncErrorOutcomes, syncErrOutcome) {
		return rail0.SyncErrorFilter{}, usagef("--outcome must be confirmed or failed (got %q)", syncErrOutcome)
	}
	if syncErrPayment != "" && !paymentIDPattern.MatchString(syncErrPayment) {
		return rail0.SyncErrorFilter{}, usagef("--payment %q is neither a payment UUID nor a rail0_id (0x followed by 64 hex characters)", syncErrPayment)
	}
	return rail0.SyncErrorFilter{
		Reason: syncErrReason, Outcome: syncErrOutcome, TxHash: syncErrTxHash, PaymentID: syncErrPayment,
	}, nil
}

// describeSyncErrorFilter says a filter in words, for the confirmation prompt.
func describeSyncErrorFilter(f rail0.SyncErrorFilter) string {
	var parts []string
	if f.Reason != "" {
		parts = append(parts, "reason "+f.Reason)
	}
	if f.Outcome != "" {
		parts = append(parts, "outcome "+f.Outcome)
	}
	if f.TxHash != "" {
		parts = append(parts, "transaction "+f.TxHash)
	}
	if f.PaymentID != "" {
		parts = append(parts, "payment "+f.PaymentID)
	}
	return strings.Join(parts, ", ")
}

var adminSyncErrorsCmd = &cobra.Command{
	Use:   "sync-errors",
	Short: "Why an indexer callback could not be applied (lists when run bare)",
	Long: `List the gateway's sync-error ledger, newest first: each row is an on-chain event the
indexer reported and the gateway could not apply, with the reason.

  not_found            no payment has that on-chain id (an operation made outside the gateway)
  external_operation   the payment exists, the operation was not prepared by the gateway
  chain_mismatch, payment_mismatch, operation_mismatch
                       the event contradicts what the gateway holds
  unapplicable         the payment is in a state the event cannot follow
  balance_divergence, impossible_balances
                       the amounts reported do not add up; 'rail0 admin health' counts these

The ledger is never pruned by itself: 'rail0 admin sync-errors clear' removes rows.`,
	Example: `  rail0 admin sync-errors
  rail0 admin sync-errors --reason payment_mismatch --pretty
  rail0 admin sync-errors --payment 0xrail0Id…
  rail0 admin sync-errors --tx-hash 0xed7c607d…`,
	Args: cobra.NoArgs,
	RunE: operatorOnly(func(_ *cobra.Command, _ []string) error {
		filter, err := syncErrorFilter()
		if err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			resp, err := newReadClient().Admin.SyncErrors(context.Background(), rail0.ListSyncErrorsParams{
				SyncErrorFilter: filter, Sort: syncErrSort, Page: syncErrPage, PerPage: syncErrPerPage,
			})
			if err != nil {
				return nil, err
			}
			return paged(table(resp.Data, syncErrorColumns), resp.Meta), nil
		})
	}),
}

var adminSyncErrorsClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Delete the sync errors the filters select (write mode, audited)",
	Long: `Delete rows of the sync-error ledger: exactly the ones the same filters list with
'rail0 admin sync-errors'. The command counts them first and asks before deleting;
--yes skips the question. It cannot be undone, and the gateway audits it with its
filters and the number of rows removed.

At least one filter is required. --all empties the whole ledger instead, and that
includes the balance_divergence rows whose count 'rail0 admin health' reports: to drop a
backlog and keep those, filter by --reason.

A write: it needs write mode ('rail0 admin write-mode on').`,
	Example: `  rail0 admin sync-errors clear --reason not_found
  rail0 admin sync-errors clear --payment 0xrail0Id… --yes
  rail0 admin sync-errors clear --all`,
	Args: cobra.NoArgs,
	RunE: operatorOnly(func(_ *cobra.Command, _ []string) error {
		filter, err := syncErrorFilter()
		if err != nil {
			return err
		}
		filtered := filter != rail0.SyncErrorFilter{}
		switch {
		case filtered && syncErrAll:
			return newUsageError("flag_conflict", "--all empties the whole ledger and cannot be combined with a filter")
		case !filtered && !syncErrAll:
			return usagef("pass at least one filter (--reason, --outcome, --tx-hash, --payment), " +
				"or --all to empty the whole ledger, balance_divergence rows included")
		}
		baseURL := resolveBaseURL()
		if _, err := sessionAddress(); err != nil {
			return err
		}
		if writeModeToken(baseURL) == "" {
			return writeModeRequired()
		}

		// Count before asking, with the filter the delete will use: the question names
		// the number that is about to go.
		ctx := context.Background()
		page, err := newReadClient().Admin.SyncErrors(ctx, rail0.ListSyncErrorsParams{SyncErrorFilter: filter, PerPage: 1})
		if err != nil {
			return err
		}
		total := page.Meta.Total
		if total == 0 {
			return ui.RunAPI(func() (any, error) { return map[string]any{"deleted": 0}, nil })
		}
		what := fmt.Sprintf("the %d sync errors with %s", total, describeSyncErrorFilter(filter))
		if !filtered {
			what = fmt.Sprintf("ALL %d sync errors, balance_divergence rows included", total)
		}
		if err := confirmDestructive(fmt.Sprintf("Delete %s? This cannot be undone.", what)); err != nil {
			return err
		}
		return ui.RunAPIWithMessage("Clearing…", func() (any, error) {
			deleted, err := newClient().Admin.ClearSyncErrors(ctx, filter)
			if err != nil {
				return nil, err
			}
			return map[string]any{"deleted": deleted}, nil
		})
	}),
}

// addSyncErrorFilterFlags registers the filters the listing and the clear share.
func addSyncErrorFilterFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVar(&syncErrReason, "reason", "", "Only this rejection reason: "+strings.Join(syncErrorReasons, ", "))
	f.StringVar(&syncErrOutcome, "outcome", "", "Only this kind of callback: confirmed or failed")
	f.StringVar(&syncErrTxHash, "tx-hash", "", "Only rows for this transaction hash")
	f.StringVar(&syncErrPayment, "payment", "", "Only rows about this payment: its id (UUID) or its rail0_id (0x…)")
	_ = cmd.RegisterFlagCompletionFunc("reason", fixedCompletions(syncErrorReasons...))
	_ = cmd.RegisterFlagCompletionFunc("outcome", fixedCompletions(syncErrorOutcomes...))
}

func init() {
	addSyncErrorFilterFlags(adminSyncErrorsCmd)
	f := adminSyncErrorsCmd.Flags()
	f.StringVar(&syncErrSort, "sort", "", "Sort by created_at or reason (prefix - for descending; default: newest first)")
	f.IntVar(&syncErrPage, "page", 0, "Page number (1-based)")
	f.IntVar(&syncErrPerPage, "per-page", 0, "Results per page")

	addSyncErrorFilterFlags(adminSyncErrorsClearCmd)
	adminSyncErrorsClearCmd.Flags().BoolVar(&syncErrAll, "all", false, "Empty the whole ledger (no filter), balance_divergence rows included")
	addYesFlag(adminSyncErrorsClearCmd)
}
