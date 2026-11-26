package cmd

import (
	"context"
	"fmt"
	"slices"
	"strings"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// Accounts, as the operator handles them: find one, read the whole record, read what it
// still has in motion, switch it off and on. `rail0 account` stays the merchant's view of
// its own profile; the switch is the operator's field and lives here.

var (
	adminAccSearch      string
	adminAccStatus      string
	adminAccOperator    string
	adminAccSort        string
	adminAccPage        int
	adminAccPerPage     int
	adminAccAckExposure bool
)

var accountStatuses = []string{"active", "deactivated"}

var adminAccountsCmd = &cobra.Command{
	Use:   "accounts",
	Short: "Every account: find one, read it, switch it off or on (lists when run bare)",
	Long: `List every account, as the operator reads them: the whole record, with the operator's
switch (active, status) and whether the account holds the operator grant (admin).

--search finds an account from whatever is at hand: a part of its name or of its email,
a wallet address or its beginning (0x…), or its id.`,
	Example: `  rail0 admin accounts
  rail0 admin accounts --search acme --pretty
  rail0 admin accounts --search 0x2D2B…            # whose wallet is this
  rail0 admin accounts --status deactivated
  rail0 admin accounts --operator true             # who holds the operator grant`,
	Args: cobra.NoArgs,
	RunE: operatorOnly(func(_ *cobra.Command, _ []string) error {
		if adminAccStatus != "" && !slices.Contains(accountStatuses, adminAccStatus) {
			return usagef("--status must be active or deactivated (got %q)", adminAccStatus)
		}
		operator, err := trueOrFalse("--operator", adminAccOperator)
		if err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			resp, err := newReadClient().Admin.Accounts(context.Background(), rail0.ListAdminAccountsParams{
				Query: strings.TrimSpace(adminAccSearch), Status: adminAccStatus, Admin: operator,
				Sort: adminAccSort, Page: adminAccPage, PerPage: adminAccPerPage,
			})
			if err != nil {
				return nil, err
			}
			return paged(table(resp.Data, operatorAccountColumns), resp.Meta), nil
		})
	}),
}

// trueOrFalse reads a flag that filters by a yes/no and may be left out: nil when it was.
func trueOrFalse(flag, value string) (*bool, error) {
	switch value {
	case "":
		return nil, nil
	case "true", "false":
		v := value == "true"
		return &v, nil
	default:
		return nil, usagef("%s must be 'true' or 'false' (got %q)", flag, value)
	}
}

var adminAccountsGetCmd = &cobra.Command{
	Use:   "get <account>",
	Short: "One account's whole record: profile, status, operator grant",
	Example: `  rail0 admin accounts get 019e748b-…
  rail0 admin accounts get 019e748b-… --pretty`,
	Args: cobra.ExactArgs(1),
	RunE: operatorOnly(func(_ *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newReadClient().Admin.Account(context.Background(), args[0])
		})
	}),
}

var adminAccountsExposureCmd = &cobra.Command{
	Use:   "exposure <account>",
	Short: "What an account still has in motion: what a deactivation would leave half-way",
	Long: `What an account still has in motion, as a payee:

  in_flight   payments created or signed and not yet on chain, authorization window open
  in_escrow   buyer funds held by the RAIL0 contract, not yet captured
  disputed    open disputes
  payouts     payouts prepared (pending) or sent and not yet settled (submitted)

by_token totals the payments per chain and token. A payment can be in two categories at
once (a partial capture with escrow left and a dispute on the captured part), so the
three counts do not add up to a number of payments; by_token's count does.

Nothing of this is lost by a deactivation: escrow always returns to the buyer by void or
release, and the wallet is the merchant's. It is what 'rail0 admin accounts deactivate'
reads and shows before it asks.`,
	Example: `  rail0 admin accounts exposure 019e748b-…
  rail0 admin accounts exposure 019e748b-… --pretty`,
	Args: cobra.ExactArgs(1),
	RunE: operatorOnly(func(_ *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newReadClient().Admin.AccountExposure(context.Background(), args[0])
		})
	}),
}

var adminAccountsDeactivateCmd = &cobra.Command{
	Use:   "deactivate <account>",
	Short: "Switch an account off (write mode, audited)",
	Long: `Switch an account off. From then on it can start no new payment (authorize, charge)
and write no payout; its sessions keep reading, and what is already in motion stays
where it is. Reversible: 'rail0 admin accounts activate'.

The command reads the account and its exposure first ('rail0 admin accounts exposure')
and asks, naming the account and what it still has in motion. Answering yes to that
question is the acknowledgement the gateway requires for an account with open exposure.
Without a question (--yes, or no terminal) an account with open exposure is refused
unless --acknowledge-exposure is passed too: --yes alone never switches off an account
that has something in motion.

The gateway also refuses your own account (another operator has to) and the account
holding the last active operator grant.

A write: it needs write mode ('rail0 admin write-mode on').`,
	Example: `  rail0 admin accounts deactivate 019e748b-…
  rail0 admin accounts deactivate 019e748b-… --yes                          # only with nothing in motion
  rail0 admin accounts deactivate 019e748b-… --yes --acknowledge-exposure`,
	Args: cobra.ExactArgs(1),
	RunE: operatorOnly(func(_ *cobra.Command, args []string) error {
		if err := requireWriteMode(); err != nil {
			return err
		}
		ctx := context.Background()
		read := newReadClient()
		account, err := read.Admin.Account(ctx, args[0])
		if err != nil {
			return err
		}
		if !account.Active {
			ui.Notice("already_deactivated", map[string]any{"account_id": account.ID},
				fmt.Sprintf("Account %s is already deactivated: nothing was changed.", accountLabel(account)))
			return ui.RunAPI(func() (any, error) { return account, nil })
		}
		exposure, err := read.Admin.AccountExposure(ctx, account.ID)
		if err != nil {
			return err
		}
		// With something in motion, a question that shows the figures is the
		// acknowledgement. Where no question will be asked, it has to be the flag.
		if !exposure.Empty && !adminAccAckExposure && (assumeYes || !interactive()) {
			return newUsageError("exposure_not_acknowledged",
				"account %s still has %s; read 'rail0 admin accounts exposure %s' and pass --acknowledge-exposure to switch it off anyway",
				accountLabel(account), exposureSummary(exposure), account.ID)
		}
		if err := confirmDestructive(deactivateAccountPrompt(account, exposure)); err != nil {
			return err
		}
		return ui.RunAPIWithMessage("Deactivating…", func() (any, error) {
			return newClient().Admin.DeactivateAccount(ctx, account.ID, !exposure.Empty)
		})
	}),
}

var adminAccountsActivateCmd = &cobra.Command{
	Use:   "activate <account>",
	Short: "Switch a deactivated account back on (write mode, audited)",
	Long: `Switch a deactivated account back on: it can start payments and write payouts again.
No confirmation is asked: 'rail0 admin accounts deactivate' reverses it.

A write: it needs write mode ('rail0 admin write-mode on').`,
	Example: `  rail0 admin accounts activate 019e748b-…`,
	Args:    cobra.ExactArgs(1),
	RunE: operatorOnly(func(_ *cobra.Command, args []string) error {
		if err := requireWriteMode(); err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return newClient().Admin.ActivateAccount(context.Background(), args[0])
		})
	}),
}

// accountLabel names an account for a person: its name and its id.
func accountLabel(a *rail0.AccountFull) string {
	if a.Name == "" {
		return a.ID
	}
	return fmt.Sprintf("%q (%s)", a.Name, a.ID)
}

// exposureSummary says an exposure in one line: the categories that are not zero, then
// the money per token.
func exposureSummary(e *rail0.AccountExposure) string {
	var parts []string
	add := func(n int, one, many string) {
		switch {
		case n == 1:
			parts = append(parts, "1 "+one)
		case n > 1:
			parts = append(parts, fmt.Sprintf("%d %s", n, many))
		}
	}
	add(e.Payments.InFlight, "payment in flight", "payments in flight")
	add(e.Payments.InEscrow, "payment in escrow", "payments in escrow")
	add(e.Payments.Disputed, "open dispute", "open disputes")
	add(e.Payouts.Pending, "payout pending", "payouts pending")
	add(e.Payouts.Submitted, "payout submitted", "payouts submitted")
	if len(parts) == 0 {
		return "nothing in motion"
	}
	out := strings.Join(parts, ", ")
	var money []string
	for _, t := range e.Payments.ByToken {
		symbol := t.Symbol
		if symbol == "" {
			symbol = ui.ShortHex(t.Token)
		}
		amount := t.AmountFormatted + " " + symbol
		if t.ChainID != nil {
			amount += " on " + chainName(*t.ChainID)
		}
		money = append(money, amount)
	}
	if len(money) > 0 {
		out += " (" + strings.Join(money, ", ") + ")"
	}
	return out
}

// chainName is a chain for a sentence: its slug when the catalogue has it, its id otherwise.
func chainName(chainID int) string {
	return chainCell(map[string]any{"chain_id": float64(chainID)})
}

// deactivateAccountPrompt is the question asked before an account is switched off: which
// account, what it still has in motion, and what the switch does.
func deactivateAccountPrompt(a *rail0.AccountFull, e *rail0.AccountExposure) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Deactivate account %s?\n", accountLabel(a))
	if e.Empty {
		b.WriteString("  - it has nothing in motion\n")
	} else {
		fmt.Fprintf(&b, "  - it still has %s\n", exposureSummary(e))
		b.WriteString("    all of it stays where it is: the merchant can still capture, refund and release\n")
		if e.Payments.InFlight > 0 {
			b.WriteString("    the payments in flight can no longer be authorized or charged\n")
		}
	}
	b.WriteString("  - it can start no new payment and write no payout\n" +
		"  - reversible: 'rail0 admin accounts activate' switches it back on")
	return b.String()
}

func init() {
	f := adminAccountsCmd.Flags()
	f.StringVar(&adminAccSearch, "search", "", "A part of the name or email, a wallet address or its beginning (0x…), or the account's id")
	f.StringVar(&adminAccStatus, "status", "", "Only accounts with this status: active or deactivated")
	f.StringVar(&adminAccOperator, "operator", "", "Only accounts that hold the operator grant ('true') or that do not ('false')")
	f.StringVar(&adminAccSort, "sort", "", "Sort by name, email or created_at (prefix - for descending; default: name)")
	f.IntVar(&adminAccPage, "page", 0, "Page number (1-based)")
	f.IntVar(&adminAccPerPage, "per-page", 0, "Results per page")
	_ = adminAccountsCmd.RegisterFlagCompletionFunc("status", fixedCompletions(accountStatuses...))
	_ = adminAccountsCmd.RegisterFlagCompletionFunc("operator", fixedCompletions("true", "false"))

	adminAccountsDeactivateCmd.Flags().BoolVar(&adminAccAckExposure, "acknowledge-exposure", false,
		"Switch the account off although it still has payments, escrow, disputes or payouts in motion (needed with --yes)")
	addYesFlag(adminAccountsDeactivateCmd)
}
