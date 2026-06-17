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

// ── accounts wallets ──────────────────────────────────────────────────────────

var accountsWalletsActive string
var accountsWalletsChainID int
var accountsWalletsSymbol string

var accountsWalletsCmd = &cobra.Command{
	Use:     "wallets <account_id>",
	Short:   "List wallet-token rows for an account",
	Example: `  rail0 accounts wallets acct_abc123 --chain-id 5042002 --symbol USDC`,
	Args:    cobra.ExactArgs(1),
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

// ── accounts wallet ───────────────────────────────────────────────────────────

var accountsWalletCmd = &cobra.Command{
	Use:     "wallet <account_id> <wallet_id>",
	Short:   "Fetch a single wallet",
	Example: `  rail0 accounts wallet acct_abc123 wlt_xyz789`,
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newClient().Accounts.GetWallet(context.Background(), args[0], args[1])
		})
	},
}

// ── accounts create-wallet ────────────────────────────────────────────────────

var createWalletAddress string
var createWalletLabel string

var accountsCreateWalletCmd = &cobra.Command{
	Use:   "create-wallet <account_id>",
	Short: "Register a new EVM wallet address on an account",
	Example: `  rail0 accounts create-wallet acct_abc123 --address 0xABCD… --label "Main wallet"`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := rail0.CreateWalletRequest{Address: createWalletAddress, Label: createWalletLabel}
		return ui.RunAPI(func() (any, error) {
			return newClient().Accounts.CreateWallet(context.Background(), args[0], req)
		})
	},
}

// ── accounts update-wallet ────────────────────────────────────────────────────

var updateWalletLabel string
var updateWalletActive string

var accountsUpdateWalletCmd = &cobra.Command{
	Use:     "update-wallet <account_id> <wallet_id>",
	Short:   "Update label or active status on a wallet",
	Example: `  rail0 accounts update-wallet acct_abc123 wlt_xyz789 --label "New label" --active true`,
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
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
			return newClient().Accounts.UpdateWallet(context.Background(), args[0], args[1], req)
		})
	},
}

// ── accounts delete-wallet ────────────────────────────────────────────────────

var accountsDeleteWalletCmd = &cobra.Command{
	Use:     "delete-wallet <account_id> <wallet_id>",
	Short:   "Remove a wallet from an account",
	Example: `  rail0 accounts delete-wallet acct_abc123 wlt_xyz789`,
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return nil, newClient().Accounts.DeleteWallet(context.Background(), args[0], args[1])
		})
	},
}

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	// accounts wallets
	accountsWalletsCmd.Flags().StringVar(&accountsWalletsActive, "active", "", "Filter by active flag ('true' or 'false')")
	accountsWalletsCmd.Flags().IntVar(&accountsWalletsChainID, "chain-id", 0, "Filter by chain id")
	accountsWalletsCmd.Flags().StringVarP(&accountsWalletsSymbol, "symbol", "s", "", "Filter by token symbol (e.g. USDC)")
	accountsCmd.AddCommand(accountsWalletsCmd)

	// accounts wallet
	accountsCmd.AddCommand(accountsWalletCmd)

	// accounts create-wallet
	accountsCreateWalletCmd.Flags().StringVarP(&createWalletAddress, "address", "A", "", "EVM wallet address (0x-prefixed hex, 20 bytes)")
	_ = accountsCreateWalletCmd.MarkFlagRequired("address")
	accountsCreateWalletCmd.Flags().StringVarP(&createWalletLabel, "label", "l", "", "Human-readable wallet label")
	accountsCmd.AddCommand(accountsCreateWalletCmd)

	// accounts update-wallet
	accountsUpdateWalletCmd.Flags().StringVarP(&updateWalletLabel, "label", "l", "", "New wallet label")
	accountsUpdateWalletCmd.Flags().StringVar(&updateWalletActive, "active", "", "Set active flag ('true' or 'false')")
	accountsCmd.AddCommand(accountsUpdateWalletCmd)

	// accounts delete-wallet
	accountsCmd.AddCommand(accountsDeleteWalletCmd)
}
