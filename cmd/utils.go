package cmd

import (
	"context"

	"github.com/rail0/cli/internal/output"
	"github.com/spf13/cobra"
)

var utilsCmd = &cobra.Command{
	Use:   "utils",
	Short: "Contract introspection helpers",
}

var utilsDomainSeparatorCmd = &cobra.Command{
	Use:   "domain-separator",
	Short: "Print the EIP-712 domain separator of the RAIL0 contract",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := newClient().Utils.DomainSeparator(context.Background())
		if err != nil {
			return err
		}
		return output.Print(resp)
	},
}

var utilsVersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the RAIL0 contract version",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := newClient().Utils.Version(context.Background())
		if err != nil {
			return err
		}
		return output.Print(resp)
	},
}

func init() {
	utilsCmd.AddCommand(utilsDomainSeparatorCmd)
	utilsCmd.AddCommand(utilsVersionCmd)
}
