package cmd

import (
	"context"

	rail0 "github.com/rail0/go-sdk"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

var accountsCmd = &cobra.Command{
	Use:   "accounts",
	Short: "Account configuration queries",
}

var pmFilters rail0.PaymentMethodsFilters

var accountsPaymentMethodsCmd = &cobra.Command{
	Use:   "payment-methods <accountId>",
	Short: "List the accepted payment methods for an account",
	Example: `  rail0 accounts payment-methods 019e748b-da9a-7c3f-ba32-50572ffd5388
  rail0 accounts payment-methods 019e748b-da9a-7c3f-ba32-50572ffd5388 --stablecoin-symbol USDC
  rail0 accounts payment-methods 019e748b-da9a-7c3f-ba32-50572ffd5388 --blockchain-slug polygon`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newClient().Accounts.PaymentMethods(context.Background(), args[0], pmFilters)
		})
	},
}

func init() {
	accountsPaymentMethodsCmd.Flags().StringVar(&pmFilters.StablecoinID, "stablecoin-id", "", "Filter by stablecoin id")
	accountsPaymentMethodsCmd.Flags().StringVar(&pmFilters.StablecoinSymbol, "stablecoin-symbol", "", "Filter by stablecoin symbol (case-insensitive)")
	accountsPaymentMethodsCmd.Flags().StringVar(&pmFilters.BlockchainID, "blockchain-id", "", "Filter by blockchain id")
	accountsPaymentMethodsCmd.Flags().StringVar(&pmFilters.BlockchainSlug, "blockchain-slug", "", "Filter by blockchain slug (case-insensitive)")
	accountsCmd.AddCommand(accountsPaymentMethodsCmd)
}
