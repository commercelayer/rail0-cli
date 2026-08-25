package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/rail0/cli/internal/ui"
	rail0 "github.com/rail0/go-sdk"
	"github.com/spf13/cobra"
)

var webhooksCmd = &cobra.Command{
	Use:   "webhooks",
	Short: "Webhook subscription management (requires login)",
	Example: `  rail0 webhooks list
  rail0 webhooks create --name "order hook" \
    --callback-url https://example.com/hook -t payments.captured -t payments.refunded
  rail0 webhooks event-callbacks whk_abc123 --status failed   # why isn't it arriving?`,
	Long: `Manage webhook subscriptions for the authenticated account.

A webhook subscribes to one or more payment-lifecycle topics and delivers a
signed callback to your endpoint. One subscription means ONE shared secret and
ONE circuit breaker however many topics it carries, and each delivery names the
event that fired in X-Rail0-Topic — so four events on one endpoint is one
subscription, not four to keep in step. The secret is returned only once, on
create and rotate-secret.

Two subscriptions for the same callback URL must not overlap: the same event
arriving twice under two different secrets is indistinguishable from a duplicate
at the receiving end, so the gateway refuses it and names the topic that
collided.

A webhook delivers only when it is active, and there are TWO independent ways
for it to stop being active:

  disabled       you turned it off              → 'enable' brings it back
  circuit open   10 consecutive failed events   → 'reset-circuit', and
                 tripped the breaker              nothing else, brings it back

They are not interchangeable. A tripped breaker also marks the webhook
disabled, so 'enable' appears to work — it returns 200 — while the webhook
stays silent, because the breaker is still open. Read 'active' in the response
rather than trusting the exit code: it is false whenever either switch is off,
and 'circuit_state' says which.`,
}

// ── webhooks list ──────────────────────────────────────────────────────────────

var webhooksListTopic string
var webhooksListActive string
var webhooksListCircuit string
var webhooksListSort string
var webhooksListPage int
var webhooksListPerPage int

var webhooksListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List webhooks for the authenticated account",
	Example: `  rail0 webhooks list --topic payments.captured --active true`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		p := rail0.ListWebhooksParams{
			Topic:        webhooksListTopic,
			CircuitState: webhooksListCircuit,
			Sort:         webhooksListSort,
			Page:         webhooksListPage,
			PerPage:      webhooksListPerPage,
		}
		active, err := parseBoolFlag(webhooksListActive)
		if err != nil {
			return err
		}
		p.Active = active
		return ui.RunAPI(func() (any, error) {
			resp, err := newReadClient().Webhooks.List(context.Background(), p)
			if err != nil {
				return nil, err
			}
			return resp.Data, nil // bare array; pagination is in headers
		})
	},
}

// ── webhooks get ────────────────────────────────────────────────────────────────

var webhooksGetCmd = &cobra.Command{
	Use:     "get <webhook_id>",
	Short:   "Fetch a single webhook",
	Example: `  rail0 webhooks get whk_abc123`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newReadClient().Webhooks.Get(context.Background(), args[0])
		})
	},
}

// ── webhooks create ──────────────────────────────────────────────────────────────

var webhooksCreateName string
var webhooksCreateURL string
var webhooksCreateTopics []string

var webhooksCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Register a new webhook (returns the one-time shared secret)",
	Long: `Register a webhook for one or more topics on the logged-in account.

Repeat -t/--topic for each event, or pass them comma-separated. They share one
secret and one circuit breaker, which is the point: a per-topic subscription
means a key to verify against and a rotation to coordinate for each one.

The response carries the shared SECRET, and it is the only time it is shown:
the gateway stores a hash, not the value, so it cannot be read back. Capture it
now — 'rotate-secret' issues a new one, which is the recovery, not a lookup.

Use the secret to verify the X-Rail0-Signature header on each delivery. A
delivery whose signature you cannot verify should be treated as untrusted, not
as a malformed request.`,
	Example: `  rail0 webhooks create --name "order lifecycle" \
    --callback-url https://example.com/hook \
    -t payments.authorized -t payments.captured -t payments.voided -t payments.refunded`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := rail0.CreateWebhookRequest{
			Name:        webhooksCreateName,
			CallbackURL: webhooksCreateURL,
			Topics:      webhooksCreateTopics,
		}
		return ui.RunAPI(func() (any, error) {
			return newClient().Webhooks.Create(context.Background(), req)
		})
	},
}

// ── webhooks update ──────────────────────────────────────────────────────────────

var webhooksUpdateName string
var webhooksUpdateURL string
var webhooksUpdateTopics []string

var webhooksUpdateCmd = &cobra.Command{
	Use:   "update <webhook_id>",
	Short: "Update a webhook's name, callback URL, or topics",
	Long: `Change a webhook's name, callback URL, or topics in place. The id and the shared
secret are kept, so the receiver keeps verifying with the secret it already has.

--topic REPLACES the whole set, which is also how a topic is REMOVED: pass the
ones you want to keep. Adding one means passing the existing topics plus the new
one, so read them first with 'webhooks get <id>'.

This does NOT change whether the webhook is active. Repointing a broken webhook
at a URL that works leaves it exactly as inactive as it was — which is the
second half of the repair sequence worth knowing:

  1. 'webhooks event-callbacks <id> --status failed'   what was it failing on
  2. 'webhooks update <id> --callback-url …'           fix it
  3. 'webhooks reset-circuit <id>'                     let it deliver again

Step 3 is not optional after a tripped breaker, and doing it before step 2 just
spends another 10 events reopening it.`,
	Example: `  rail0 webhooks update whk_abc123 --callback-url https://example.com/new`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := rail0.UpdateWebhookRequest{
			Name:        webhooksUpdateName,
			CallbackURL: webhooksUpdateURL,
			Topics:      webhooksUpdateTopics,
		}
		return ui.RunAPI(func() (any, error) {
			return newClient().Webhooks.Update(context.Background(), args[0], req)
		})
	},
}

// ── webhooks enable / disable / rotate-secret / reset-circuit ─────────────────────

var webhooksEnableCmd = &cobra.Command{
	Use:   "enable <webhook_id>",
	Short: "Re-enable a disabled webhook",
	Long: `Turn a manually disabled webhook back on.

This clears ONLY the manual disable. If the delivery circuit breaker tripped,
the webhook stays inactive and no delivery goes out — the command still returns
200, so check 'active' in the response rather than the exit code. When it comes
back false with circuit_state "open", the fix is 'reset-circuit'.`,
	Example: `  rail0 webhooks enable whk_abc123`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newClient().Webhooks.Enable(context.Background(), args[0])
		})
	},
}

var webhooksDisableCmd = &cobra.Command{
	Use:   "disable <webhook_id>",
	Short: "Disable a webhook",
	Long: `Stop deliveries without deleting the webhook.

Reversible with 'enable', and it keeps the subscription, its id and its secret —
which is what makes it the right tool for a receiver that is down for
maintenance. 'delete' is the permanent form and loses the secret with it.

Note this is only ONE of the two ways a webhook goes inactive; see the circuit
breaker under 'rail0 webhooks --help'.`,
	Example: `  rail0 webhooks disable whk_abc123`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newClient().Webhooks.Disable(context.Background(), args[0])
		})
	},
}

var webhooksRotateSecretCmd = &cobra.Command{
	Use:   "rotate-secret <webhook_id>",
	Short: "Generate a new shared secret (returned once on the response)",
	Long: `Issue a new shared secret for a webhook, returned once on the response.

The old secret stops verifying immediately — there is no overlap window. Any
delivery already in flight was signed with the old one, so a receiver that has
not yet been updated will reject it and the delivery counts as failed, which
feeds the circuit breaker. Update the receiver first where you can, or expect a
short window of failures.

Rotate on suspected exposure, and after anyone who had the secret loses
access.`,
	Example: `  rail0 webhooks rotate-secret whk_abc123`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Gated like every other destructive command. Rotating is not "undoable by
		// rotating back": the previous secret is gone, and from the instant this
		// returns, every live consumer verifying X-Rail0-Signature against the old
		// one rejects deliveries — a silent production break with no undo. The
		// prompt names that consequence rather than just asking "are you sure".
		if err := confirmDestructive(fmt.Sprintf(
			"Rotate the shared secret for webhook %s? Consumers still verifying with the "+
				"current secret will reject deliveries until they are updated.", args[0],
		)); err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return newClient().Webhooks.RotateSecret(context.Background(), args[0])
		})
	},
}

var webhooksResetCircuitCmd = &cobra.Command{
	Use:   "reset-circuit <webhook_id>",
	Short: "Reset the delivery circuit breaker and re-enable the webhook",
	Long: `Close the delivery circuit breaker and re-enable the webhook.

The breaker opens after 10 consecutive failed EVENTS (a distinct event, not
each retry of one), and opening it also marks the webhook disabled. Once open
it never closes on its own: a later success cannot close it, because nothing is
delivered while it is open. This command is the only way back, and it clears
the manual disable at the same time — so it works whichever way the webhook
went inactive.

Fix the endpoint FIRST. Resetting a breaker in front of a receiver that is
still failing just spends another 10 events reopening it. 'event-callbacks
<id> --status failed' shows what it was failing on.`,
	Example: `  rail0 webhooks reset-circuit whk_abc123`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newClient().Webhooks.ResetCircuit(context.Background(), args[0])
		})
	},
}

// ── webhooks event-callbacks ──────────────────────────────────────────────────────

var webhooksEventsStatus string
var webhooksEventsSort string
var webhooksEventsTopic string
var webhooksEventsPaymentID string
var webhooksEventsSince string
var webhooksEventsUntil string

var webhooksEventCallbacksCmd = &cobra.Command{
	Use:   "event-callbacks <webhook_id>",
	Short: "List delivery attempts for a webhook",
	Long: `List the delivery attempts for a webhook — the log to read when a callback did
not arrive.

Each row is one attempt, with its status and the event it carried, so it
answers the two questions worth asking: is the gateway trying at all (no rows
means the event never dispatched — check that the webhook is active and the
topic matches), and if it is, what is the receiver returning.

'--status failed' is the usual starting point, and the one to read before
'reset-circuit'.`,
	Example: `  rail0 webhooks event-callbacks whk_abc123 --status failed
  rail0 webhooks event-callbacks whk_abc123 --topic payments.captured
  rail0 webhooks event-callbacks whk_abc123 --payment-id 0x… --since 2026-01-01T00:00:00Z`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := rail0.ListEventCallbacksParams{
			Status:    webhooksEventsStatus,
			Topic:     webhooksEventsTopic,
			PaymentID: webhooksEventsPaymentID,
			Since:     webhooksEventsSince,
			Until:     webhooksEventsUntil,
			Sort:      webhooksEventsSort,
		}
		return ui.RunAPI(func() (any, error) {
			resp, err := newReadClient().Webhooks.EventCallbacks(context.Background(), args[0], p)
			if err != nil {
				return nil, err
			}
			return resp.Data, nil // bare array; pagination is in headers
		})
	},
}

// ── webhooks delete ──────────────────────────────────────────────────────────────

var webhooksDeleteCmd = &cobra.Command{
	Use:   "delete <webhook_id>",
	Short: "Remove a webhook",
	Long: `Remove a webhook permanently.

The subscription and its shared secret go with it — there is no undo and no way
to recover the secret, so a recreated webhook needs its receiver reconfigured
with the new one. To stop deliveries reversibly, use 'disable' instead.`,
	Example: `  rail0 webhooks delete whk_abc123`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := confirmDestructive(fmt.Sprintf(
			"Delete webhook %s? Its delivery history goes with it and this cannot be undone.", args[0],
		)); err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return nil, newClient().Webhooks.Delete(context.Background(), args[0])
		})
	},
}

// parseBoolFlag turns a "true"/"false"/"" string flag into an *bool, returning
// nil for the empty string (no filter) and an error for any other value.
func parseBoolFlag(s string) (*bool, error) {
	switch s {
	case "":
		return nil, nil
	case "true":
		t := true
		return &t, nil
	case "false":
		f := false
		return &f, nil
	default:
		return nil, errors.New("flag must be 'true' or 'false'")
	}
}

func init() {
	webhooksListCmd.Flags().StringVar(&webhooksListTopic, "topic", "",
		"Show subscriptions that INCLUDE this topic (e.g. payments.captured)")
	webhooksListCmd.Flags().StringVar(&webhooksListActive, "active", "", "Filter by active flag ('true' or 'false')")
	webhooksListCmd.Flags().StringVar(&webhooksListCircuit, "circuit-state", "", "Filter by circuit state ('closed' or 'open')")
	webhooksListCmd.Flags().StringVar(&webhooksListSort, "sort", "", "Sort fields (e.g. -created_at)")
	webhooksListCmd.Flags().IntVar(&webhooksListPage, "page", 0, "Page number (1-based)")
	webhooksListCmd.Flags().IntVar(&webhooksListPerPage, "per-page", 0, "Results per page")
	webhooksCmd.AddCommand(webhooksListCmd)

	webhooksCmd.AddCommand(webhooksGetCmd)

	webhooksCreateCmd.Flags().StringVarP(&webhooksCreateName, "name", "n", "", "Human-readable webhook name")
	webhooksCreateCmd.Flags().StringVarP(&webhooksCreateURL, "callback-url", "u", "", "HTTPS endpoint that receives deliveries")
	// StringSlice, so -t repeats and also accepts a comma-separated list.
	webhooksCreateCmd.Flags().StringSliceVarP(&webhooksCreateTopics, "topic", "t", nil,
		"Topic to subscribe to; repeat for several (e.g. -t payments.captured -t payments.refunded)")
	_ = webhooksCreateCmd.MarkFlagRequired("name")
	_ = webhooksCreateCmd.MarkFlagRequired("callback-url")
	_ = webhooksCreateCmd.MarkFlagRequired("topic")
	webhooksCmd.AddCommand(webhooksCreateCmd)

	webhooksUpdateCmd.Flags().StringVarP(&webhooksUpdateName, "name", "n", "", "New webhook name")
	webhooksUpdateCmd.Flags().StringVarP(&webhooksUpdateURL, "callback-url", "u", "", "New callback URL")
	webhooksUpdateCmd.Flags().StringSliceVarP(&webhooksUpdateTopics, "topic", "t", nil,
		"REPLACE the topic set; repeat for several. Omitting a topic removes it")
	webhooksCmd.AddCommand(webhooksUpdateCmd)

	webhooksCmd.AddCommand(webhooksEnableCmd)
	webhooksCmd.AddCommand(webhooksDisableCmd)
	addYesFlag(webhooksRotateSecretCmd)
	webhooksCmd.AddCommand(webhooksRotateSecretCmd)
	webhooksCmd.AddCommand(webhooksResetCircuitCmd)

	webhooksEventCallbacksCmd.Flags().StringVar(&webhooksEventsStatus, "status", "", "Filter by delivery status ('pending', 'delivered', 'failed')")
	webhooksEventCallbacksCmd.Flags().StringVar(&webhooksEventsTopic, "topic", "", "Filter by event topic (e.g. payments.captured)")
	webhooksEventCallbacksCmd.Flags().StringVar(&webhooksEventsPaymentID, "payment-id", "", "Filter by the payment the delivery is for")
	webhooksEventCallbacksCmd.Flags().StringVar(&webhooksEventsSince, "since", "", "Only deliveries at/after this ISO-8601 time")
	webhooksEventCallbacksCmd.Flags().StringVar(&webhooksEventsUntil, "until", "", "Only deliveries at/before this ISO-8601 time")
	webhooksEventCallbacksCmd.Flags().StringVar(&webhooksEventsSort, "sort", "", "Sort fields (e.g. -created_at)")
	webhooksCmd.AddCommand(webhooksEventCallbacksCmd)

	addYesFlag(webhooksDeleteCmd)
	webhooksCmd.AddCommand(webhooksDeleteCmd)
}
