package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// payoutDestinationsCmd manages WHERE payouts go: the venue deposit addresses (an
// exchange account, Circle Mint, another wallet of the merchant's) a wallet may send
// its tokens to. Run bare to list.

var payoutDestinationStatuses = []string{
	rail0.PayoutDestinationStatusUnverified,
	rail0.PayoutDestinationStatusVerified,
	rail0.PayoutDestinationStatusDisabled,
}

var payoutDestinationKinds = []string{
	rail0.PayoutDestinationKindExchange,
	rail0.PayoutDestinationKindCircleMint,
	rail0.PayoutDestinationKindSelfCustody,
	rail0.PayoutDestinationKindProvider,
}

// ── payouts destinations [list] ──────────────────────────────────────────────────

var (
	payoutDestListWallet  string
	payoutDestListStatus  string
	payoutDestListSort    string
	payoutDestListPage    int
	payoutDestListPerPage int
)

var payoutDestinationsCmd = &cobra.Command{
	Use:   "destinations",
	Short: "Manage where payouts go (lists when run bare)",
	Long: `Manage the account's payout destinations — the deposit addresses at a venue of
your own (an exchange account, Circle Mint, another wallet you control) that a
wallet may pay out to. Run bare to list them.

A destination is bound to ONE chain and ONE token. An EVM address is the same
string on every chain, but a venue credits only the network it issued the address
for: USDC sent on another chain to that address is usually lost. Generate the
deposit address at the venue WITH THE NETWORK SET to the destination's chain.

Its lifecycle:

  1. 'add'       register it — you confirm you checked the details at the venue.
                 It is born "unverified".
  2. proof       'rail0 payouts send --destination <id> --proof' sends a small
                 transfer (1 token unit, raised to the minimum).
  3. 'confirm'   once the venue shows the proof CREDITED: the destination
                 becomes "verified" and takes payouts of any amount.

Minimum: a venue may silently ignore a deposit below its minimum, and that deposit
can be lost for good. Every destination states it: --min-amount with the venue's
minimum, or --no-minimum when the venue has none. Payouts below it are refused, and
the proof is raised to it. Changing the minimum asks for the details confirmation
again, and raising it above the latest proof takes the verification away (the
destination needs a new proof). A destination registered before the minimum was
asked shows it as "not set" (the "minimum" field of list and get): state it with
'update'.

Refund reserve: a full-balance payout ('send --amount all') leaves --reserve in the
wallet, so refunds still inside their window can be paid. Unset, the gateway keeps
its suggestion: the refundable balance of the wallet's payments in that token.

Every wallet of the account can READ every destination and disable one; only the
destination's own wallet may change or re-enable it. A deactivated account is
read-only: every change is refused (403 account_deactivated).`,
	Example: `  rail0 payouts destinations
  rail0 payouts destinations add -c arc -t USDC -A 0xVenueDeposit… -l "Exchange" --min-amount 10
  rail0 payouts destinations confirm <destination_id>
  rail0 payouts destinations events <destination_id>`,
	Args: cobra.NoArgs,
	RunE: runPayoutDestinationsList,
}

var payoutDestinationsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the account's payout destinations (every wallet's)",
	Example: `  rail0 payouts destinations list --status verified
  rail0 payouts destinations list --wallet <wallet_id>`,
	Args: cobra.NoArgs,
	RunE: runPayoutDestinationsList,
}

func runPayoutDestinationsList(_ *cobra.Command, _ []string) error {
	if _, err := statusFilterValues("--status", []string{payoutDestListStatus}, payoutDestinationStatuses); err != nil {
		return err
	}
	p := rail0.ListPayoutDestinationsParams{
		WalletID: payoutDestListWallet,
		Status:   payoutDestListStatus,
		Sort:     payoutDestListSort,
		Page:     payoutDestListPage,
		PerPage:  payoutDestListPerPage,
	}
	return ui.RunAPI(func() (any, error) {
		resp, err := newReadClient().PayoutDestinations.List(context.Background(), p)
		if err != nil {
			return nil, err
		}
		return viewPayoutDestinations(resp.Data), nil // bare array; pagination is in headers
	})
}

// addPayoutDestinationListFlags registers the listing flags on both spellings of the
// list (the bare group and 'list'), bound to the same variables.
func addPayoutDestinationListFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&payoutDestListWallet, "wallet", "", "Only this wallet's destinations (wallet id)")
	cmd.Flags().StringVar(&payoutDestListStatus, "status", "", "Filter by status: "+strings.Join(payoutDestinationStatuses, ", "))
	_ = cmd.RegisterFlagCompletionFunc("status", fixedCompletions(payoutDestinationStatuses...))
	cmd.Flags().StringVar(&payoutDestListSort, "sort", "", "Sort fields (e.g. -created_at)")
	cmd.Flags().IntVar(&payoutDestListPage, "page", 0, "Page number (1-based)")
	cmd.Flags().IntVar(&payoutDestListPerPage, "per-page", 0, "Destinations per page")
}

// ── payouts destinations get ──────────────────────────────────────────────────

var payoutDestinationsGetCmd = &cobra.Command{
	Use:     "get <destination_id>",
	Short:   "Fetch one payout destination",
	Example: `  rail0 payouts destinations get <destination_id>`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			d, err := newReadClient().PayoutDestinations.Get(context.Background(), args[0])
			return viewPayoutDestination(d), err
		})
	},
}

// ── payouts destinations add ──────────────────────────────────────────────────

var (
	payoutDestAddWallet  string
	payoutDestAddChainID int
	payoutDestAddToken   string
	payoutDestAddAddress string
	payoutDestAddLabel   string
	payoutDestAddKind    string
	payoutDestAddReserve string
	payoutDestAddMin     string
	payoutDestAddNoMin   bool
)

var payoutDestinationsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Register a payout destination (asks you to confirm the details)",
	Long: `Register a deposit address as a payout destination of the session's wallet.

Before running it, open the venue's deposit page with the NETWORK set to --chain
and the asset set to --token, and copy the address and the minimum deposit from
there.

The minimum is required, and stated explicitly: --min-amount <decimal> (the
figure; 0 is an ordinary figure), or --no-minimum when the venue has none — never
both. On a terminal, leaving both out prompts for it (an empty answer is not
accepted: type "none" if the venue has none); without a terminal or with --agent
it is a usage error, before any request.

The command shows what it will register and the details statement, and you must
accept it: a y/N prompt (default no) on a terminal, or --confirm-details when
there is no terminal or with --agent. Without it nothing is sent. --yes does not
accept it for you.

--token is a symbol (USDC) resolved on --chain, or the token's 0x address;
--chain and --token fall back to the configured defaults ('rail0 config').
--min-amount and --reserve are decimals in token units (10.00), never negative,
converted with the token's decimals. --wallet defaults to the wallet you are
signed in with, and must be it.

The destination is born "unverified". Next: send the proof with
'rail0 payouts send --destination <id> --proof', and once the venue shows it
credited, 'rail0 payouts destinations confirm <id>'.

Refused (422): the zero address, the wallet itself, a contract the gateway knows on
that chain, no minimum stated (minimum_required), both a figure and --no-minimum
(minimum_conflict), or no confirmation; 409 when the wallet already has this
address for this token and chain.`,
	Example: `  rail0 payouts destinations add -c arc -t USDC -A 0xVenueDeposit… \
    -l "Exchange USDC (Arc)" --min-amount 10

  # non-interactive: state the minimum and accept the statement explicitly
  rail0 payouts destinations add -c arc -t USDC -A 0xVenueDeposit… --no-minimum --confirm-details`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		// The minimum is checked first: refused without a terminal before anything is
		// read, prompted for on one once the token (and so its decimals) is known.
		noMinimum := payoutDestAddNoMin
		minArg := payoutDestAddMin
		if noMinimum && minArg != "" {
			return errPayoutMinimumConflict()
		}
		if err := checkNotNegative("--min-amount", minArg); err != nil {
			return err
		}
		minGiven := noMinimum || minArg != ""
		if !minGiven && !interactive() {
			return errPayoutMinimumMissing("the venue's minimum deposit is required")
		}
		wallet := payoutDestAddWallet
		if wallet == "" {
			var err error
			if wallet, err = sessionAddress(); err != nil {
				return err
			}
		}
		chainID := payoutDestAddChainID
		if chainID == 0 {
			id, _, ok, err := configuredChainID(ctx)
			if err != nil {
				return err
			}
			if !ok {
				return errChainRequired()
			}
			chainID = id
		}
		tokenArg := payoutDestAddToken
		if tokenArg == "" {
			tokenArg = configuredDefault("token")
		}
		if tokenArg == "" {
			return errors.New("--token is required (a symbol such as USDC, or the token's 0x address)")
		}
		if payoutDestAddAddress == "" {
			return errors.New("--address is required: the deposit address copied from the venue")
		}
		if err := validateAddress(payoutDestAddAddress); err != nil {
			return err
		}
		address, err := resolveAlias(payoutDestAddAddress)
		if err != nil {
			return err
		}
		tokenAddress, symbol := tokenArg, ""
		if !strings.HasPrefix(tokenArg, "0x") {
			if tokenAddress, _, _, err = lookupTokenAddress(ctx, chainID, tokenArg); err != nil {
				return err
			}
			symbol = strings.ToUpper(tokenArg)
		}
		if !minGiven {
			answer, err := promptPayoutMinimum(ctx, chainID, tokenAddress, symbol)
			if err != nil {
				return err
			}
			if strings.EqualFold(strings.TrimSpace(answer), payoutNoMinimumAnswer) {
				noMinimum = true
			} else {
				minArg = answer
			}
		}
		minUnits, err := payoutAmountUnits(ctx, "--min-amount", minArg, chainID, tokenAddress)
		if err != nil {
			return err
		}
		reserveUnits, err := payoutAmountUnits(ctx, "--reserve", payoutDestAddReserve, chainID, tokenAddress)
		if err != nil {
			return err
		}
		reserve := payoutAmountLabel(ctx, reserveUnits, chainID, tokenAddress, symbol)
		if reserveUnits == "" {
			reserve = "the gateway's suggestion (refundable balance still in its window)"
		}
		summary := fmt.Sprintf("Register this payout destination?\n"+
			"  deposit address  %s\n"+
			"  network          %s\n"+
			"  token            %s\n"+
			"  minimum deposit  %s\n"+
			"  refund reserve   %s\n"+
			"  from wallet      %s",
			address, chainLabel(chainID), tokenLabel(symbol, tokenAddress),
			payoutMinimumLabel(ctx, minUnits, noMinimum, chainID, tokenAddress, symbol), reserve, wallet)
		if err := confirmPayoutDetails(summary); err != nil {
			return err
		}
		req := rail0.CreatePayoutDestinationRequest{
			WalletID:       wallet,
			ChainID:        chainID,
			Token:          rail0.Address(tokenAddress),
			Address:        rail0.Address(address),
			Label:          payoutDestAddLabel,
			Kind:           payoutDestAddKind,
			ReserveAmount:  reserveUnits,
			NoMinimum:      noMinimum,
			ConfirmDetails: true, // reached only past confirmPayoutDetails
		}
		if !noMinimum {
			m := rail0.Uint256String(minUnits)
			req.MinAmount = &m
		}
		var created *rail0.PayoutDestination
		err = ui.RunAPI(func() (any, error) {
			d, err := newClient().PayoutDestinations.Create(ctx, req)
			created = d
			return viewPayoutDestination(d), err
		})
		if err != nil {
			return err
		}
		if !ui.IsQuiet() && created != nil {
			ui.Notice("hint", map[string]any{"destination_id": created.ID},
				fmt.Sprintf("\nThe destination is unverified. Send the proof, then confirm it once the venue shows it credited:\n"+
					"  rail0 payouts send --destination %s --proof -p @<wallet-key>\n"+
					"  rail0 payouts destinations confirm %s", created.ID, created.ID))
		}
		return nil
	},
}

// promptPayoutMinimum asks, on a terminal, for the venue's minimum deposit 'add' was
// not given. An empty answer is refused inline — the point of the field is that the
// merchant states it — and "none" is the answer for a venue with none (the flag
// no_minimum; "0" is an ordinary figure). The value is
// validated with the token's decimals before the prompt closes.
func promptPayoutMinimum(ctx context.Context, chainID int, token, symbol string) (string, error) {
	return promptValidated("Minimum deposit",
		fmt.Sprintf("The venue's minimum deposit for %s on %s, from its deposit page, in token units (e.g. 10.00) — or %q if the venue has none",
			tokenLabel(symbol, token), chainLabel(chainID), payoutNoMinimumAnswer),
		false, func(s string) error {
			if strings.EqualFold(strings.TrimSpace(s), payoutNoMinimumAnswer) {
				return nil
			}
			_, err := payoutAmountUnits(ctx, "minimum deposit", s, chainID, token)
			return err
		})
}

// tokenLabel renders "USDC (0x36…)" when the symbol is known, else the bare address.
func tokenLabel(symbol, address string) string {
	if symbol == "" {
		return address
	}
	return symbol + " (" + address + ")"
}

// ── payouts destinations update ───────────────────────────────────────────────

var (
	payoutDestUpdateLabel   string
	payoutDestUpdateReserve string
	payoutDestUpdateMin     string
	payoutDestUpdateNoMin   bool
)

var payoutDestinationsUpdateCmd = &cobra.Command{
	Use:   "update <destination_id>",
	Short: "Change a destination's label, refund reserve or minimum",
	Long: `Change a payout destination's label, refund reserve or minimum deposit. Only the
destination's own wallet may.

--reserve and --min-amount are decimals in token units, converted with the
destination token's decimals. --reserve "" restores the gateway's suggestion. The
minimum cannot be blanked: --min-amount "" is refused — to state that the venue has
no minimum, pass --no-minimum. --min-amount and --no-minimum exclude each other. A
destination registered before the minimum was asked shows it as "not set": state it
here.

CHANGING the minimum (a new figure, or --no-minimum on one that had a figure)
asks for the details confirmation again, exactly like 'add' (a y/N prompt on a
terminal, or --confirm-details; --yes does not count): the minimum is copied from
the venue, and a wrong one is how a deposit gets lost.
Re-sending the current value asks nothing. Raising it above the destination's
latest proof takes the verification away: the destination goes back to
"unverified" and needs a new proof at the new minimum.

To switch a destination off or on, use 'disable' / 'enable'.`,
	Example: `  rail0 payouts destinations update <destination_id> --label "Exchange (main)"
  rail0 payouts destinations update <destination_id> --reserve 50
  rail0 payouts destinations update <destination_id> --min-amount 25 --confirm-details
  rail0 payouts destinations update <destination_id> --no-minimum --confirm-details`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		flags := cmd.Flags()
		var req rail0.UpdatePayoutDestinationRequest
		if flags.Changed("label") {
			req.Label = &payoutDestUpdateLabel
		}
		noMinimum := payoutDestUpdateNoMin
		if flags.Changed("min-amount") && noMinimum {
			return errPayoutMinimumConflict()
		}
		if flags.Changed("min-amount") && strings.TrimSpace(payoutDestUpdateMin) == "" {
			return errPayoutMinimumMissing(`--min-amount may not be blank`)
		}
		if err := checkNotNegative("--min-amount", payoutDestUpdateMin); err != nil {
			return err
		}
		changesMin := flags.Changed("min-amount") || noMinimum
		changesAmounts := flags.Changed("reserve") || changesMin
		if req.Label == nil && !changesAmounts {
			return newUsageError("usage_error", "nothing to update: pass --label, --reserve, --min-amount or --no-minimum")
		}
		client := newClient()
		if changesAmounts {
			// The amounts are decimals of the destination's token, and only the stored
			// minimum says whether --min-amount CHANGES it — so read the destination first.
			dest, err := client.PayoutDestinations.Get(ctx, args[0])
			if err != nil {
				return err
			}
			if flags.Changed("reserve") {
				units, err := payoutAmountUnits(ctx, "--reserve", payoutDestUpdateReserve, dest.ChainID, string(dest.Token))
				if err != nil {
					return err
				}
				req.ReserveAmount = &units
			}
			if changesMin {
				// A change is a different [figure, no_minimum] pair — what the gateway
				// compares to ask for confirm_details; re-sending the current one is not.
				units := ""
				if noMinimum {
					req.NoMinimum = true
				} else {
					if units, err = payoutAmountUnits(ctx, "--min-amount", payoutDestUpdateMin, dest.ChainID, string(dest.Token)); err != nil {
						return err
					}
					m := rail0.Uint256String(units)
					req.MinAmount = &m
				}
				if noMinimum != dest.NoMinimum || units != string(dest.MinAmount) {
					if err := confirmPayoutDetails(minChangeSummary(ctx, dest, units, noMinimum)); err != nil {
						return err
					}
					req.ConfirmDetails = true // reached only past confirmPayoutDetails
				}
			}
		}
		return ui.RunAPI(func() (any, error) {
			d, err := client.PayoutDestinations.Update(ctx, args[0], req)
			return viewPayoutDestination(d), err
		})
	},
}

// minChangeSummary is the prompt for a minimum change: the destination it applies to
// (so the merchant re-checks the right venue page) and the old and new minimum.
func minChangeSummary(ctx context.Context, d *rail0.PayoutDestination, units string, none bool) string {
	token := string(d.Token)
	return fmt.Sprintf("Change the minimum deposit of this payout destination?\n"+
		"  deposit address  %s\n"+
		"  network          %s\n"+
		"  token            %s\n"+
		"  minimum deposit  %s → %s",
		d.Address, chainLabel(d.ChainID), tokenLabel(d.TokenSymbol, token),
		payoutMinimumLabel(ctx, string(d.MinAmount), d.NoMinimum, d.ChainID, token, d.TokenSymbol),
		payoutMinimumLabel(ctx, units, none, d.ChainID, token, d.TokenSymbol))
}

// ── payouts destinations disable / enable ─────────────────────────────────────

var payoutDestinationsDisableCmd = &cobra.Command{
	Use:   "disable <destination_id>",
	Short: "Switch a destination off (reversible)",
	Long: `Switch a payout destination off: nothing can be prepared to it until it is
enabled again. Any wallet of the account may disable it.

Its pending payout is settled first: an unsigned one fails "cancelled"; a SIGNED
one refuses the switch-off (409 payout_already_signed), because its transaction
may still be mined — resend it or let it settle, then disable.`,
	Example: `  rail0 payouts destinations disable <destination_id>`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setPayoutDestinationActive(args[0], false)
	},
}

var payoutDestinationsEnableCmd = &cobra.Command{
	Use:   "enable <destination_id>",
	Short: "Switch a disabled destination back on",
	Long: `Switch a disabled payout destination back on. It gets back the verification it
had: a verified destination stays verified. Only the destination's own wallet may.`,
	Example: `  rail0 payouts destinations enable <destination_id>`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setPayoutDestinationActive(args[0], true)
	},
}

func setPayoutDestinationActive(id string, active bool) error {
	req := rail0.UpdatePayoutDestinationRequest{Active: &active}
	return ui.RunAPI(func() (any, error) {
		d, err := newClient().PayoutDestinations.Update(context.Background(), id, req)
		return viewPayoutDestination(d), err
	})
}

// ── payouts destinations confirm ──────────────────────────────────────────────

var payoutDestinationsConfirmCmd = &cobra.Command{
	Use:   "confirm <destination_id>",
	Short: "Attest the venue credited the proof: the destination becomes verified",
	Long: `Record that the venue CREDITED the proof transfer to your account there. The
destination becomes "verified" and accepts payouts of any amount.

Confirm only after you see the deposit at the venue — a transfer mined on-chain is
not the same as a deposit credited: a wrong network, a missing memo or an amount
below the venue's minimum can all be mined and never credited. The proof must be
confirmed on-chain first (422 payout_destination_proof_missing otherwise:
'rail0 payouts list --destination <id> --purpose proof' shows it).

You are asked to confirm (y/N); pass --yes to skip the prompt in a script.`,
	Example: `  rail0 payouts destinations confirm <destination_id>`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := confirmDestructive(fmt.Sprintf(
			"Has the venue credited the proof transfer for destination %s to your account there?", args[0])); err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			d, err := newClient().PayoutDestinations.Confirm(context.Background(), args[0])
			return viewPayoutDestination(d), err
		})
	},
}

// ── payouts destinations events ───────────────────────────────────────────────

var payoutDestEventsPage, payoutDestEventsPerPage int

var payoutDestinationsEventsCmd = &cobra.Command{
	Use:   "events <destination_id>",
	Short: "A destination's audit trail (its events and its payouts'), newest first",
	Long: `List a payout destination's audit trail: its own events (created, minimum
changed — with the details confirmation recorded —, verified, disabled, …) AND those
of every payout made to it, newest first.`,
	Example: `  rail0 payouts destinations events <destination_id> --per-page 50`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := rail0.ListPayoutEventsParams{Page: payoutDestEventsPage, PerPage: payoutDestEventsPerPage}
		return ui.RunAPI(func() (any, error) {
			resp, err := newReadClient().PayoutDestinations.Events(context.Background(), args[0], p)
			if err != nil {
				return nil, err
			}
			return resp.Data, nil
		})
	},
}

func init() {
	addPayoutDestinationListFlags(payoutDestinationsCmd)
	addPayoutDestinationListFlags(payoutDestinationsListCmd)

	f := payoutDestinationsAddCmd.Flags()
	f.StringVar(&payoutDestAddWallet, "wallet", "", "Wallet that will send to it, id or address (default: the signed-in wallet)")
	addChainFlag(payoutDestinationsAddCmd, &payoutDestAddChainID, "c", "Chain the deposit address was issued for (default: 'rail0 config' chain)")
	f.StringVarP(&payoutDestAddToken, "token", "t", "", "Token symbol (USDC) or 0x address on that chain (default: 'rail0 config' token)")
	f.StringVarP(&payoutDestAddAddress, "address", "A", "", "The venue's deposit address (0x…, or @alias)")
	f.StringVarP(&payoutDestAddLabel, "label", "l", "", "Human-readable label")
	f.StringVar(&payoutDestAddKind, "kind", "", "Informational: "+strings.Join(payoutDestinationKinds, ", ")+" (default exchange)")
	f.StringVar(&payoutDestAddMin, "min-amount", "", "The venue's minimum deposit, decimal in token units (e.g. 10.00) — required, or --no-minimum")
	f.BoolVar(&payoutDestAddNoMin, "no-minimum", false, "The venue has no minimum deposit (no_minimum: true); excludes --min-amount")
	f.StringVar(&payoutDestAddReserve, "reserve", "", "Refund reserve left in the wallet on '--amount all', decimal in token units")
	_ = payoutDestinationsAddCmd.RegisterFlagCompletionFunc("kind", fixedCompletions(payoutDestinationKinds...))
	_ = payoutDestinationsAddCmd.RegisterFlagCompletionFunc("address", completeAliasNames)
	addConfirmDetailsFlag(payoutDestinationsAddCmd)

	u := payoutDestinationsUpdateCmd.Flags()
	u.StringVarP(&payoutDestUpdateLabel, "label", "l", "", "New label")
	u.StringVar(&payoutDestUpdateReserve, "reserve", "", "New refund reserve, decimal in token units (\"\" restores the gateway's suggestion)")
	u.StringVar(&payoutDestUpdateMin, "min-amount", "", "New minimum deposit, decimal in token units; asks for the details confirmation")
	u.BoolVar(&payoutDestUpdateNoMin, "no-minimum", false, "The venue has no minimum deposit (no_minimum: true; removes a figure); asks for the details confirmation")
	addConfirmDetailsFlag(payoutDestinationsUpdateCmd)

	addYesFlag(payoutDestinationsConfirmCmd)

	payoutDestinationsEventsCmd.Flags().IntVar(&payoutDestEventsPage, "page", 0, "Page number (1-based)")
	payoutDestinationsEventsCmd.Flags().IntVar(&payoutDestEventsPerPage, "per-page", 0, "Events per page (max 100)")

	payoutDestinationsCmd.AddCommand(
		payoutDestinationsListCmd,
		payoutDestinationsGetCmd,
		payoutDestinationsAddCmd,
		payoutDestinationsUpdateCmd,
		payoutDestinationsDisableCmd,
		payoutDestinationsEnableCmd,
		payoutDestinationsConfirmCmd,
		payoutDestinationsEventsCmd,
	)
}
