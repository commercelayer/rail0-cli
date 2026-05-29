package cmd

import (
	"context"

	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

var tokensChain string

var tokensCmd = &cobra.Command{
	Use:   "tokens",
	Short: "List supported tokens (symbols used in --token)",
	Example: `  rail0 tokens
  rail0 tokens --chain arc-testnet`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			chainID := 0
			if tokensChain != "" {
				chains, err := newClient().Chains.List(context.Background())
				if err != nil {
					return nil, err
				}
				for _, b := range chains {
					if b.Slug == tokensChain {
						chainID = b.ChainID
						break
					}
				}
			}
			return newClient().Tokens.List(context.Background(), chainID)
		})
	},
}

func init() {
	tokensCmd.Flags().StringVarP(&tokensChain, "chain", "c", "",
		"Filter by chain slug (e.g. arc-testnet)")
}
