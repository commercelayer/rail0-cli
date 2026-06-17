package cmd

import (
	"context"

	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

var tokensChainID int

var tokensCmd = &cobra.Command{
	Use:   "tokens",
	Short: "List supported tokens (symbols used in --token)",
	Example: `  rail0 tokens
  rail0 tokens --chain-id 5042002`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newClient().Tokens.List(context.Background(), tokensChainID)
		})
	},
}

func init() {
	tokensCmd.Flags().IntVarP(&tokensChainID, "chain-id", "c", 0,
		"Filter by EVM chain ID (e.g. 5042002)")
}
