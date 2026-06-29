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
// wallet's own identity; unique per account) — no @alias there, a wallet
// already has its own address. (create --address does accept an @alias: it
// names the address to register.)

var walletsAccount string
var walletsActive string
var walletsChainID int
var walletsSymbol string

var walletsCmd = &cobra.Command{
	Use:   "wallets",
	Short: "Manage the logged-in account's wallets (lists when run bare)",
	Long: `List and manage the wallets of an account.

When listing, the account is --account if given, otherwise the logged-in
session's account ('rail0 auth login'); one of the two must be present, else it
errors. The verbs below act on one wallet (id or 0x address) of the session
account.`,
	Example: `  rail0 wallets                                   # session account
  rail0 wallets --account 019e748b-… --chain-id 5042002 --symbol USDC
  rail0 wallets get 0xABCD…
  rail0 wallets create --address 0xABCD…`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		account, err := resolveAccountID(walletsAccount)
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
			resp, err := newClient().Wallets.List(context.Background(), account, p)
			if err != nil {
				return nil, err
			}
			return resp.Data, nil // the API returns a bare array; pagination is in headers
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
			return newClient().Wallets.Get(context.Background(), account, args[0])
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
		// Resolve @alias here: create names the address to register, so pointing
		// at a saved address is meaningful (unlike addressing an existing wallet).
		addr, err := resolveAlias(createWalletAddress)
		if err != nil {
			return err
		}
		req := rail0.CreateWalletRequest{Address: addr, Label: createWalletLabel}
		return ui.RunAPI(func() (any, error) {
			return newClient().Wallets.Create(context.Background(), account, req)
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
			return newClient().Wallets.Update(context.Background(), account, wallet, req)
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
			return nil, newClient().Wallets.Delete(context.Background(), account, args[0])
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
			return newClient().Wallets.Balances(context.Background(), account, args[0], p)
		})
	},
}

// walletsAliasCmd is a convenience alias of `aliases add` with the arguments in
// wallet-first order (<address> <name>), reading naturally after copying an
// address out of `wallets`. It shares addAlias, so the same name-uniqueness
// rule applies (an existing name is rejected — rm it first).
var walletsAliasCmd = &cobra.Command{
	Use:     "alias <address> <name>",
	Short:   "Save a unique address-book alias for an address (like 'aliases add', args reversed)",
	Example: `  rail0 wallets alias 0xAbC123… payee`,
	Args:    cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		addr, name := args[0], args[1]
		if err := addAlias(name, addr); err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return map[string]any{"status": "saved", "alias": name, "address": addr}, nil
		})
	},
}

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	// wallets (list) — each wallet lists its tokens nested under "tokens".
	// --account targets a specific account; without it the logged-in session's
	// account is used (one of the two must be present, else error).
	// --active filters the wallets; --chain-id/--symbol narrow the nested tokens
	// (a wallet still appears, possibly with no tokens).
	walletsCmd.Flags().StringVar(&walletsAccount, "account", "", "Account id to list (default: the logged-in session account)")
	walletsCmd.Flags().StringVar(&walletsActive, "active", "", "Filter wallets by active flag ('true' or 'false')")
	walletsCmd.Flags().IntVar(&walletsChainID, "chain-id", 0, "Restrict nested tokens to this chain id")
	walletsCmd.Flags().StringVarP(&walletsSymbol, "symbol", "s", "", "Restrict nested tokens to this symbol (e.g. USDC)")

	// wallets create
	walletsCreateCmd.Flags().StringVarP(&createWalletAddress, "address", "A", "", "EVM wallet address (0x-prefixed hex, or @alias)")
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
		walletsAliasCmd,
	)
}
