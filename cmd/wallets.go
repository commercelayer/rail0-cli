package cmd

import (
	"context"
	"errors"

	"github.com/rail0/cli/internal/ui"
	rail0 "github.com/rail0/go-sdk"
	"github.com/spf13/cobra"
)

// walletsCmd manages the logged-in account's wallets. The account id is taken
// from the session (JWT), never passed as an argument — run bare to list; the
// verbs below act on a single wallet, named by its id or its 0x address (the
// wallet's own identity; unique per account). Address-book @aliases are for
// external addresses in `payments`, not for your own wallets.

var walletsActive string
var walletsChainID int
var walletsSymbol string

var walletsCmd = &cobra.Command{
	Use:   "wallets",
	Short: "Manage the logged-in account's wallets (lists when run bare)",
	Long: `List and manage the wallets of the logged-in account.

The account is taken from the login session — run 'rail0 auth login' first.
Run bare to list; the verbs below act on one wallet, named by its id or 0x
address (unique per account).`,
	Example: `  rail0 wallets --chain-id 5042002 --symbol USDC
  rail0 wallets get 0xABCD…
  rail0 wallets create --address 0xABCD…`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		account, err := currentAccountID()
		if err != nil {
			return err
		}
		p := rail0.ListWalletsParams{ChainID: walletsChainID, TokenSymbol: walletsSymbol}
		if walletsActive != "" {
			switch walletsActive {
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
			return newClient().Accounts.Wallets(context.Background(), account, p)
		})
	},
}

// ── wallets get ───────────────────────────────────────────────────────────────

var walletsGetCmd = &cobra.Command{
	Use:     "get <wallet>",
	Short:   "Fetch a single wallet (by id or 0x address)",
	Example: `  rail0 wallets get 0xABCD…`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		account, err := currentAccountID()
		if err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return newClient().Accounts.GetWallet(context.Background(), account, args[0])
		})
	},
}

// ── wallets create ──────────────────────────────────────────────────────────

var createWalletAddress string
var createWalletLabel string

var walletsCreateCmd = &cobra.Command{
	Use:     "create",
	Short:   "Register a new EVM wallet address on the account",
	Example: `  rail0 wallets create --address 0xABCD… --label "Main wallet"`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		account, err := currentAccountID()
		if err != nil {
			return err
		}
		req := rail0.CreateWalletRequest{Address: createWalletAddress, Label: createWalletLabel}
		return ui.RunAPI(func() (any, error) {
			return newClient().Accounts.CreateWallet(context.Background(), account, req)
		})
	},
}

// ── wallets update ──────────────────────────────────────────────────────────

var updateWalletLabel string
var updateWalletActive string

var walletsUpdateCmd = &cobra.Command{
	Use:     "update <wallet>",
	Short:   "Update label or active status on a wallet (by id or 0x address)",
	Example: `  rail0 wallets update 0xABCD… --label "New label" --active true`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		account, err := currentAccountID()
		if err != nil {
			return err
		}
		wallet := args[0]
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
			return newClient().Accounts.UpdateWallet(context.Background(), account, wallet, req)
		})
	},
}

// ── wallets delete ──────────────────────────────────────────────────────────

var walletsDeleteCmd = &cobra.Command{
	Use:     "delete <wallet>",
	Short:   "Remove a wallet from the account (by id or 0x address)",
	Example: `  rail0 wallets delete 0xABCD…`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		account, err := currentAccountID()
		if err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return nil, newClient().Accounts.DeleteWallet(context.Background(), account, args[0])
		})
	},
}

// ── wallets balances ──────────────────────────────────────────────────────────

var balancesChainID int
var balancesSymbol string

var walletsBalancesCmd = &cobra.Command{
	Use:   "balances <wallet>",
	Short: "Read a wallet's live on-chain balances (by id or 0x address)",
	Long: `Read a wallet's live on-chain balances across the configured chains.

Each chain reports the native gas-token balance and the active token balances,
or an error if that chain's RPC was unreachable (reported per chain, so one dead
RPC never hides the others). Use --chain-id / --symbol to narrow the result.`,
	Example: `  rail0 wallets balances 0xABCD… --chain-id 84532 --symbol USDC`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		account, err := currentAccountID()
		if err != nil {
			return err
		}
		p := rail0.WalletBalancesParams{ChainID: balancesChainID, TokenSymbol: balancesSymbol}
		return ui.RunAPI(func() (any, error) {
			return newClient().Accounts.Balances(context.Background(), account, args[0], p)
		})
	},
}

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	// wallets (list)
	walletsCmd.Flags().StringVar(&walletsActive, "active", "", "Filter by active flag ('true' or 'false')")
	walletsCmd.Flags().IntVar(&walletsChainID, "chain-id", 0, "Filter by chain id")
	walletsCmd.Flags().StringVarP(&walletsSymbol, "symbol", "s", "", "Filter by token symbol (e.g. USDC)")

	// wallets create
	walletsCreateCmd.Flags().StringVarP(&createWalletAddress, "address", "A", "", "EVM wallet address (0x-prefixed hex)")
	_ = walletsCreateCmd.MarkFlagRequired("address")
	walletsCreateCmd.Flags().StringVarP(&createWalletLabel, "label", "l", "", "Human-readable wallet label")

	// wallets update
	walletsUpdateCmd.Flags().StringVarP(&updateWalletLabel, "label", "l", "", "New wallet label")
	walletsUpdateCmd.Flags().StringVar(&updateWalletActive, "active", "", "Set active flag ('true' or 'false')")

	// wallets balances
	walletsBalancesCmd.Flags().IntVar(&balancesChainID, "chain-id", 0, "Restrict to one chain id")
	walletsBalancesCmd.Flags().StringVarP(&balancesSymbol, "symbol", "s", "", "Restrict tokens to this symbol (e.g. USDC)")

	walletsCmd.AddCommand(
		walletsGetCmd,
		walletsCreateCmd,
		walletsUpdateCmd,
		walletsDeleteCmd,
		walletsBalancesCmd,
	)
}
