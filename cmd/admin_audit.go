package cmd

import (
	"context"
	"regexp"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// The operator audit log: every write an operator made, who made it, on whose account
// and with which detail. The gateway records the row with the write itself. Read-only:
// nothing here edits or clears it.

var (
	auditAction   string
	auditActor    string
	auditTarget   string
	auditResource string
	auditPayment  string
	auditFrom     string
	auditTo       string
	auditSort     string
	auditPage     int
	auditPerPage  int
)

// auditActions are the actions the gateway records today. They back shell completion
// only: the gateway validates --action, and refusing one here would make a CLI older than
// the gateway unable to filter by an action added since.
var auditActions = []string{
	"account.create", "account.update", "account.activate", "account.deactivate",
	"onboarding.create", "onboarding.update", "onboarding.send", "onboarding.delete",
	"wallet.add", "wallet.update", "wallet.deactivate", "wallet.reactivate",
	"administrator.grant", "administrator.revoke",
	"webhook.reset_circuit", "transaction.redrive", "catalog.reload", "sync_errors.clear",
	"write_mode.enable", "write_mode.disable",
}

// uuidPattern is a UUID, the only form the audit log knows a payment by.
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

var adminAuditCmd = &cobra.Command{
	Use:   "audit",
	Short: "The operator audit log: every operator write, who did it and to whom",
	Long: `List the operator audit log, newest first: one row per write an operator made, with
the operator (actor), the account it landed on (target), the resource, and the detail the
gateway recorded with it (the fields changed and the values they replaced, the exposure an
operator acknowledged, the filters of a clear, …).

What is recorded: accounts (update, activate, deactivate), onboarding requests (create,
update, send, delete), wallets (update, deactivate, reactivate), the operator role
(grant, revoke), a webhook circuit reset, a transaction redrive, a catalogue reload, a
sync-error clear, and write mode switched on and off. A merchant's own writes are not
operator actions and are not here.

--actor and --target take account ids ('rail0 admin accounts --search …' finds them).
--payment takes the payment's UUID, not its rail0_id: 'rail0 admin trace 0x…' shows a
payment's operator actions from either.`,
	Example: `  rail0 admin audit                                   # the latest operator writes
  rail0 admin audit --target 019e748b-…               # everything done to one account
  rail0 admin audit --actor 019f8a3d-… --from 2026-11-26T00:00:00Z
  rail0 admin audit --action account.deactivate --pretty
  rail0 admin audit --resource-type wallet`,
	Args: cobra.NoArgs,
	RunE: operatorOnly(func(_ *cobra.Command, _ []string) error {
		if auditPayment != "" && !uuidPattern.MatchString(auditPayment) {
			return usagef("--payment %q is not a payment UUID: the audit log knows a payment by its UUID only "+
				"('rail0 admin trace <rail0_id>' shows a payment's operator actions from either id)", auditPayment)
		}
		return ui.RunAPI(func() (any, error) {
			resp, err := newReadClient().Admin.Actions(context.Background(), rail0.ListAdminActionsParams{
				Action: auditAction, ActorAccountID: auditActor, TargetAccountID: auditTarget,
				ResourceType: auditResource, PaymentID: auditPayment, From: auditFrom, To: auditTo,
				Sort: auditSort, Page: auditPage, PerPage: auditPerPage,
			})
			if err != nil {
				return nil, err
			}
			return paged(table(resp.Data, auditColumns), resp.Meta), nil
		})
	}),
}

func init() {
	f := adminAuditCmd.Flags()
	f.StringVar(&auditAction, "action", "", "Only this kind of action, e.g. account.deactivate, wallet.deactivate, sync_errors.clear")
	f.StringVar(&auditActor, "actor", "", "Only actions by this operator (account id)")
	f.StringVar(&auditTarget, "target", "", "Only actions that landed on this account (account id)")
	f.StringVar(&auditResource, "resource-type", "", "Only actions on this kind of resource, e.g. account, wallet, onboarding, webhook")
	f.StringVar(&auditPayment, "payment", "", "Only actions whose detail names this payment (its UUID): a redrive")
	f.StringVar(&auditFrom, "from", "", "Only actions at/after this ISO-8601 time")
	f.StringVar(&auditTo, "to", "", "Only actions at/before this ISO-8601 time")
	f.StringVar(&auditSort, "sort", "", "Sort by created_at or action (prefix - for descending; default: newest first)")
	f.IntVar(&auditPage, "page", 0, "Page number (1-based)")
	f.IntVar(&auditPerPage, "per-page", 0, "Results per page")
	_ = adminAuditCmd.RegisterFlagCompletionFunc("action", fixedCompletions(auditActions...))
}
