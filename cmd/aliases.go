package cmd

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/rail0/cli/internal/config"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// evmAddressPattern matches a 0x-prefixed 20-byte (40 hex) EVM address. Aliases
// must point at a well-formed address so a saved "@name" always resolves to a
// value the gateway will accept.
var evmAddressPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

// privateKeyPattern matches a 32-byte secp256k1 private key (64 hex, 0x
// optional) — twice an address's length. It's checked first so pasting a key
// into `aliases add` is caught with a clear, secret-safe message (the value is
// never echoed) instead of the generic "invalid address" error.
var privateKeyPattern = regexp.MustCompile(`^(?:0x)?[0-9a-fA-F]{64}$`)

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
  rail0 aliases rm payee
  rail0 wallets alias 0xAbC123… payee`,
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
	Short:   "Add an address alias (names must be unique — rm to replace)",
	Example: `  rail0 aliases add payee 0xAbC123…`,
	Args:    cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		name, addr := args[0], args[1]
		if err := addAlias(name, addr); err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return map[string]any{"status": "saved", "alias": name, "address": addr}, nil
		})
	},
}

// addAlias stores name → addr in the address book after validating both. Alias
// names are unique: an existing name is rejected (the caller must `aliases rm`
// it first) rather than silently overwritten, so a name always points at
// exactly one address. The "@" sigil is reserved for *referencing* an alias on
// an address flag, so it must never be part of a stored name. The address must
// be a well-formed 0x EVM address, so a saved "@name" always resolves to a
// value the gateway accepts. Shared by `aliases add` and `wallets alias` (which
// only differ in argument order).
func addAlias(name, addr string) error {
	if strings.HasPrefix(name, "@") {
		return fmt.Errorf("alias name must not include the leading '@'")
	}
	// Catch a pasted private key before the address check — and never echo the
	// value (it may be a secret). Aliases hold public addresses only.
	if privateKeyPattern.MatchString(addr) {
		return errors.New("that looks like a private key — aliases store public 0x addresses, not secrets. Save a signing key with 'rail0 keys add' instead")
	}
	if !evmAddressPattern.MatchString(addr) {
		return fmt.Errorf("invalid address %q: expected a 0x-prefixed 40-hex-character EVM address", addr)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if _, exists := cfg.Aliases[name]; exists {
		return fmt.Errorf("alias %q already exists; remove it first with 'rail0 aliases rm %s'", name, name)
	}
	if cfg.Aliases == nil {
		cfg.Aliases = map[string]string{}
	}
	cfg.Aliases[name] = addr
	return cfg.Save()
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
