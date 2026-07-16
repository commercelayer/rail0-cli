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
func submitThenWait(ctx context.Context, client *rail0.Client, rail0Id, op string, submit func() (any, error)) (any, error) {
	res, err := submit()
	if err != nil {
		return nil, err
	}
	if !waitFlag {
		return res, nil
	}
	return waitForOp(ctx, client, rail0Id, op, waitTimeout)
}

// waitForOp polls the payment's transactions until the most recent one for `op`
// is confirmed (returns the settled payment) or failed (returns its decoded
// error), or the timeout elapses. The gateway lists transactions newest-first,
// so the first row is the latest attempt of that operation.
func waitForOp(ctx context.Context, client *rail0.Client, rail0Id, op string, timeout time.Duration) (any, error) {
	deadline := time.Now().Add(timeout)
	for {
		txs, err := client.Payments.Transactions(ctx, rail0Id, rail0.ListTransactionsParams{Operation: op})
		if err != nil {
			return nil, err
		}
		if len(txs.Data) > 0 {
			tx := txs.Data[0]
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
		client := newClient()
		ctx := context.Background()
		rail0Id, err := resolvePaymentID(ctx, client, args[0])
		if err != nil {
			return err
		}
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
