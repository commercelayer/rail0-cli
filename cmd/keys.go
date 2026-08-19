package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/term"
	"github.com/rail0/cli/internal/config"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
)

// keyringService is the service name under which signing keys are stored in the
// OS keychain; each key is one entry keyed by its name.
const keyringService = "rail0-cli"

// keysCmd lists the saved key names when run bare, and holds the add/rm/address
// verbs. Secrets live in the OS keychain (macOS Keychain, Windows Credential
// Manager, Linux Secret Service); only the names are tracked in the config so
// they can be listed and referenced as "@name" on the --private-key flag.
var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage signing keys in the OS keychain (lists names when run bare)",
	Long: `Save signing keys in the OS keychain and reference them as "@name" on the
--private-key flag (e.g. 'rail0 payments create -p @payer'), instead of pasting
a raw key or exporting RAIL0_PRIVATE_KEY.

The secret never touches the config file or your shell history — only the key
names are tracked locally so they can be listed. Backends: macOS Keychain,
Windows Credential Manager, Linux Secret Service. On a system without one (e.g.
headless Linux) these commands return an error; use --private-key /
RAIL0_PRIVATE_KEY there.`,
	Example: `  rail0 keys add payer
  rail0 payments create -p @payer …
  rail0 keys
  rail0 keys rm payer`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		rows := make([]map[string]any, 0, len(cfg.Keys))
		for _, name := range cfg.Keys {
			rows = append(rows, map[string]any{"key": name})
		}
		return ui.RunAPI(func() (any, error) { return rows, nil })
	},
}

var keysAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Save a signing key in the OS keychain (prompts for the key; names must be unique)",
	Long: `Save a signing key in the OS keychain under a name, then use it as @name on
--private-key anywhere a key is taken.

The secret is PROMPTED (hidden), never taken as an argument, so it stays out of
your shell history and out of the process list. It goes to the keychain; only
the NAME is written to the config file, so the config can be copied between
machines without carrying a secret.

Because names live in the config and secrets live in the keychain, the two can
disagree — delete the config, or move it between machines, and the keychain may
still hold an entry the name index no longer lists. This command REFUSES to
write over such an orphan rather than asking yes/no: a private key cannot be
recovered, and a confirmation in that position is one people answer by reflex.
'keys rm <name>' clears it, and is the only way to, since a keychain cannot be
enumerated — which is exactly why the name index exists.`,
	Example: `  rail0 keys add payer`,
	Args:    cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		name := args[0]
		if strings.HasPrefix(name, "@") {
			return errors.New("key name must not include the leading '@'")
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if slices.Contains(cfg.Keys, name) {
			return fmt.Errorf("key %q already exists; remove it first with 'rail0 keys rm %s'", name, name)
		}
		// The index is not the only place a key can live. Ask the KEYCHAIN too,
		// because the two can disagree: delete config.yaml to "start clean", or copy
		// a config between machines, and the index no longer lists an entry the
		// keychain still holds. The write below is an unconditional Set, so without
		// this probe `keys add payer` would overwrite that orphaned secret with no
		// warning and no way back — and if it held funds, they are gone. A key is the
		// one thing in this tool that cannot be re-derived, so it refuses rather than
		// prompting: an interactive "overwrite? [y/N]" is exactly the prompt someone
		// answers by reflex. (#70)
		if orphaned, err := keyring.Get(keyringService, name); err == nil && orphaned != "" {
			return fmt.Errorf(
				"the OS keychain already holds a key named %q, even though it is not in the local index "+
					"(a deleted or copied config.yaml does this).\n"+
					"Refusing to overwrite it — a private key cannot be recovered.\n"+
					"Inspect it with 'rail0 keys address %s', then either choose another name or "+
					"delete it deliberately with 'rail0 keys rm %s --yes'",
				name, name, name,
			)
		}

		// Read the secret without it ever being a CLI argument (no shell history).
		hexKey, err := promptSecret("Private key (0x-prefixed hex, hidden)")
		if err != nil {
			return err
		}
		hexKey = strings.TrimSpace(hexKey)
		addr, err := addressFromKey(hexKey) // validates the secp256k1 key
		if err != nil {
			return err
		}
		if err := keyring.Set(keyringService, name, hexKey); err != nil {
			return fmt.Errorf("could not save the key to the OS keychain: %w", err)
		}
		cfg.Keys = append(cfg.Keys, name)
		if err := cfg.Save(); err != nil {
			_ = keyring.Delete(keyringService, name) // roll back the secret on index failure
			return err
		}
		// Echo the derived address (never the secret) as confirmation.
		return ui.RunAPI(func() (any, error) {
			return map[string]any{"status": "saved", "key": name, "address": addr}, nil
		})
	},
}

var keysRemoveCmd = &cobra.Command{
	Use:     "rm <name>",
	Short:   "Remove a saved signing key from the OS keychain",
	Example: `  rail0 keys rm payer`,
	Args:    cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		name := args[0]
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		// Removal must reach an ORPHANED entry — one the keychain holds but the index
		// does not list — or `keys add` refusing to overwrite it (#70) would leave the
		// name permanently unusable: not addable, and not removable either. So when the
		// index has no such key, fall back to asking the keychain directly. `keys` can't
		// list these (the keychain backend cannot be enumerated, which is why the index
		// exists), so this is the only way to clear one.
		indexed := slices.Contains(cfg.Keys, name)
		if !indexed {
			if _, kerr := keyring.Get(keyringService, name); kerr != nil {
				return fmt.Errorf("unknown key %q", name)
			}
		}

		scope := "from the OS keychain"
		if !indexed {
			scope = "from the OS keychain (an orphaned entry, not in the local index)"
		}
		if err := confirmDestructive(fmt.Sprintf("Remove signing key %q %s? This cannot be undone.", name, scope)); err != nil {
			return err
		}
		if err := keyring.Delete(keyringService, name); err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return fmt.Errorf("could not remove the key from the OS keychain: %w", err)
		}
		cfg.Keys = slices.DeleteFunc(cfg.Keys, func(k string) bool { return k == name })
		if err := cfg.Save(); err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return map[string]any{"status": "removed", "key": name}, nil
		})
	},
}

var keysAddressCmd = &cobra.Command{
	Use:     "address <name>",
	Short:   "Show the EVM address derived from a saved key (the address, never the key)",
	Example: `  rail0 keys address payer`,
	Args:    cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		hexKey, err := loadKey(args[0])
		if err != nil {
			return err
		}
		addr, err := addressFromKey(hexKey)
		if err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return map[string]any{"key": args[0], "address": addr}, nil
		})
	},
}

// loadKey fetches a saved key's secret from the OS keychain by name. Shared by
// `keys address` and signing-key resolution ("@name" on --private-key).
func loadKey(name string) (string, error) {
	secret, err := keyring.Get(keyringService, name)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", fmt.Errorf("unknown key %q — add it with 'rail0 keys add %s'", name, name)
	}
	if err != nil {
		return "", fmt.Errorf("could not read the key from the OS keychain: %w", err)
	}
	return secret, nil
}

// promptSecret reads a secret without echoing it. On a TTY it shows a masked
// huh input; when stdin is piped it reads a single line, so scripts can feed the
// key in without it appearing in the shell history or process arguments.
func promptSecret(label string) (string, error) {
	if !term.IsTerminal(os.Stdin.Fd()) {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return "", fmt.Errorf("read key from stdin: %w", err)
		}
		return line, nil
	}
	var v string
	err := huh.NewInput().Title(label).EchoMode(huh.EchoModePassword).Value(&v).Run()
	if errors.Is(err, huh.ErrUserAborted) {
		// ErrAborted, not a fresh error: cancelling the key prompt is the same
		// "user said no" as any other cancellation, and must exit 130. (#65)
		return "", ErrAborted
	}
	return v, err
}

func init() {
	addYesFlag(keysRemoveCmd)
	keysRemoveCmd.ValidArgsFunction = completeKeyNames
	keysAddressCmd.ValidArgsFunction = completeKeyNames
	keysCmd.AddCommand(keysAddCmd, keysRemoveCmd, keysAddressCmd)
}
