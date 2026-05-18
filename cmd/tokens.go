package cmd

import (
	"context"

	"github.com/rail0/cli/internal/output"
	"github.com/spf13/cobra"
)

var tokensCmd = &cobra.Command{
	Use:   "tokens",
	Short: "Token allowlist queries",
}

var tokensCheckCmd = &cobra.Command{
	Use:   "check <address>",
	Short: "Check whether a token is in the RAIL0 allowlist",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := newClient().Tokens.IsAccepted(context.Background(), args[0])
		if err != nil {
			return err
		}
		return output.Print(resp)
	},
}

func init() {
	tokensCmd.AddCommand(tokensCheckCmd)
}
