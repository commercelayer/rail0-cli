package cmd

import (
	"github.com/rail0/cli/internal/config"
	"github.com/spf13/cobra"
)

// completeKeyNames is a cobra ValidArgsFunction that suggests the saved signing-key
// names (from the local config index) for commands that take a <name> argument —
// so `rail0 keys rm <TAB>` completes to the real key names. Local-only: no network
// or auth, so tab-completion stays instant.
func completeKeyNames(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return cfg.Keys, cobra.ShellCompDirectiveNoFileComp
}
