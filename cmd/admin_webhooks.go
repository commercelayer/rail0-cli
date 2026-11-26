package cmd

import (
	"context"
	"slices"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// Webhooks across accounts: every subscription with its circuit and its newest attempt,
// the delivery log, and the circuit reset. `rail0 webhooks` stays the merchant's: its own
// subscriptions, their secrets, their deliveries one subscription at a time. An operator
// never sees a signing key here: the gateway does not return one.

var (
	adminWhAccount string
	adminWhCircuit string
	adminWhTopic   string
	adminWhSort    string
	adminWhPage    int
	adminWhPerPage int
)

var circuitStates = []string{"open", "closed"}

var adminWebhooksCmd = &cobra.Command{
	Use:   "webhooks",
	Short: "Every account's webhook subscriptions, with circuit and last attempt (lists when run bare)",
	Long: `List the webhook subscriptions of every account: each with the account that owns it,
whether it is active, its circuit state and failure count, and when and how its newest
delivery attempt went. The open circuits come first.

A circuit opens after 10 consecutive failed events and never closes by itself: nothing is
delivered to that subscription until it is reset, by its merchant ('rail0 webhooks
reset-circuit') or here ('rail0 admin webhooks reset-circuit'). --circuit open lists the
subscriptions that are receiving nothing right now.

No signing key is shown: a shared secret is returned once, to the subscription's owner.`,
	Example: `  rail0 admin webhooks --circuit open               # who is receiving nothing
  rail0 admin webhooks --account 019e748b-… --pretty
  rail0 admin webhooks --topic payments.captured --sort -circuit_failure_count
  rail0 admin deliveries --webhook 01a0f3c2-… --status failed   # what it was failing on`,
	Args: cobra.NoArgs,
	RunE: operatorOnly(func(_ *cobra.Command, _ []string) error {
		if adminWhCircuit != "" && !slices.Contains(circuitStates, adminWhCircuit) {
			return usagef("--circuit must be open or closed (got %q)", adminWhCircuit)
		}
		return ui.RunAPI(func() (any, error) {
			resp, err := newReadClient().Admin.Webhooks(context.Background(), rail0.ListAdminWebhooksParams{
				AccountID: adminWhAccount, CircuitState: adminWhCircuit, Topic: adminWhTopic,
				Sort: adminWhSort, Page: adminWhPage, PerPage: adminWhPerPage,
			})
			if err != nil {
				return nil, err
			}
			return paged(table(resp.Data, operatorWebhookColumns), resp.Meta), nil
		})
	}),
}

var adminWebhooksResetCircuitCmd = &cobra.Command{
	Use:   "reset-circuit <webhook>",
	Short: "Close a subscription's circuit breaker and re-enable it, on any account (write mode, audited)",
	Long: `Close the delivery circuit breaker of a subscription and re-enable it, whoever it
belongs to: the same write as the merchant's 'rail0 webhooks reset-circuit'.

It re-arms delivery to an endpoint the operator does not control, so check first that
the endpoint is back: 'rail0 admin deliveries --webhook <webhook> --status failed' shows
what it was failing on. Resetting a breaker in front of a receiver that still fails only
spends another 10 events reopening it. Events that fired while the circuit was open were
not delivered, and the reset does not send them.

The gateway audits it with the callback URL. No confirmation is asked: nothing is lost,
and a subscription that fails again opens its circuit again.

A write: it needs write mode ('rail0 admin write-mode on').`,
	Example: `  rail0 admin webhooks reset-circuit 01a0f3c2-…`,
	Args:    cobra.ExactArgs(1),
	RunE: operatorOnly(func(_ *cobra.Command, args []string) error {
		if err := requireWriteMode(); err != nil {
			return err
		}
		return ui.RunAPIWithMessage("Resetting…", func() (any, error) {
			return newClient().Admin.ResetWebhookCircuit(context.Background(), args[0])
		})
	}),
}

// ── admin deliveries ─────────────────────────────────────────────────────────

var (
	adminDelPayment string
	adminDelWebhook string
	adminDelStatus  string
	adminDelTopic   string
	adminDelCode    string
	adminDelSince   string
	adminDelUntil   string
	adminDelSort    string
	adminDelPage    int
	adminDelPerPage int
	adminDelPayload bool
)

var deliveryStatuses = []string{"delivered", "failed"}

var adminDeliveriesCmd = &cobra.Command{
	Use:   "deliveries",
	Short: "Webhook deliveries across accounts: what a merchant was told, and whether it landed",
	Long: `List webhook delivery attempts across every account, newest first, each with the
subscription it went to and that subscription's account.

Start from what is at hand: --payment (what was this payment's merchant told), --webhook
(what is this subscription failing on), or neither with --status failed for everything
that is not landing. --response-code tells a receiver's crashes (500) from its refusals
(401, 429). The gateway keeps 15 days of attempts.

The event body (payload) is left out: it is the whole event, several kilobytes a row.
--payload includes it.`,
	Example: `  rail0 admin deliveries --status failed --since 2026-11-26T00:00:00Z
  rail0 admin deliveries --payment 0xrail0Id…
  rail0 admin deliveries --webhook 01a0f3c2-… --status failed --response-code 500
  rail0 admin deliveries --topic payments.captured --pretty`,
	Args: cobra.NoArgs,
	RunE: operatorOnly(func(_ *cobra.Command, _ []string) error {
		if adminDelStatus != "" && !slices.Contains(deliveryStatuses, adminDelStatus) {
			return usagef("--status must be delivered or failed (got %q)", adminDelStatus)
		}
		if adminDelPayment != "" && !paymentIDPattern.MatchString(adminDelPayment) {
			return usagef("--payment %q is neither a payment UUID nor a rail0_id (0x followed by 64 hex characters)", adminDelPayment)
		}
		return ui.RunAPI(func() (any, error) {
			resp, err := newReadClient().Admin.EventCallbacks(context.Background(), rail0.ListAdminEventCallbacksParams{
				PaymentID: adminDelPayment, WebhookID: adminDelWebhook, Status: adminDelStatus, Topic: adminDelTopic,
				ResponseCode: adminDelCode, Since: adminDelSince, Until: adminDelUntil,
				Sort: adminDelSort, Page: adminDelPage, PerPage: adminDelPerPage,
			})
			if err != nil {
				return nil, err
			}
			var rows any = traceDeliveries(resp.Data)
			if adminDelPayload {
				rows = resp.Data
			}
			return paged(table(rows, operatorDeliveryColumns), resp.Meta), nil
		})
	}),
}

func init() {
	f := adminWebhooksCmd.Flags()
	f.StringVar(&adminWhAccount, "account", "", "Only this account's subscriptions (account id)")
	f.StringVar(&adminWhCircuit, "circuit", "", "Only subscriptions whose circuit is open or closed")
	f.StringVar(&adminWhTopic, "topic", "", "Only subscriptions that include this topic")
	f.StringVar(&adminWhSort, "sort", "", "Sort by created_at, updated_at, circuit_failure_count or name (prefix - for descending; default: open circuits first)")
	f.IntVar(&adminWhPage, "page", 0, "Page number (1-based)")
	f.IntVar(&adminWhPerPage, "per-page", 0, "Results per page")
	_ = adminWebhooksCmd.RegisterFlagCompletionFunc("circuit", fixedCompletions(circuitStates...))
	_ = adminWebhooksCmd.RegisterFlagCompletionFunc("topic", fixedCompletions(webhookTopics...))

	d := adminDeliveriesCmd.Flags()
	d.StringVar(&adminDelPayment, "payment", "", "Only deliveries for this payment: its id (UUID) or its rail0_id (0x…)")
	d.StringVar(&adminDelWebhook, "webhook", "", "Only this subscription's deliveries (webhook id)")
	d.StringVar(&adminDelStatus, "status", "", "Only deliveries that were delivered or failed")
	d.StringVar(&adminDelTopic, "topic", "", "Only deliveries of this event topic")
	d.StringVar(&adminDelCode, "response-code", "", "Only deliveries the receiver answered with exactly this HTTP status")
	d.StringVar(&adminDelSince, "since", "", "Only deliveries at/after this ISO-8601 time")
	d.StringVar(&adminDelUntil, "until", "", "Only deliveries at/before this ISO-8601 time")
	d.StringVar(&adminDelSort, "sort", "", "Sort by created_at or status (prefix - for descending; default: newest first)")
	d.IntVar(&adminDelPage, "page", 0, "Page number (1-based)")
	d.IntVar(&adminDelPerPage, "per-page", 0, "Results per page")
	d.BoolVar(&adminDelPayload, "payload", false, "Include each delivery's event body")
	_ = adminDeliveriesCmd.RegisterFlagCompletionFunc("status", fixedCompletions(deliveryStatuses...))
	_ = adminDeliveriesCmd.RegisterFlagCompletionFunc("topic", fixedCompletions(webhookTopics...))
}
