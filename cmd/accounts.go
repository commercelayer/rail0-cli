package cmd

import (
	"context"
	"errors"

	"github.com/rail0/cli/internal/ui"
	rail0 "github.com/rail0/go-sdk"
	"github.com/spf13/cobra"
)

var accountsCmd = &cobra.Command{
	Use:   "accounts",
	Short: "Account wallet management",
}

// ── accounts wallets (list + subcommand group) ─────────────────────────────────
// Bare `accounts wallets <account_id>` lists; the verbs below act on a wallet.

var accountsWalletsActive string
var accountsWalletsChainID int
var accountsWalletsSymbol string

var accountsWalletsCmd = &cobra.Command{
	Use:   "wallets <account_id>",
	Short: "List an account's wallet-token rows (or manage them — see subcommands)",
	Example: `  rail0 accounts wallets acct_abc123 --chain-id 5042002 --symbol USDC
  rail0 accounts wallets acct_abc123 create --address 0xABCD…`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := rail0.ListWalletsParams{ChainID: accountsWalletsChainID, TokenSymbol: accountsWalletsSymbol}
		if accountsWalletsActive != "" {
			switch accountsWalletsActive {
			case "true":
				t := true
				p.Active = &t
			case "false":
				f := false
				p.Active = &f
			default:
				return errors.New("--active must be 'true' or 'false'")
			}
		}
		return ui.RunAPI(func() (any, error) {
			return newClient().Accounts.Wallets(context.Background(), args[0], p)
		})
	},
}

// ── accounts wallets get ────────────────────────────────────────────────────

var accountsWalletsGetCmd = &cobra.Command{
	Use:   "get <account_id> <wallet>",
	Short: "Fetch a single wallet (by id, 0x address, or @alias)",
	Example: `  rail0 accounts wallets get acct_abc123 0xABCD…
  rail0 accounts wallets get acct_abc123 @payee`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		wallet, err := resolveAlias(args[1])
		if err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return newClient().Accounts.GetWallet(context.Background(), args[0], wallet)
		})
	},
}

// ── accounts wallets create ───────────────────────────────────────────────────

var createWalletAddress string
var createWalletLabel string

var accountsWalletsCreateCmd = &cobra.Command{
	Use:     "create <account_id>",
	Short:   "Register a new EVM wallet address on an account",
	Example: `  rail0 accounts wallets create acct_abc123 --address 0xABCD… --label "Main wallet"`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, err := resolveAlias(createWalletAddress)
		if err != nil {
			return err
		}
		req := rail0.CreateWalletRequest{Address: addr, Label: createWalletLabel}
		return ui.RunAPI(func() (any, error) {
			return newClient().Accounts.CreateWallet(context.Background(), args[0], req)
		})
	},
}

// ── accounts wallets update ───────────────────────────────────────────────────

var updateWalletLabel string
var updateWalletActive string

var accountsWalletsUpdateCmd = &cobra.Command{
	Use:     "update <account_id> <wallet>",
	Short:   "Update label or active status on a wallet (by id, 0x address, or @alias)",
	Example: `  rail0 accounts wallets update acct_abc123 @payee --label "New label" --active true`,
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		wallet, err := resolveAlias(args[1])
		if err != nil {
			return err
		}
		req := rail0.UpdateWalletRequest{Label: updateWalletLabel}
		if updateWalletActive != "" {
			switch updateWalletActive {
			case "true":
				t := true
				req.Active = &t
			case "false":
				f := false
				req.Active = &f
			default:
				return errors.New("--active must be 'true' or 'false'")
			}
		}
		return ui.RunAPI(func() (any, error) {
			return newClient().Accounts.UpdateWallet(context.Background(), args[0], wallet, req)
		})
	},
}

// ── accounts wallets delete ───────────────────────────────────────────────────

var accountsWalletsDeleteCmd = &cobra.Command{
	Use:     "delete <account_id> <wallet>",
	Short:   "Remove a wallet from an account (by id, 0x address, or @alias)",
	Example: `  rail0 accounts wallets delete acct_abc123 @payee`,
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		wallet, err := resolveAlias(args[1])
		if err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return nil, newClient().Accounts.DeleteWallet(context.Background(), args[0], wallet)
		})
	},
}

// ── accounts wallets balances ─────────────────────────────────────────────────

var balancesChainID int
var balancesSymbol string

var accountsWalletsBalancesCmd = &cobra.Command{
	Use:   "balances <account_id> <wallet>",
	Short: "Read a wallet's live on-chain balances (by id, 0x address, or @alias)",
	Long: `Read a wallet's live on-chain balances across the configured chains.

Each chain reports the native gas-token balance and the active token balances,
or an error if that chain's RPC was unreachable (reported per chain, so one dead
RPC never hides the others). Use --chain-id / --symbol to narrow the result.`,
	Example: `  rail0 accounts wallets balances acct_abc123 @payee
  rail0 accounts wallets balances acct_abc123 0xABCD… --chain-id 84532 --symbol USDC`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		wallet, err := resolveAlias(args[1])
		if err != nil {
			return err
		}
		p := rail0.WalletBalancesParams{ChainID: balancesChainID, TokenSymbol: balancesSymbol}
		return ui.RunAPI(func() (any, error) {
			return newClient().Accounts.Balances(context.Background(), args[0], wallet, p)
		})
	},
}

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	// accounts wallets (list)
	accountsWalletsCmd.Flags().StringVar(&accountsWalletsActive, "active", "", "Filter by active flag ('true' or 'false')")
	accountsWalletsCmd.Flags().IntVar(&accountsWalletsChainID, "chain-id", 0, "Filter by chain id")
	accountsWalletsCmd.Flags().StringVarP(&accountsWalletsSymbol, "symbol", "s", "", "Filter by token symbol (e.g. USDC)")

	// accounts wallets create
	accountsWalletsCreateCmd.Flags().StringVarP(&createWalletAddress, "address", "A", "", "EVM wallet address (0x-prefixed hex, or @alias)")
	_ = accountsWalletsCreateCmd.MarkFlagRequired("address")
	accountsWalletsCreateCmd.Flags().StringVarP(&createWalletLabel, "label", "l", "", "Human-readable wallet label")

	// accounts wallets update
	accountsWalletsUpdateCmd.Flags().StringVarP(&updateWalletLabel, "label", "l", "", "New wallet label")
	accountsWalletsUpdateCmd.Flags().StringVar(&updateWalletActive, "active", "", "Set active flag ('true' or 'false')")

	// accounts wallets balances
	accountsWalletsBalancesCmd.Flags().IntVar(&balancesChainID, "chain-id", 0, "Restrict to one chain id")
	accountsWalletsBalancesCmd.Flags().StringVarP(&balancesSymbol, "symbol", "s", "", "Restrict tokens to this symbol (e.g. USDC)")

	accountsWalletsCmd.AddCommand(
		accountsWalletsGetCmd,
		accountsWalletsCreateCmd,
		accountsWalletsUpdateCmd,
		accountsWalletsDeleteCmd,
		accountsWalletsBalancesCmd,
	)
	accountsCmd.AddCommand(accountsWalletsCmd)
}
