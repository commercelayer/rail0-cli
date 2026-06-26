package cmd

import (
	"fmt"
	"os"
	"strings"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/rail0/cli/internal/config"
	"github.com/rail0/cli/internal/keystore"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// keysCmd lists the stored signing accounts when run bare, and holds the
// import/address/rm verbs.
var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Signing keys in the OS keychain — list them, or manage with import/address/rm",
	Long: `Store signing private keys in the OS keychain (macOS Keychain, Linux Secret
Service, or Windows Credential Manager) and reference them by name with
--account (-k) on any signing command — so the raw key never appears in your
shell history.

Run bare to list the stored accounts (names + derived addresses, never keys).
Only the derived address is recorded in the config file; the private key itself
lives in the keychain.`,
	Example: `  rail0 keys
  rail0 keys import payee
  rail0 keys rm payee`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		rows := make([]map[string]any, 0, len(cfg.Keys))
		for name, addr := range cfg.Keys {
			rows = append(rows, map[string]any{"account": name, "address": addr})
		}
		return ui.RunAPI(func() (any, error) { return rows, nil })
	},
}

var keysFromStdin bool

var keysImportCmd = &cobra.Command{
	Use:   "import <name>",
	Short: "Import a private key into the OS keychain",
	Long: `Import a secp256k1 private key under a name. By default the key is read from
a hidden prompt (so it never lands in your shell history); pass --from-stdin to
pipe it in for scripted setup.`,
	Example: `  rail0 keys import payee
  echo "$ACCOUNT_PRIVATE_KEY" | rail0 keys import ci --from-stdin`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		name := args[0]
		key, err := readPrivateKey(keysFromStdin)
		if err != nil {
			return err
		}
		addr, err := addressFromKey(key)
		if err != nil {
			return err
		}
		if err := keystore.Set(name, key); err != nil {
			return fmt.Errorf("store key in keychain: %w", err)
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if cfg.Keys == nil {
			cfg.Keys = map[string]string{}
		}
		cfg.Keys[name] = addr
		if err := cfg.Save(); err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return map[string]any{"status": "stored", "account": name, "address": addr}, nil
		})
	},
}

var keysAddressCmd = &cobra.Command{
	Use:     "address <name>",
	Short:   "Print the address derived from a stored key",
	Args:    cobra.ExactArgs(1),
	Example: `  rail0 keys address payee`,
	RunE: func(_ *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		addr, ok := cfg.Keys[args[0]]
		if !ok {
			return fmt.Errorf("unknown account %q", args[0])
		}
		return ui.RunAPI(func() (any, error) {
			return map[string]any{"account": args[0], "address": addr}, nil
		})
	},
}

var keysRemoveCmd = &cobra.Command{
	Use:     "rm <name>",
	Short:   "Remove a stored key from the keychain",
	Args:    cobra.ExactArgs(1),
	Example: `  rail0 keys rm payee`,
	RunE: func(_ *cobra.Command, args []string) error {
		name := args[0]
		if err := keystore.Delete(name); err != nil {
			return fmt.Errorf("remove key from keychain: %w", err)
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		delete(cfg.Keys, name)
		if err := cfg.Save(); err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return map[string]any{"status": "removed", "account": name}, nil
		})
	},
}

// readPrivateKey reads a 0x-prefixed hex private key, either from a hidden
// terminal prompt (default — keeps the secret out of shell history) or from
// stdin when --from-stdin is set (for scripted setup).
func readPrivateKey(fromStdin bool) (string, error) {
	if fromStdin {
		var key string
		if _, err := fmt.Scanln(&key); err != nil {
			return "", fmt.Errorf("read key from stdin: %w", err)
		}
		return strings.TrimSpace(key), nil
	}
	fmt.Fprint(os.Stderr, "Enter private key: ")
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read key: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}

// addressFromKey derives the 0x checksummed EVM address for a hex private key,
// validating the key in the process.
func addressFromKey(privateKeyHex string) (string, error) {
	key, err := ethcrypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return "", fmt.Errorf("invalid private key: %w", err)
	}
	return ethcrypto.PubkeyToAddress(key.PublicKey).Hex(), nil
}

func init() {
	keysImportCmd.Flags().BoolVar(&keysFromStdin, "from-stdin", false, "Read the private key from stdin instead of a hidden prompt")
	keysCmd.AddCommand(keysImportCmd, keysAddressCmd, keysRemoveCmd)
}
