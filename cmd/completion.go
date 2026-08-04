package cmd

import (
	"sort"

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

// fixedCompletions builds a flag completion function for a finite value set — the
// enums the gateway accepts (--status, --mode, --by, --interval, and the environment
// names). Cheap and local, and a typo on one of these otherwise costs a round-trip to
// discover. (#110)
func fixedCompletions(values ...string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return values, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeAliasNames suggests the address-book entries with the "@" prefix the
// address flags accept, so `--to @<TAB>` completes to the saved names. Local-only,
// like completeKeyNames.
func completeAliasNames(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	cfg, err := config.Load()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names := make([]string, 0, len(cfg.Aliases))
	for name := range cfg.Aliases {
		names = append(names, "@"+name)
	}
	sort.Strings(names)
	return names, cobra.ShellCompDirectiveNoFileComp
}
