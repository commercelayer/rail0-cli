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
	Use:     "add <name>",
	Short:   "Save a signing key in the OS keychain (prompts for the key; names must be unique)",
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
		if !slices.Contains(cfg.Keys, name) {
			return fmt.Errorf("unknown key %q", name)
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
		return "", errors.New("aborted")
	}
	return v, err
}

func init() {
	keysCmd.AddCommand(keysAddCmd, keysRemoveCmd, keysAddressCmd)
}
