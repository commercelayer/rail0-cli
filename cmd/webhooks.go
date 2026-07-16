package cmd

import (
	"context"
	"errors"

	"github.com/rail0/cli/internal/ui"
	rail0 "github.com/rail0/go-sdk"
	"github.com/spf13/cobra"
)

var webhooksCmd = &cobra.Command{
	Use:   "webhooks",
	Short: "Webhook subscription management (requires login)",
	Long: `Manage webhook subscriptions for the authenticated account.

A webhook subscribes to exactly one payment-lifecycle topic (e.g.
payments.captured) and delivers a signed callback to your endpoint. The
shared secret used to verify delivery signatures is returned only once, on
create and rotate-secret.`,
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
			resp, err := newClient().Webhooks.List(context.Background(), p)
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
			return newClient().Webhooks.Get(context.Background(), args[0])
		})
	},
}

// ── webhooks create ──────────────────────────────────────────────────────────────

var webhooksCreateName string
var webhooksCreateURL string
var webhooksCreateTopic string

var webhooksCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Register a new webhook (returns the one-time shared secret)",
	Example: `  rail0 webhooks create --name "capture hook" \
    --callback-url https://example.com/hook --topic payments.captured`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := rail0.CreateWebhookRequest{
			Name:        webhooksCreateName,
			CallbackURL: webhooksCreateURL,
			Topic:       webhooksCreateTopic,
		}
		return ui.RunAPI(func() (any, error) {
			return newClient().Webhooks.Create(context.Background(), req)
		})
	},
}

// ── webhooks update ──────────────────────────────────────────────────────────────

var webhooksUpdateName string
var webhooksUpdateURL string
var webhooksUpdateTopic string

var webhooksUpdateCmd = &cobra.Command{
	Use:     "update <webhook_id>",
	Short:   "Update a webhook's name, callback URL, or topic",
	Example: `  rail0 webhooks update whk_abc123 --callback-url https://example.com/new`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := rail0.UpdateWebhookRequest{
			Name:        webhooksUpdateName,
			CallbackURL: webhooksUpdateURL,
			Topic:       webhooksUpdateTopic,
		}
		return ui.RunAPI(func() (any, error) {
			return newClient().Webhooks.Update(context.Background(), args[0], req)
		})
	},
}

// ── webhooks enable / disable / rotate-secret / reset-circuit ─────────────────────

var webhooksEnableCmd = &cobra.Command{
	Use:     "enable <webhook_id>",
	Short:   "Re-enable a disabled webhook",
	Example: `  rail0 webhooks enable whk_abc123`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newClient().Webhooks.Enable(context.Background(), args[0])
		})
	},
}

var webhooksDisableCmd = &cobra.Command{
	Use:     "disable <webhook_id>",
	Short:   "Disable a webhook",
	Example: `  rail0 webhooks disable whk_abc123`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newClient().Webhooks.Disable(context.Background(), args[0])
		})
	},
}

var webhooksRotateSecretCmd = &cobra.Command{
	Use:     "rotate-secret <webhook_id>",
	Short:   "Generate a new shared secret (returned once on the response)",
	Example: `  rail0 webhooks rotate-secret whk_abc123`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newClient().Webhooks.RotateSecret(context.Background(), args[0])
		})
	},
}

var webhooksResetCircuitCmd = &cobra.Command{
	Use:     "reset-circuit <webhook_id>",
	Short:   "Reset the delivery circuit breaker and re-enable the webhook",
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
			resp, err := newClient().Webhooks.EventCallbacks(context.Background(), args[0], p)
			if err != nil {
				return nil, err
			}
			return resp.Data, nil // bare array; pagination is in headers
		})
	},
}

// ── webhooks delete ──────────────────────────────────────────────────────────────

var webhooksDeleteCmd = &cobra.Command{
	Use:     "delete <webhook_id>",
	Short:   "Remove a webhook",
	Example: `  rail0 webhooks delete whk_abc123`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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
	webhooksListCmd.Flags().StringVar(&webhooksListTopic, "topic", "", "Filter by topic (e.g. payments.captured)")
	webhooksListCmd.Flags().StringVar(&webhooksListActive, "active", "", "Filter by active flag ('true' or 'false')")
	webhooksListCmd.Flags().StringVar(&webhooksListCircuit, "circuit-state", "", "Filter by circuit state ('closed' or 'open')")
	webhooksListCmd.Flags().StringVar(&webhooksListSort, "sort", "", "Sort fields (e.g. -created_at)")
	webhooksListCmd.Flags().IntVar(&webhooksListPage, "page", 0, "Page number (1-based)")
	webhooksListCmd.Flags().IntVar(&webhooksListPerPage, "per-page", 0, "Results per page")
	webhooksCmd.AddCommand(webhooksListCmd)

	webhooksCmd.AddCommand(webhooksGetCmd)

	webhooksCreateCmd.Flags().StringVarP(&webhooksCreateName, "name", "n", "", "Human-readable webhook name")
	webhooksCreateCmd.Flags().StringVarP(&webhooksCreateURL, "callback-url", "u", "", "HTTPS endpoint that receives deliveries")
	webhooksCreateCmd.Flags().StringVarP(&webhooksCreateTopic, "topic", "t", "", "Topic to subscribe to (e.g. payments.captured)")
	_ = webhooksCreateCmd.MarkFlagRequired("name")
	_ = webhooksCreateCmd.MarkFlagRequired("callback-url")
	_ = webhooksCreateCmd.MarkFlagRequired("topic")
	webhooksCmd.AddCommand(webhooksCreateCmd)

	webhooksUpdateCmd.Flags().StringVarP(&webhooksUpdateName, "name", "n", "", "New webhook name")
	webhooksUpdateCmd.Flags().StringVarP(&webhooksUpdateURL, "callback-url", "u", "", "New callback URL")
	webhooksUpdateCmd.Flags().StringVarP(&webhooksUpdateTopic, "topic", "t", "", "New topic")
	webhooksCmd.AddCommand(webhooksUpdateCmd)

	webhooksCmd.AddCommand(webhooksEnableCmd)
	webhooksCmd.AddCommand(webhooksDisableCmd)
	webhooksCmd.AddCommand(webhooksRotateSecretCmd)
	webhooksCmd.AddCommand(webhooksResetCircuitCmd)

	webhooksEventCallbacksCmd.Flags().StringVar(&webhooksEventsStatus, "status", "", "Filter by delivery status ('pending', 'delivered', 'failed')")
	webhooksEventCallbacksCmd.Flags().StringVar(&webhooksEventsTopic, "topic", "", "Filter by event topic (e.g. payments.captured)")
	webhooksEventCallbacksCmd.Flags().StringVar(&webhooksEventsPaymentID, "payment-id", "", "Filter by the payment the delivery is for")
	webhooksEventCallbacksCmd.Flags().StringVar(&webhooksEventsSince, "since", "", "Only deliveries at/after this ISO-8601 time")
	webhooksEventCallbacksCmd.Flags().StringVar(&webhooksEventsUntil, "until", "", "Only deliveries at/before this ISO-8601 time")
	webhooksEventCallbacksCmd.Flags().StringVar(&webhooksEventsSort, "sort", "", "Sort fields (e.g. -created_at)")
	webhooksCmd.AddCommand(webhooksEventCallbacksCmd)

	webhooksCmd.AddCommand(webhooksDeleteCmd)
}
