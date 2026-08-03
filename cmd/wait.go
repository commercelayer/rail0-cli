package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rail0/cli/internal/ui"
	rail0 "github.com/rail0/go-sdk"
	"github.com/spf13/cobra"
)

var (
	waitFlag    bool
	waitTimeout time.Duration
	waitUntil   string
)

const waitPollInterval = 2 * time.Second

// addWaitFlags registers --wait/-w and --timeout on an async on-chain action, so
// the command can block until the operation confirms (or fails) instead of
// returning at the gateway's 202.
func addWaitFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&waitFlag, "wait", "w", false, "Block until the operation confirms on-chain (or fails)")
	cmd.Flags().DurationVar(&waitTimeout, "timeout", 2*time.Minute, "Max time to wait when --wait is set")
}

// actionLabel appends a "waiting" hint to the spinner label when --wait is set,
// so the user sees the command has moved from broadcasting to awaiting the chain.
func actionLabel(base string) string {
	if waitFlag {
		return strings.TrimSuffix(base, "…") + " (waiting for confirmation)…"
	}
	return base
}

// submitThenWait runs the submit step and, when --wait is set, polls until the
// operation's transaction confirms — returning the settled payment — or fails.
// Without --wait it returns the raw submit response (the 202 body), preserving
// the previous behaviour.
//
// The submit response is no longer discarded: every submit endpoint answers 202 with
// the transaction row it just created, so its id is what the wait follows. See
// pickWaitTarget for what that fixes. (#66)
func submitThenWait(ctx context.Context, client *rail0.Client, rail0Id, op string, submit func() (any, error)) (any, error) {
	res, err := submit()
	if err != nil {
		return nil, err
	}
	if !waitFlag {
		return res, nil
	}
	return waitForOp(ctx, client, rail0Id, op, submittedTransactionID(res), waitTimeout)
}

// submittedTransactionID extracts the transaction id from a submit response. All
// eight submit methods return *rail0.Transaction, so the assertion holds; it returns
// "" on any other shape so waitForOp degrades to its previous newest-first behaviour
// rather than failing a command whose operation has ALREADY been broadcast.
func submittedTransactionID(res any) string {
	if tx, ok := res.(*rail0.Transaction); ok {
		return tx.ID
	}
	return ""
}

// waitForOp polls the payment's transactions until the one this attempt created is
// confirmed (returns the settled payment) or failed (returns its decoded error), or
// the timeout elapses. `txID` identifies that attempt; see pickWaitTarget.
func waitForOp(ctx context.Context, client *rail0.Client, rail0Id, op, txID string, timeout time.Duration) (any, error) {
	deadline := time.Now().Add(timeout)
	for {
		txs, err := client.Payments.Transactions(ctx, rail0Id, rail0.ListTransactionsParams{Operation: op})
		if err != nil {
			return nil, err
		}
		if tx := pickWaitTarget(txs.Data, txID); tx != nil {
			switch tx.Status {
			case "confirmed":
				return client.Payments.Get(ctx, rail0Id)
			case "failed":
				reason := tx.ErrorMessage
				if reason == "" {
					reason = tx.ErrorCode
				}
				if reason == "" {
					reason = "reverted"
				}
				if h := hintFor(tx.ErrorCode, ""); h != "" {
					return nil, fmt.Errorf("%s failed on-chain: %s — %s", op, reason, h)
				}
				return nil, fmt.Errorf("%s failed on-chain: %s", op, reason)
			}
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out after %s; run 'rail0 payments get %s' to check", timeout, shortID(rail0Id))
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(waitPollInterval):
		}
	}
}

// pickWaitTarget returns the transaction the wait is following, or nil to keep
// polling.
//
// With a txID it matches on that id, which is the whole fix. Trusting list order
// alone was wrong for any operation that can be attempted more than once: on a
// second partial capture, capture #1 is already `confirmed` and sits at index 0,
// while capture #2's row has not been persisted yet — so the first poll read #1,
// reported "confirmed", and the command exited successfully having waited for the
// wrong transaction. The operator was told capture #2 had settled when it had only
// been broadcast.
//
// Returning nil when the id is absent from the listing is deliberate: the row
// certainly exists (the gateway returned it in the 202), so this is a read that
// raced the write, and the right response is to poll again — NOT to fall back to
// index 0, which is precisely the confusion being removed.
//
// The empty-txID path keeps the old newest-first behaviour for callers that have no
// submit response to key on. (#66)
func pickWaitTarget(txs []rail0.Transaction, txID string) *rail0.Transaction {
	if txID != "" {
		for i := range txs {
			if txs[i].ID == txID {
				return &txs[i]
			}
		}
		return nil
	}
	if len(txs) > 0 {
		return &txs[0]
	}
	return nil
}

// anyInFlight reports whether any transaction is still in a non-terminal state.
func anyInFlight(txs []rail0.Transaction) bool {
	for _, t := range txs {
		switch t.Status {
		case "pending", "submitting", "submitted":
			return true
		}
	}
	return false
}

// ── payments wait ────────────────────────────────────────────────────────────

var paymentsWaitCmd = &cobra.Command{
	Use:   "wait <rail0Id>",
	Short: "Block until a payment settles (or reaches a given status)",
	Long: `Poll a payment until it settles, then print its final state.

By default it waits until no transaction is still in flight (pending/submitted).
With --until it instead waits for a specific payment status.`,
	Example: `  rail0 payments wait 0xrail0Id…
  rail0 payments wait 0xrail0Id… --until captured`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newReadClient()
		ctx := context.Background()
		rail0Id := args[0]
		return ui.RunAPIWithMessage("Waiting…", func() (any, error) {
			deadline := time.Now().Add(waitTimeout)
			for {
				if waitUntil != "" {
					p, err := client.Payments.Get(ctx, rail0Id)
					if err != nil {
						return nil, err
					}
					if p.Status == waitUntil {
						return p, nil
					}
				} else {
					txs, err := client.Payments.Transactions(ctx, rail0Id)
					if err != nil {
						return nil, err
					}
					if !anyInFlight(txs.Data) {
						return client.Payments.Get(ctx, rail0Id)
					}
				}
				if time.Now().After(deadline) {
					return nil, fmt.Errorf("timed out after %s", waitTimeout)
				}
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(waitPollInterval):
				}
			}
		})
	},
}

func init() {
	paymentsWaitCmd.Flags().StringVar(&waitUntil, "until", "", "Wait until the payment reaches this status (e.g. captured, refunded)")
	paymentsWaitCmd.Flags().DurationVar(&waitTimeout, "timeout", 2*time.Minute, "Max time to wait")
	paymentsCmd.AddCommand(paymentsWaitCmd)
}
