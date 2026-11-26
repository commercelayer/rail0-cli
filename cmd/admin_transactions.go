package cmd

import (
	"context"
	"slices"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	adminTxStuck     string
	adminTxStatuses  []string
	adminTxOperation string
	adminTxChainID   int
	adminTxAccount   string
	adminTxHash      string
	adminTxSort      string
	adminTxPage      int
	adminTxPerPage   int
)

// adminTxStuckKinds are the values of --stuck, the gateway's own.
var adminTxStuckKinds = []string{"unbroadcast", "unconfirmed", "any"}

var adminTransactionsCmd = &cobra.Command{
	Use:   "transactions",
	Short: "Transactions across every account, or only the stuck ones",
	Long: `List transactions across every account, each with the payment it belongs to.

--stuck keeps the ones that need attention: 'unbroadcast' (signed, never sent to the
chain), 'unconfirmed' (sent, never confirmed) or 'any'. --hash finds the transaction an
explorer link or a merchant's message names. A stuck broadcast is re-sent with
'rail0 payments redrive <payment> <transaction>' (in write mode, for an operator).`,
	Example: `  rail0 admin transactions --stuck any
  rail0 admin transactions --hash 0xed7c607d…
  rail0 admin transactions --account 019e748b-… --status failed --chain arc
  rail0 admin transactions --operation refund --sort -created_at --pretty`,
	Args: cobra.NoArgs,
	RunE: operatorOnly(func(_ *cobra.Command, _ []string) error {
		if adminTxStuck != "" && !slices.Contains(adminTxStuckKinds, adminTxStuck) {
			return usagef("--stuck must be one of unbroadcast, unconfirmed, any (got %q)", adminTxStuck)
		}
		statuses, err := statusFilterValues("--status", adminTxStatuses, transactionStatuses)
		if err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			ctx := context.Background()
			resp, err := newReadClient().Admin.Transactions(ctx, rail0.ListAdminTransactionsParams{
				Stuck: adminTxStuck, Statuses: statuses, Operation: adminTxOperation,
				ChainID: adminTxChainID, AccountID: adminTxAccount, Hash: adminTxHash,
				Sort: adminTxSort, Page: adminTxPage, PerPage: adminTxPerPage,
			})
			if err != nil {
				return nil, err
			}
			return paged(table(operatorTransactionViews(resp.Data), operatorTransactionColumns), resp.Meta), nil
		})
	}),
}

// operatorTransactionView is an operator's transaction plus its amount in token units,
// read on the scale of the payment the row carries (the same companion every payment
// view has). Omitted when the row has no amount or the payment's decimals are unknown.
type operatorTransactionView struct {
	*rail0.OperatorTransaction
	AmountHuman string `json:"amount_human,omitempty"`
}

func operatorTransactionViews(txs []rail0.OperatorTransaction) []*operatorTransactionView {
	out := make([]*operatorTransactionView, len(txs))
	for i := range txs {
		v := &operatorTransactionView{OperatorTransaction: &txs[i]}
		if p := txs[i].Payment; p != nil && p.Decimals != nil && txs[i].Amount != "" && !ui.IsQuiet() {
			v.AmountHuman = rail0.FormatAmount(string(txs[i].Amount), *p.Decimals)
		}
		out[i] = v
	}
	return out
}

func init() {
	f := adminTransactionsCmd.Flags()
	f.StringVar(&adminTxStuck, "stuck", "", "Only stuck transactions: unbroadcast, unconfirmed or any")
	f.StringSliceVar(&adminTxStatuses, "status", nil, "Filter by status (comma-separated or repeated): pending, submitting, submitted, confirmed, failed")
	f.StringVar(&adminTxOperation, "operation", "", "Filter by operation: authorize, capture, charge, void, release, refund, …")
	addChainFlag(adminTransactionsCmd, &adminTxChainID, "", "Only transactions on this chain")
	f.StringVar(&adminTxAccount, "account", "", "Only transactions on payments whose payee is one of this account's wallets (account id)")
	f.StringVar(&adminTxHash, "hash", "", "The transaction with this hash (0x + 64 hex)")
	f.StringVar(&adminTxSort, "sort", "", "Sort fields (prefix - for descending, comma-separated), e.g. -created_at")
	f.IntVar(&adminTxPage, "page", 0, "Page number (1-based)")
	f.IntVar(&adminTxPerPage, "per-page", 0, "Results per page")
	_ = adminTransactionsCmd.RegisterFlagCompletionFunc("stuck", fixedCompletions(adminTxStuckKinds...))
	_ = adminTransactionsCmd.RegisterFlagCompletionFunc("operation", fixedCompletions(transactionOperations...))
	_ = adminTransactionsCmd.RegisterFlagCompletionFunc("status", fixedListCompletions(transactionStatuses...))
}
