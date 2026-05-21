package cmd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

var merchantsCmd = &cobra.Command{
	Use:   "merchants",
	Short: "Merchant configuration queries",
	Long: `Query merchant configuration from the RAIL0 API.

Merchants expose one or more payment methods — each method is a
(chain, token, wallet) combination that a buyer can use to pay. Use
'merchants payment-methods' to discover the correct payee address and
token address to pass to 'payments create'.`,
}

var merchantsPaymentMethodsCmd = &cobra.Command{
	Use:   "payment-methods <merchantId>",
	Short: "List the accepted payment methods for a merchant",
	Long: `Fetch the active payment methods for a merchant.

Each payment method represents one (chain, token, wallet) combination
the merchant accepts. The returned fields map directly to the flags
required by 'payments create':

  walletAddress  →  --payee
  tokenAddress   →  --token
  chainId        →  --chain-id

The isDefault field marks the merchant's preferred payment method.`,
	Example: `  rail0 merchants payment-methods 1
  rail0 merchants payment-methods 42 --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("merchantId must be an integer: %w", err)
		}
		return ui.RunAPI(func() (any, error) {
			return newClient().Merchants.PaymentMethods(context.Background(), id)
		})
	},
}

func init() {
	merchantsCmd.AddCommand(merchantsPaymentMethodsCmd)
}
