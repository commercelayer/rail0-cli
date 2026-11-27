package cmd

import (
	"sort"

	"github.com/spf13/cobra"
)

// The operator's commands: `rail0 admin …`, the gateway's cross-account /admin routes.
//
// They need a session whose account holds the operator grant; the gateway answers any
// other session 403 `forbidden`. They are for a PERSON at a terminal: agent mode refuses
// every one of them (operatorOnly), and never carries the write-mode token either
// (bearerToken), so an operator's session left on a machine an agent drives reads and
// writes nothing through this group.

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Operator commands: diagnostics, transactions, sync errors, webhooks, accounts, wallets, onboarding, audit, write mode",
	Long: `Commands for a gateway operator. They read and repair across every account, so they
need a session whose account holds the operator grant: 'rail0 auth login' with an
operator's key. Any other session is answered 403.

An operator session reads. To write (clear sync errors, switch an account or a wallet
off or on, invite a merchant, reset a webhook circuit, and the operator's writes elsewhere
in the CLI, such as 'account update --account …') switch write mode on with
'rail0 admin write-mode on': it lasts an hour, both switches are audited on the gateway,
and 'rail0 admin write-mode off' ends it early. 'rail0 admin audit' lists every operator
write.

Not available in agent mode: these commands are for a person at a terminal.`,
	Example: `  rail0 admin health                       # one verdict, then a check per thing that can be wrong
  rail0 admin health --rpc                 # every chain's RPC endpoints, probed from the gateway
  rail0 admin transactions --stuck any     # what is stuck, across every account
  rail0 admin trace 0xrail0Id…             # one payment: transactions, sync errors, deliveries, disputes
  rail0 admin sync-errors --reason not_found
  rail0 admin accounts --search 0x2D2B…    # whose wallet is this
  rail0 admin onboardings                  # invites and sign-ups still waiting
  rail0 admin webhooks --circuit open      # subscriptions that are receiving nothing
  rail0 admin audit --target 019e748b-…    # everything operators did to one account`,
}

// operatorCommandsReason is why agent mode refuses the admin group, said the same way
// by every command and in the help data.
const operatorCommandsReason = "operator commands are for a person at a terminal"

// operatorOnly wraps the RunE of an admin command: it refuses in agent mode before
// anything is read or sent, and otherwise runs the command.
func operatorOnly(run func(cmd *cobra.Command, args []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := refuseInAgentMode(cmd, operatorCommandsReason); err != nil {
			return err
		}
		return run(cmd, args)
	}
}

// operatorCommands returns every runnable command under `rail0 admin`, by path.
func operatorCommands() []*cobra.Command {
	var out []*cobra.Command
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if c.Runnable() {
			out = append(out, c)
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(adminCmd)
	sort.Slice(out, func(i, j int) bool { return out[i].CommandPath() < out[j].CommandPath() })
	return out
}

func init() {
	// The whole tree is assembled here, not in each file's own init: the walk below reads
	// it, and Go runs this file's init before the other admin_*.go files'.
	adminWriteModeCmd.AddCommand(adminWriteModeOnCmd, adminWriteModeOffCmd, adminWriteModeStatusCmd)
	adminSyncErrorsCmd.AddCommand(adminSyncErrorsClearCmd)
	adminAccountsCmd.AddCommand(adminAccountsGetCmd, adminAccountsExposureCmd,
		adminAccountsDeactivateCmd, adminAccountsActivateCmd)
	adminWalletsCmd.AddCommand(adminWalletsUsageCmd, adminWalletsDeactivateCmd, adminWalletsActivateCmd)
	adminOnboardingsCmd.AddCommand(adminOnboardingsInviteCmd, adminOnboardingsCorrectCmd,
		adminOnboardingsResendCmd, adminOnboardingsCancelCmd)
	adminWebhooksCmd.AddCommand(adminWebhooksResetCircuitCmd)
	adminCmd.AddCommand(adminHealthCmd, adminTransactionsCmd, adminSyncErrorsCmd, adminTraceCmd,
		adminWebhooksCmd, adminDeliveriesCmd, adminAccountsCmd, adminWalletsCmd, adminOnboardingsCmd,
		adminAuditCmd, adminWriteModeCmd)
	rootCmd.AddCommand(adminCmd)

	// The help data lists what agent mode refuses (agentUnavailable); the admin group is
	// listed from the tree, so a command added here cannot be forgotten there.
	for _, c := range operatorCommands() {
		agentUnavailable[c.CommandPath()] = operatorCommandsReason
	}
}
