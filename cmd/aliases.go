package cmd

import (
	"fmt"
	"strings"

	"github.com/rail0/cli/internal/config"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// aliasesCmd lists the address book when run bare, and holds the add/rm verbs.
var aliasesCmd = &cobra.Command{
	Use:   "aliases",
	Short: "Address book — list saved addresses, or manage them with add/rm",
	Long: `Manage a global address book stored in the config file.

Run bare to list the saved aliases. Once an alias is set, reference it as
"@name" on any address flag (--from / --to / --address) instead of pasting a
raw 0x address.`,
	Example: `  rail0 aliases
  rail0 aliases add payee 0xAbC123…
  rail0 aliases rm payee`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		rows := make([]map[string]any, 0, len(cfg.Aliases))
		for name, addr := range cfg.Aliases {
			rows = append(rows, map[string]any{"alias": name, "address": addr})
		}
		return ui.RunAPI(func() (any, error) { return rows, nil })
	},
}

var aliasesAddCmd = &cobra.Command{
	Use:     "add <name> <address>",
	Short:   "Add or update an address alias",
	Example: `  rail0 aliases add payee 0xAbC123…`,
	Args:    cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		name, addr := args[0], args[1]
		if strings.HasPrefix(name, "@") {
			return fmt.Errorf("alias name must not include the leading '@'")
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if cfg.Aliases == nil {
			cfg.Aliases = map[string]string{}
		}
		cfg.Aliases[name] = addr
		if err := cfg.Save(); err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return map[string]any{"status": "saved", "alias": name, "address": addr}, nil
		})
	},
}

var aliasesRemoveCmd = &cobra.Command{
	Use:     "rm <name>",
	Short:   "Remove an address alias",
	Example: `  rail0 aliases rm payee`,
	Args:    cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if _, ok := cfg.Aliases[args[0]]; !ok {
			return fmt.Errorf("unknown alias %q", args[0])
		}
		delete(cfg.Aliases, args[0])
		if err := cfg.Save(); err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return map[string]any{"status": "removed", "alias": args[0]}, nil
		})
	},
}

// resolveAlias maps "@name" to its address from the config address book. Any
// other value (a literal 0x address, or an empty string) passes through
// unchanged, so callers can resolve every address flag uniformly.
func resolveAlias(v string) (string, error) {
	if !strings.HasPrefix(v, "@") {
		return v, nil
	}
	name := strings.TrimPrefix(v, "@")
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	addr, ok := cfg.Aliases[name]
	if !ok {
		return "", fmt.Errorf("unknown alias %q — add it with 'rail0 aliases add %s <address>'", v, name)
	}
	return addr, nil
}

func init() {
	aliasesCmd.AddCommand(aliasesAddCmd, aliasesRemoveCmd)
}
