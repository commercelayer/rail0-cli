package cmd

import (
	"context"
	"fmt"
	"strings"

	rail0 "github.com/rail0/go-sdk"

	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	chainsSymbol      string
	chainsNetworkType string
)

var chainsCmd = &cobra.Command{
	Use:     "chains",
	Aliases: []string{"blockchains"},
	Short:   "List supported blockchains (chain IDs used in --chain-id)",
	Long: `List the blockchains supported by the environment.

Filter with --symbol (the chain's native symbol, e.g. ETH / USDC / CELO) and/or
--network-type (testnet | mainnet). Both match case-insensitively; combine them
to narrow further.`,
	Example: `  rail0 chains
  rail0 chains --symbol USDC
  rail0 chains --network-type testnet
  rail0 chains -s ETH -n mainnet`,
	RunE: func(cmd *cobra.Command, args []string) error {
		networkType := strings.ToLower(chainsNetworkType)
		if networkType != "" && networkType != "testnet" && networkType != "mainnet" {
			return fmt.Errorf("invalid --network-type %q: must be 'testnet' or 'mainnet'", chainsNetworkType)
		}
		return ui.RunAPI(func() (any, error) {
			return newClient().Chains.List(context.Background(),
				rail0.ListChainsParams{NetworkType: networkType, Symbol: chainsSymbol})
		})
	},
}

func init() {
	chainsCmd.Flags().StringVarP(&chainsSymbol, "symbol", "s", "",
		"Filter by native symbol (e.g. ETH, USDC, CELO)")
	chainsCmd.Flags().StringVarP(&chainsNetworkType, "network-type", "n", "",
		"Filter by network type: testnet or mainnet")
}
