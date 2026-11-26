package cmd

import (
	"context"
	"fmt"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// Any account's wallets, as the operator handles them: the list, what still names an
// address, and the switch. `rail0 wallets` stays the merchant's: its own account, and
// writes on the wallet it signed in with only. An operator's switch-off is the definitive
// revocation: the wallet's own session cannot lift it.

var (
	adminWalletsAccount string
	adminWalletsActive  string
	adminWalletsSort    string
	adminWalletsPage    int
	adminWalletsPerPage int
)

var adminWalletsCmd = &cobra.Command{
	Use:   "wallets",
	Short: "Any account's wallets: list them, see what names one, switch it off or on (lists when run bare)",
	Long: `List the wallets of an account (--account), each with the tokens it accepts and, when
it is off, who switched it off (deactivated_by: owner or operator).

The account is always named with --account: 'rail0 admin accounts --search 0x…' finds
the account a wallet address belongs to.`,
	Example: `  rail0 admin wallets --account 019e748b-…
  rail0 admin wallets --account 019e748b-… --active false --pretty
  rail0 admin wallets usage 0x2D2B… --account 019e748b-…
  rail0 admin wallets deactivate 0x2D2B… --account 019e748b-…`,
	Args: cobra.NoArgs,
	RunE: operatorOnly(func(_ *cobra.Command, _ []string) error {
		account, err := adminWalletsAccountID()
		if err != nil {
			return err
		}
		active, err := trueOrFalse("--active", adminWalletsActive)
		if err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			resp, err := newReadClient().Wallets.List(context.Background(), account, rail0.ListWalletsParams{
				Active: active, Sort: adminWalletsSort, Page: adminWalletsPage, PerPage: adminWalletsPerPage,
			})
			if err != nil {
				return nil, err
			}
			return paged(table(resp.Data, operatorWalletColumns), resp.Meta), nil
		})
	}),
}

// adminWalletsAccountID is the --account every wallet command needs. An operator's own
// account is never the default here: the command acts on someone else's wallets, and a
// forgotten flag must not land on the operator's own.
func adminWalletsAccountID() (string, error) {
	if adminWalletsAccount == "" {
		return "", usagef("pass --account <id>: the account the wallet belongs to " +
			"('rail0 admin accounts --search 0x…' finds it from an address)")
	}
	return adminWalletsAccount, nil
}

// adminWallet reads the two things a wallet command takes: the account and the wallet
// (its id, its 0x address or an @alias).
func adminWallet(args []string) (account, wallet string, err error) {
	if account, err = adminWalletsAccountID(); err != nil {
		return "", "", err
	}
	if wallet, err = resolveAlias(args[0]); err != nil {
		return "", "", err
	}
	return account, wallet, nil
}

var adminWalletsUsageCmd = &cobra.Command{
	Use:   "usage <wallet>",
	Short: "What still names a wallet's address: payments as payee and payer, transactions sent",
	Long: `Count what still names a wallet's address, across every account: the payments it is
the payee of, the payments it is the payer of, and the transactions it sent. How much
history the address carries: what 'rail0 admin wallets deactivate' shows before it asks.`,
	Example: `  rail0 admin wallets usage 0x2D2B… --account 019e748b-…`,
	Args:    cobra.ExactArgs(1),
	RunE: operatorOnly(func(_ *cobra.Command, args []string) error {
		account, wallet, err := adminWallet(args)
		if err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			usage, err := newReadClient().Admin.WalletUsage(context.Background(), account, wallet)
			if err != nil {
				return nil, err
			}
			return walletUsageView{usage.WalletID, usage.Address, usage.PaymentsAsPayee, usage.PaymentsAsPayer, usage.TransactionsAsSender}, nil
		})
	}),
}

// walletUsageView prints the three counts whatever they are: a zero is an answer here
// ("no payment names it as payer"), and the SDK's type leaves zeros out.
type walletUsageView struct {
	WalletID             string `json:"wallet_id"`
	Address              string `json:"address"`
	PaymentsAsPayee      int    `json:"payments_as_payee"`
	PaymentsAsPayer      int    `json:"payments_as_payer"`
	TransactionsAsSender int    `json:"transactions_as_sender"`
}

var adminWalletsDeactivateCmd = &cobra.Command{
	Use:   "deactivate <wallet>",
	Short: "Switch a wallet off, on any account (write mode, audited)",
	Long: `Switch a wallet off. It stops being offered to buyers, so no new payment can name it,
and webhooks stop being dispatched for its payments. Existing payments settle whatever
the switch says: they carry the address themselves.

This is the definitive revocation, for a leaked or lost key: a wallet an operator
switched off can be switched back on only by an operator ('rail0 admin wallets
activate'). Its own session cannot, where it could after switching itself off.

The command reads what still names the address first ('rail0 admin wallets usage') and
asks. The account's only active wallet is refused (422 last_active_wallet): the account
could no longer sign in.

A write: it needs write mode ('rail0 admin write-mode on').`,
	Example: `  rail0 admin wallets deactivate 0x2D2B… --account 019e748b-…
  rail0 admin wallets deactivate 019f02aa-… --account 019e748b-… --yes`,
	Args: cobra.ExactArgs(1),
	RunE: operatorOnly(func(_ *cobra.Command, args []string) error {
		account, wallet, err := adminWallet(args)
		if err != nil {
			return err
		}
		if err := requireWriteMode(); err != nil {
			return err
		}
		ctx := context.Background()
		usage, err := newReadClient().Admin.WalletUsage(ctx, account, wallet)
		if err != nil {
			return err
		}
		if err := confirmDestructive(deactivateWalletPrompt(account, usage)); err != nil {
			return err
		}
		return ui.RunAPIWithMessage("Deactivating…", func() (any, error) {
			off := false
			return newClient().Wallets.Update(ctx, account, wallet, rail0.UpdateWalletRequest{Active: &off})
		})
	}),
}

var adminWalletsActivateCmd = &cobra.Command{
	Use:   "activate <wallet>",
	Short: "Switch a wallet back on, on any account (write mode, audited)",
	Long: `Switch a wallet back on: it is offered to buyers again and its webhooks resume. The
only way back for a wallet an operator switched off. No confirmation is asked:
'rail0 admin wallets deactivate' reverses it.

A write: it needs write mode ('rail0 admin write-mode on').`,
	Example: `  rail0 admin wallets activate 0x2D2B… --account 019e748b-…`,
	Args:    cobra.ExactArgs(1),
	RunE: operatorOnly(func(_ *cobra.Command, args []string) error {
		account, wallet, err := adminWallet(args)
		if err != nil {
			return err
		}
		if err := requireWriteMode(); err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			on := true
			return newClient().Wallets.Update(context.Background(), account, wallet, rail0.UpdateWalletRequest{Active: &on})
		})
	}),
}

// deactivateWalletPrompt is the question asked before an operator switches a wallet off:
// which wallet, how much history its address carries, and what the switch does.
func deactivateWalletPrompt(account string, u *rail0.WalletUsage) string {
	return fmt.Sprintf(
		"Switch off wallet %s of account %s?\n"+
			"  - its address is the payee of %d payments and the payer of %d, and sent %d transactions:\n"+
			"    they all stay, and the open ones still settle\n"+
			"  - buyers can no longer discover it and no new payment can name it\n"+
			"  - webhooks STOP for its payments that are still in flight\n"+
			"  - only an operator can switch it back on ('rail0 admin wallets activate')",
		u.Address, account, u.PaymentsAsPayee, u.PaymentsAsPayer, u.TransactionsAsSender)
}

func init() {
	adminWalletsCmd.PersistentFlags().StringVar(&adminWalletsAccount, "account", "", "The account the wallets belong to (required)")
	f := adminWalletsCmd.Flags()
	f.StringVar(&adminWalletsActive, "active", "", "Only wallets that are on ('true') or off ('false')")
	f.StringVar(&adminWalletsSort, "sort", "", "Sort wallets (e.g. -created_at)")
	f.IntVar(&adminWalletsPage, "page", 0, "Page number (1-based)")
	f.IntVar(&adminWalletsPerPage, "per-page", 0, "Wallets per page")
	_ = adminWalletsCmd.RegisterFlagCompletionFunc("active", fixedCompletions("true", "false"))

	addYesFlag(adminWalletsDeactivateCmd)
}
