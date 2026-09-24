package cmd

import (
	"context"

	rail0 "github.com/commercelayer/rail0-go"

	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// The gateway's transaction vocabularies — TRANSACTION_OPERATIONS and the status
// values its `status` filter is validated against. Distinct from paymentStatuses on
// purpose: a transaction is `failed`, a payment never is (see cmd/wait.go).
var (
	transactionOperations = []string{
		"authorize", "charge", "capture", "void", "release", "refund", "dispute", "close_dispute",
	}
	transactionStatuses = []string{"pending", "submitting", "submitted", "confirmed", "failed"}
)

// ── payments transaction ──────────────────────────────────────────────────────

var paymentsTransactionCmd = &cobra.Command{
	Use:   "transaction <rail0Id> <transaction_id>",
	Short: "Fetch one of a payment's transactions",
	Long: `Fetch a single transaction of a payment by its id — the id every lifecycle
command prints when the gateway accepts an operation (202).

The direct lookup for "what happened to THAT attempt": its status, the decoded
revert reason of a failed one, and whether a stuck one is 'redrivable'.
Readable by either participant.`,
	Example: `  rail0 payments transaction 0xabc123… 7f1c0c8e-…`,
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			ctx := context.Background()
			client := newReadClient()
			tx, err := client.Payments.GetTransaction(ctx, args[0], args[1])
			if err != nil {
				return nil, err
			}
			return humanizeTransactions(ctx, client, args[0], []rail0.Transaction{*tx})[0], nil
		})
	},
}

// ── payments redrive ──────────────────────────────────────────────────────────

var paymentsRedriveCmd = &cobra.Command{
	Use:   "redrive <rail0Id> <transaction_id>",
	Short: "Re-send a signed transaction whose broadcast never reached the chain (payee)",
	Long: `Ask the gateway to broadcast again a transaction that was prepared and SIGNED
but never reached the chain — a worker that died between the two, a drained queue.
The same signed bytes go out again; nothing about the payment changes, and no new
signature is asked for.

Only a transaction reading 'redrivable: true' qualifies (see 'payments
transaction' or 'payments transactions'). A pending row WITHOUT a signed
transaction is not stuck, it is unfinished: re-run the operation instead. The
gateway also redrives on its own recovery cadence; this asks for it now.

Payee only. Add -w/--wait to block until the transaction confirms (or fails).`,
	Example: `  rail0 payments redrive 0xabc123… 7f1c0c8e-… --wait`,
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		client := newClient()
		rail0Id := args[0]
		return ui.RunAPIWithMessage(actionLabel("Redriving…"), func() (any, error) {
			tx, err := client.Payments.Redrive(ctx, rail0Id, args[1])
			if err != nil {
				return nil, err
			}
			if !waitFlag {
				return tx, nil
			}
			return waitForOp(ctx, client, rail0Id, tx.Operation, tx.ID, true, waitTimeout)
		})
	},
}

func init() {
	addWaitFlags(paymentsRedriveCmd)
	paymentsCmd.AddCommand(paymentsTransactionCmd, paymentsRedriveCmd)
}
