package cmd

import (
	"context"
	"sort"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

var adminTraceTimeline bool

// traceLimit is how many rows of each kind a trace reads. One payment has a handful of
// each; past the limit the trace says so (`totals`) instead of paging silently.
const traceLimit = 100

var adminTraceCmd = &cobra.Command{
	Use:   "trace <payment>",
	Short: "Everything about one payment: transactions, sync errors, deliveries, disputes, operator actions",
	Long: `Answer "what happened to this payment" in one command, whoever the payment belongs to.

The payment is read with its transactions, and then everything else that names it:
the sync errors (indexer callbacks about it that could not be applied), the webhook
deliveries across every subscription, its disputes, and what operators did to it.
'totals' says how many of each exist, in case one kind has more than a page.

--timeline prints the same facts as one chronological list instead: the payment's own
history with the sync errors, deliveries, disputes and operator actions in their place.

The argument is the payment's id (UUID) or its rail0_id (0x…, 64 hex).`,
	Example: `  rail0 admin trace 0xrail0Id…
  rail0 admin trace 019e748b-… --timeline --pretty`,
	Args: cobra.ExactArgs(1),
	RunE: operatorOnly(func(_ *cobra.Command, args []string) error {
		id := args[0]
		if !paymentIDPattern.MatchString(id) {
			return usagef("%q is neither a payment UUID nor a rail0_id (0x followed by 64 hex characters)", id)
		}
		return ui.RunAPIWithMessage("tracing…", func() (any, error) {
			ctx := context.Background()
			t, err := tracePayment(ctx, newReadClient(), id)
			if err != nil {
				return nil, err
			}
			if adminTraceTimeline {
				return table(traceTimeline(ctx, t), traceTimelineColumns), nil
			}
			return t.view(ctx), nil
		})
	}),
}

// paymentTrace is what a trace gathers about one payment.
type paymentTrace struct {
	Payment    *rail0.Payment
	SyncErrors *rail0.PaginatedResponse[rail0.SyncError]
	Deliveries *rail0.PaginatedResponse[rail0.OperatorEventCallback]
	Disputes   *rail0.PaginatedResponse[rail0.OperatorDispute]
	Actions    *rail0.PaginatedResponse[rail0.AdminAction]
}

// tracePayment reads the payment and, by its ids, everything that names it. The payment
// comes first: an id that names none fails here with the gateway's 404, and the four
// listings need the ids it resolves to — the ledger and the deliveries know the payment
// by either id, the audit log by its UUID only.
func tracePayment(ctx context.Context, client *rail0.Client, id string) (*paymentTrace, error) {
	p, err := client.Payments.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	t := &paymentTrace{Payment: p}
	if t.SyncErrors, err = client.Admin.SyncErrors(ctx, rail0.ListSyncErrorsParams{
		SyncErrorFilter: rail0.SyncErrorFilter{PaymentID: p.ID}, PerPage: traceLimit,
	}); err != nil {
		return nil, err
	}
	if t.Deliveries, err = client.Admin.EventCallbacks(ctx, rail0.ListAdminEventCallbacksParams{
		PaymentID: p.ID, PerPage: traceLimit,
	}); err != nil {
		return nil, err
	}
	if t.Disputes, err = client.Admin.Disputes(ctx, rail0.ListAdminDisputesParams{
		PaymentID: p.ID, PerPage: traceLimit,
	}); err != nil {
		return nil, err
	}
	if t.Actions, err = client.Admin.Actions(ctx, rail0.ListAdminActionsParams{
		PaymentID: p.ID, PerPage: traceLimit,
	}); err != nil {
		return nil, err
	}
	return t, nil
}

// view is the trace as sections: the payment as every payment view prints it (its
// transactions inside), then one list per kind, and how many of each exist.
func (t *paymentTrace) view(ctx context.Context) map[string]any {
	return map[string]any{
		"payment":     humanize(ctx, t.Payment),
		"sync_errors": t.SyncErrors.Data,
		"deliveries":  traceDeliveries(t.Deliveries.Data),
		"disputes":    t.Disputes.Data,
		"actions":     t.Actions.Data,
		"totals": map[string]int{
			"transactions": len(t.Payment.Transactions),
			"sync_errors":  listTotal(t.SyncErrors.Meta, len(t.SyncErrors.Data)),
			"deliveries":   listTotal(t.Deliveries.Meta, len(t.Deliveries.Data)),
			"disputes":     listTotal(t.Disputes.Meta, len(t.Disputes.Data)),
			"actions":      listTotal(t.Actions.Meta, len(t.Actions.Data)),
		},
	}
}

// listTotal is a listing's total from its paging headers, or the rows in hand when the
// gateway sent none.
func listTotal(meta rail0.PageMeta, rows int) int {
	if meta.Total > rows {
		return meta.Total
	}
	return rows
}

// traceDelivery is a delivery without its payload: the request body is the whole
// event, several kilobytes a row, and it is what 'webhooks event-callbacks' is for.
type traceDelivery struct {
	rail0.OperatorEventCallback
	Payload string `json:"payload,omitempty"` // shadows the embedded field: always empty
}

func traceDeliveries(rows []rail0.OperatorEventCallback) []traceDelivery {
	out := make([]traceDelivery, len(rows))
	for i := range rows {
		out[i] = traceDelivery{OperatorEventCallback: rows[i]}
	}
	return out
}

// traceTimeline merges a trace into one chronological list: the payment's own history
// (created, signed, every transaction — the same rows as 'payments history'), with the
// sync errors, deliveries, disputes and operator actions where they happened. Each row
// says its kind, the event, its status and the one detail worth a column.
func traceTimeline(ctx context.Context, t *paymentTrace) []map[string]any {
	var rows []map[string]any
	for _, e := range paymentHistory(ctx, t.Payment) {
		row, _ := e.(map[string]any)
		row["kind"] = "payment"
		if row["event"] != "created" && row["event"] != "signed" {
			row["kind"] = "transaction"
		}
		if hash, _ := row["tx_hash"].(string); hash != "" {
			row["detail"] = hash
		}
		rows = append(rows, row)
	}
	for _, e := range t.SyncErrors.Data {
		rows = append(rows, map[string]any{
			"at": e.CreatedAt, "kind": "sync_error", "event": firstNonEmptyStr(e.EventType, e.Outcome),
			"status": e.Reason, "detail": e.TransactionHash,
		})
	}
	for _, d := range t.Deliveries.Data {
		detail := d.ErrorReason
		if d.Webhook != nil {
			detail = firstNonEmptyStr(detail, d.Webhook.Name, d.Webhook.CallbackURL)
		}
		row := map[string]any{"at": d.CreatedAt, "kind": "delivery", "event": d.Topic, "status": d.Status, "detail": detail}
		if d.ResponseCode != nil {
			row["code"] = *d.ResponseCode
		}
		rows = append(rows, row)
	}
	for _, d := range t.Disputes.Data {
		rows = append(rows, map[string]any{
			"at": d.OpenedAt, "kind": "dispute", "event": "opened", "status": "open",
			"detail": firstNonEmptyStr(string(d.ReasonCode), d.Reason),
		})
		if d.ClosedAt != "" {
			rows = append(rows, map[string]any{
				"at": d.ClosedAt, "kind": "dispute", "event": "closed", "status": "closed",
				"detail": firstNonEmptyStr(string(d.CloseReasonCode), d.ClosedBy),
			})
		}
	}
	for _, a := range t.Actions.Data {
		rows = append(rows, map[string]any{
			"at": a.CreatedAt, "kind": "operator", "event": a.Action, "detail": a.ActorAddress,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, _ := rows[i]["at"].(string)
		b, _ := rows[j]["at"].(string)
		return eventTimeLess(a, b)
	})
	return rows
}

func init() {
	adminTraceCmd.Flags().BoolVar(&adminTraceTimeline, "timeline", false,
		"Print one chronological list instead of the sections")
}
