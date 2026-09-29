package cmd

import (
	"slices"
	"sort"
	"strings"

	"github.com/rail0/cli/internal/config"
	"github.com/spf13/cobra"
)

// completeKeyNames is a cobra ValidArgsFunction that suggests the saved signing-key
// names (from the local config index) for commands that take a <name> argument —
// so `rail0 keys rm <TAB>` completes to the real key names. Local-only: no network
// or auth, so tab-completion stays instant.
// completeKeyNames suggests saved key names at the TAB key. It is the ONLY
// consumer of config.Keys, and deliberately the only kind of consumer that may
// exist: the list is a copy of what the keychain holds, and a keychain cannot
// be enumerated to reconcile it. A missing suggestion therefore means a stale
// index, never a missing key — which is tolerable for completion and would not
// be for anything a user reads as an answer. (#139)
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

// fixedListCompletions is fixedCompletions for a flag that takes SEVERAL values —
// repeated, or comma-separated in one argument (a StringSlice). After a comma it
// completes the next value behind what is already typed ("authorized,<TAB>" offers
// "authorized,captured", …), leaving out the values already listed, so the
// comma-separated form completes as well as the repeated one.
func fixedListCompletions(values ...string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		cut := strings.LastIndex(toComplete, ",") + 1
		prefix, chosen := toComplete[:cut], strings.Split(toComplete[:cut], ",")
		out := make([]string, 0, len(values))
		for _, v := range values {
			if !slices.Contains(chosen, v) {
				out = append(out, prefix+v)
			}
		}
		return out, cobra.ShellCompDirectiveNoFileComp
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
