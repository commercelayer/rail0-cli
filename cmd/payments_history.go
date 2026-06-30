package cmd

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/rail0/cli/internal/token"
	"github.com/rail0/cli/internal/ui"
	rail0 "github.com/rail0/go-sdk"
	"github.com/spf13/cobra"
)

// paymentsHistoryCmd prints a payment's chronological history: the `created` and
// `signed` milestones plus one entry per on-chain transaction (authorize,
// capture, void, release, refund, dispute, close_dispute), oldest first, with
// only the significant fields. It's assembled client-side from a single
// Payments.Get — no dedicated gateway endpoint.
//
// Requires a logged-in session (rail0 auth login); a rail0_id argument is also
// resolved through the authenticated lookup.
var paymentsHistoryCmd = &cobra.Command{
	Use:   "history <rail0Id>",
	Short: "Show a payment's chronological history (requires login)",
	Long: `Print the timeline of everything that happened to a payment since it was
created: the created and signed milestones and every on-chain transaction
(authorize/capture/void/release/refund/dispute/close_dispute), oldest first.
Only the most significant fields are shown per entry (time, event, status,
amount, tx hash, block).

Requires a session — run 'rail0 auth login' first.`,
	Example: `  rail0 payments history 0xabc123…
  rail0 payments history 019e748b-…`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if token.Load() == "" {
			return errors.New("payments history requires a session — run 'rail0 auth login' first")
		}
		ctx := context.Background()
		client := newClient()
		id, err := resolvePaymentID(ctx, client, args[0])
		if err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			payment, err := client.Payments.Get(ctx, id)
			if err != nil {
				return nil, err
			}
			return paymentHistory(payment), nil
		})
	},
}

// paymentHistory turns a payment into a chronological list of events, each
// carrying only its significant fields (empty ones omitted). The transactions
// already arrive oldest-first; the whole list is re-sorted by time so the
// created/signed milestones interleave correctly.
func paymentHistory(p *rail0.Payment) []any {
	type event struct {
		at     string
		fields map[string]any
	}
	events := []event{
		{p.CreatedAt, map[string]any{
			"at": p.CreatedAt, "event": "created", "mode": p.Mode, "amount": string(p.Amount),
			"from": string(p.Payer), "to": string(p.Payee),
		}},
	}
	// The payer's offline signing. Use signed_at when known; otherwise (payment
	// signed before signed_at was tracked) still record it — any status past
	// "unsigned" means it was signed — placed right after `created` with an
	// unknown time.
	if p.SignedAt != "" {
		events = append(events, event{p.SignedAt, map[string]any{"at": p.SignedAt, "event": "signed"}})
	} else if p.Status != "" && p.Status != "unsigned" {
		events = append(events, event{p.CreatedAt, map[string]any{"at": nil, "event": "signed"}})
	}
	for _, t := range p.Transactions {
		at := firstNonEmptyStr(t.ConfirmedAt, t.SubmittedAt, t.PendingAt, t.CreatedAt)
		f := map[string]any{"at": at, "event": t.Operation, "status": t.Status}
		if amt := string(t.Amount); amt != "" {
			f["amount"] = amt
		}
		if t.TransactionHash != nil && string(*t.TransactionHash) != "" {
			f["tx_hash"] = string(*t.TransactionHash)
		}
		if t.BlockNumber != nil {
			f["block"] = *t.BlockNumber
		}
		events = append(events, event{at, f})
	}

	sort.SliceStable(events, func(i, j int) bool { return eventTimeLess(events[i].at, events[j].at) })

	out := make([]any, len(events))
	for i, e := range events {
		out[i] = e.fields
	}
	return out
}

// eventTimeLess orders two timestamps chronologically, parsing RFC3339 (with
// optional fractional seconds); if either can't be parsed it falls back to a
// lexicographic compare, which is still chronological for ISO-8601 strings.
func eventTimeLess(a, b string) bool {
	ta, ea := time.Parse(time.RFC3339, a)
	tb, eb := time.Parse(time.RFC3339, b)
	if ea == nil && eb == nil {
		return ta.Before(tb)
	}
	return a < b
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func init() {
	paymentsCmd.AddCommand(paymentsHistoryCmd)
}
