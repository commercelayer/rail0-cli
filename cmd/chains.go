package cmd

import (
	"context"

	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

var chainsCmd = &cobra.Command{
	Use:     "chains",
	Short:   "List supported blockchains (chain IDs used in --chain-id)",
	Example: `  rail0 chains`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newClient().Chains.List(context.Background())
		})
	},
}

func init() {}
