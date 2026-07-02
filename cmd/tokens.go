package cmd

import (
	"context"

	rail0 "github.com/rail0/go-sdk"

	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	tokensChainID int
	tokensSymbol  string
)

var tokensCmd = &cobra.Command{
	Use:   "tokens",
	Short: "List supported tokens (symbols used in --token)",
	Example: `  rail0 tokens
  rail0 tokens --chain-id 5042002
  rail0 tokens --symbol USDC`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newClient().Tokens.List(context.Background(),
				rail0.ListTokensParams{ChainID: tokensChainID, Symbol: tokensSymbol})
		})
	},
}

func init() {
	tokensCmd.Flags().IntVarP(&tokensChainID, "chain-id", "c", 0,
		"Filter by EVM chain ID (e.g. 5042002)")
	tokensCmd.Flags().StringVarP(&tokensSymbol, "symbol", "s", "",
		"Filter by token symbol (case-insensitive, e.g. USDC)")
}
