package cmd

import (
	"context"

	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

var accountsCmd = &cobra.Command{
	Use:   "accounts",
	Short: "Account configuration queries",
}

var accountsPaymentMethodsCmd = &cobra.Command{
	Use:     "payment-methods <accountId>",
	Short:   "List the accepted payment methods for an account",
	Example: `  rail0 accounts payment-methods 019e6f9e-46f8-7174-8919-802e0953ae42`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newClient().Accounts.PaymentMethods(context.Background(), args[0])
		})
	},
}

func init() {
	accountsCmd.AddCommand(accountsPaymentMethodsCmd)
}
