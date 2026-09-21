package cmd

import (
	"context"

	rail0 "github.com/commercelayer/rail0-go"

	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	tokensChainID int
	tokensSymbol  string
	tokensActive  bool
)

var tokensCmd = &cobra.Command{
	Use:   "tokens",
	Short: "List supported tokens (symbols used in --token)",
	Long: `List the token catalog.

Includes RETIRED tokens by default (an existing payment references its token
forever, so they stay listed and resolvable); each row carries an "active" flag.
Only an active token can be used for a NEW payment — pass --active to list just
those, or --active=false to see only the retired ones.`,
	Example: `  rail0 tokens
  rail0 tokens --chain-id 5042002
  rail0 tokens --symbol USDC
  rail0 tokens --active           # only what a new payment can use`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Tri-state: the filter is applied only when --active was actually
		// passed, so an untouched flag means "no filter" rather than false.
		params := rail0.ListTokensParams{ChainID: tokensChainID, Symbol: tokensSymbol}
		if cmd.Flags().Changed("active") {
			params.Active = &tokensActive
		}
		return ui.RunAPI(func() (any, error) {
			return newReadClient().Tokens.List(context.Background(), params)
		})
	},
}

func init() {
	tokensCmd.Flags().IntVarP(&tokensChainID, "chain-id", "c", 0,
		"Filter by EVM chain ID (e.g. 5042002)")
	tokensCmd.Flags().StringVarP(&tokensSymbol, "symbol", "s", "",
		"Filter by token symbol (case-insensitive, e.g. USDC)")
	tokensCmd.Flags().BoolVar(&tokensActive, "active", false,
		"Filter by active flag (--active, or --active=false for retired only); omit for every token")
}
