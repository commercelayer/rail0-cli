package cmd

import (
	"context"
	"errors"

	"github.com/rail0/cli/internal/ui"
	rail0 "github.com/rail0/go-sdk"
	"github.com/spf13/cobra"
)

// paymentMethodsCmd is the buyer-facing, public discovery of a merchant's
// accepted payment methods. Unlike `wallets` (which is behind SIWE and lists
// the logged-in account's OWN wallets), this needs no session: a payer that
// only knows the merchant — by account id, or by one of the merchant's wallet
// addresses — lists the active wallet/token combinations it accepts. It maps
// the gateway's public GET /payment_methods and requires exactly one of
// --account or --address, mirroring the endpoint's own rule.

var (
	paymentMethodsAccount string
	paymentMethodsAddress string
)

var paymentMethodsCmd = &cobra.Command{
	Use:   "payment-methods",
	Short: "Discover a merchant's accepted wallets/tokens (public, no login)",
	Long: `List a merchant's active payment methods: its active wallets, each with the
active tokens (and chains) it accepts.

Public — no login required. Identify the merchant with exactly one of --account
(all its active wallets) or --address (just that one wallet; accepts an @alias).`,
	Example: `  rail0 payment-methods --account 019e748b-…
  rail0 payment-methods --address 0xABCD…
  rail0 payment-methods --address @payee`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		// Exactly one handle, matching the gateway's exactly_one_of rule — fail
		// fast with a clear message instead of a 400 round-trip.
		if (paymentMethodsAccount == "") == (paymentMethodsAddress == "") {
			return errors.New("provide exactly one of --account <id> or --address <0x…|@alias>")
		}
		addr, err := resolveAlias(paymentMethodsAddress)
		if err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return newClient().PaymentMethods.List(context.Background(), rail0.PaymentMethodsQuery{
				AccountID: paymentMethodsAccount,
				Address:   addr,
			})
		})
	},
}

func init() {
	paymentMethodsCmd.Flags().StringVar(&paymentMethodsAccount, "account", "", "Merchant account id — list all its active wallets")
	paymentMethodsCmd.Flags().StringVarP(&paymentMethodsAddress, "address", "A", "", "A merchant wallet address (0x… or @alias) — list just that wallet")
}
