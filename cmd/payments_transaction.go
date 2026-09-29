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

// The gateway's transaction vocabularies — TRANSACTION_OPERATIONS and the status
// values its `status` filter is validated against. Distinct from paymentStatuses on
// purpose: a transaction is `failed`, a payment never is (see cmd/wait.go).
var (
	transactionOperations = []string{
		"authorize", "charge", "capture", "void", "release", "refund", "dispute", "close_dispute",
	}
	transactionStatuses = []string{"pending", "submitting", "submitted", "confirmed", "failed"}
)

// statusFilterValues checks each value of a multi-value --status filter against the
// vocabulary it filters on and returns them trimmed, blanks dropped, for the SDK's
// Statuses (sent as one `status=a,b`, which the gateway matches as ANY of them).
//
// Checked here, before any request, because the gateway rejects the whole filter with
// a 400 for one bad value — and among several values the 400 does not say which. The
// vocabularies are the gateway's own (see paymentStatuses and transactionStatuses), so
// a value refused here is one the gateway would refuse too. A payment status on the
// transaction filter (or the reverse — `failed` on `payments list`) is the mistake this
// catches most: the two vocabularies overlap just enough to invite it.
func statusFilterValues(flag string, values, allowed []string) ([]string, error) {
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if !slices.Contains(allowed, v) {
			return nil, fmt.Errorf("unknown %s value %q; expected one of: %s", flag, v, strings.Join(allowed, ", "))
		}
		out = append(out, v)
	}
	return out, nil
}

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
			// One read, not two: the payment embeds every transaction as the same entity
			// the single-transaction endpoint serves, and it is also where the chain and
			// token that scale the amount come from. That endpoint plus the payment used
			// to be two requests for the same row.
			p, err := client.Payments.Get(ctx, args[0])
			if err != nil {
				return nil, err
			}
			for i := range p.Transactions {
				if p.Transactions[i].ID == args[1] {
					return humanizeTransactionsOf(ctx, p, p.Transactions[i:i+1])[0], nil
				}
			}
			// Not among the payment's rows: ask the endpoint anyway, so the answer is the
			// gateway's own (a 404 for an id that is not this payment's) rather than a
			// message invented here — and a row created since the read above is found.
			tx, err := client.Payments.GetTransaction(ctx, args[0], args[1])
			if err != nil {
				return nil, err
			}
			return humanizeTransactionsOf(ctx, p, []rail0.Transaction{*tx})[0], nil
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
