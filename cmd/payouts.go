package cmd

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// payoutsCmd is the merchant cash-out: moving a wallet's tokens to a registered
// destination. The gateway builds an unsigned ERC-20 transfer and the CLI signs it
// locally; nothing here holds funds or keys. Run bare to list. Sending lives in
// payouts_send.go, destinations in payout_destinations.go.

var payoutStatuses = []string{
	rail0.PayoutStatusPending,
	rail0.PayoutStatusSubmitted,
	rail0.PayoutStatusConfirmed,
	rail0.PayoutStatusFailed,
}

var payoutPurposes = []string{rail0.PayoutPurposePayout, rail0.PayoutPurposeProof}

// ── payouts [list] ───────────────────────────────────────────────────────────

var (
	payoutsListDestination string
	payoutsListWallet      string
	payoutsListStatuses    []string
	payoutsListPurpose     string
	payoutsListCreatedFrom string
	payoutsListCreatedTo   string
	payoutsListSort        string
	payoutsListPage        int
	payoutsListPerPage     int
)

var payoutsCmd = &cobra.Command{
	Use:   "payouts",
	Short: "Send a wallet's tokens to your own venue (lists when run bare; requires login)",
	Long: `Pay out: move a merchant wallet's tokens to a deposit address at a venue of your
own — an exchange account, Circle Mint, another wallet you control. Run bare to
list the account's payouts.

The gateway does the on-chain leg and nothing else. It builds an unsigned ERC-20
transfer from the wallet to a registered destination; this CLI checks it is
EXACTLY that transfer (token, recipient, amount, chain, no value) and signs it
locally with your key. The gateway never holds funds or keys and never talks to
the venue.

First payout to a new venue:
  1. payouts destinations add -c <chain> -t USDC -A 0xDeposit… --min-amount <min>
       register the deposit address; you confirm you checked the details
  2. payouts send --destination <id> --proof -p @merchant-key
       a small proof transfer (1 token unit, raised to the minimum)
  3. payouts destinations confirm <id>
       once the venue shows the proof CREDITED: the destination is verified
  4. payouts send --destination <id> --amount 250.00 -p @merchant-key
       or --amount all: the balance minus the refund reserve

Minimum: a deposit below the venue's minimum may be ignored by the venue and lost
for good. Set it on the destination (--min-amount): payouts below it are refused.

Refund reserve: '--amount all' keeps the destination's reserve in the wallet, so
refunds still inside their window can be paid (default: the gateway's suggestion,
the refundable balance of the wallet's payments in that token).

A payout goes pending → submitted → confirmed (or failed: its error_code says
why). One pending payout per destination. A deactivated account is read-only: it
cannot prepare, send or change payouts or destinations (403 account_deactivated).

Every read covers the whole account; writes act for the wallet you sign in with,
which must be the destination's.`,
	Example: `  rail0 payouts
  rail0 payouts --status submitted,confirmed --purpose payout
  rail0 payouts send --destination <destination_id> --amount 100.00 -p @merchant-key
  rail0 payouts get <payout_id>`,
	Args: cobra.NoArgs,
	RunE: runPayoutsList,
}

var payoutsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the account's payouts, newest first",
	Long: `List the account's payouts, newest first. --status takes several values, repeated
or comma-separated, and matches any of them.`,
	Example: `  rail0 payouts list --destination <destination_id>
  rail0 payouts list --status failed --created-from 2026-10-01T00:00:00Z`,
	Args: cobra.NoArgs,
	RunE: runPayoutsList,
}

func runPayoutsList(_ *cobra.Command, _ []string) error {
	statuses, err := statusFilterValues("--status", payoutsListStatuses, payoutStatuses)
	if err != nil {
		return err
	}
	if _, err := statusFilterValues("--purpose", []string{payoutsListPurpose}, payoutPurposes); err != nil {
		return err
	}
	p := rail0.ListPayoutsParams{
		DestinationID: payoutsListDestination,
		WalletID:      payoutsListWallet,
		Statuses:      statuses,
		Purpose:       payoutsListPurpose,
		CreatedFrom:   payoutsListCreatedFrom,
		CreatedTo:     payoutsListCreatedTo,
		Sort:          payoutsListSort,
		Page:          payoutsListPage,
		PerPage:       payoutsListPerPage,
	}
	return ui.RunAPI(func() (any, error) {
		resp, err := newReadClient().Payouts.List(context.Background(), p)
		if err != nil {
			return nil, err
		}
		return resp.Data, nil // bare array; pagination is in headers
	})
}

// addPayoutListFlags registers the listing flags on both spellings of the list (the
// bare group and 'list'), bound to the same variables.
func addPayoutListFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVar(&payoutsListDestination, "destination", "", "Only payouts to this destination (destination id)")
	f.StringVar(&payoutsListWallet, "wallet", "", "Only payouts from this wallet (wallet id)")
	f.StringSliceVar(&payoutsListStatuses, "status", nil,
		"Filter by status ("+strings.Join(payoutStatuses, ", ")+"); repeat or comma-separate for any of several")
	f.StringVar(&payoutsListPurpose, "purpose", "", "Filter by purpose: payout or proof")
	f.StringVar(&payoutsListCreatedFrom, "created-from", "", "Only payouts created at/after this ISO-8601 time")
	f.StringVar(&payoutsListCreatedTo, "created-to", "", "Only payouts created at/before this ISO-8601 time")
	f.StringVar(&payoutsListSort, "sort", "", "Sort fields (e.g. -created_at)")
	f.IntVar(&payoutsListPage, "page", 0, "Page number (1-based)")
	f.IntVar(&payoutsListPerPage, "per-page", 0, "Payouts per page (max 100)")
	_ = cmd.RegisterFlagCompletionFunc("status", fixedListCompletions(payoutStatuses...))
	_ = cmd.RegisterFlagCompletionFunc("purpose", fixedCompletions(payoutPurposes...))
}

// ── payouts get / events ─────────────────────────────────────────────────────

var payoutsGetCmd = &cobra.Command{
	Use:   "get <payout_id>",
	Short: "Fetch one payout",
	Long: `Fetch one payout: its status, amount (base units and amount_formatted), the
transaction hash once signed, and — when failed — error_code: reverted,
foreign_transfer, dropped, submission_timeout or cancelled. submission_timeout and
dropped are not final: 'report-hash' with the transaction that did go out puts the
payout back to submitted.`,
	Example: `  rail0 payouts get <payout_id>`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newReadClient().Payouts.Get(context.Background(), args[0])
		})
	},
}

var payoutEventsPage, payoutEventsPerPage int

var payoutsEventsCmd = &cobra.Command{
	Use:     "events <payout_id>",
	Short:   "A payout's audit trail, newest first",
	Example: `  rail0 payouts events <payout_id>`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := rail0.ListPayoutEventsParams{Page: payoutEventsPage, PerPage: payoutEventsPerPage}
		return ui.RunAPI(func() (any, error) {
			resp, err := newReadClient().Payouts.Events(context.Background(), args[0], p)
			if err != nil {
				return nil, err
			}
			return resp.Data, nil
		})
	},
}

// ── payouts resend / cancel / report-hash ────────────────────────────────────

var payoutsResendCmd = &cobra.Command{
	Use:   "resend <payout_id>",
	Short: "Re-broadcast the signed transfer a submit stored",
	Long: `Re-broadcast the signed transfer the gateway stored on submit, after the node
refused it or did not answer. The same bytes, so the same nonce and hash: it can be
mined once at most, and resending is always safe. Nothing is signed again.

422 payout_not_signed when nothing is stored: sign it with 'payouts submit <id>'.

-w/--wait follows the payout until confirmed or failed, as in 'payouts send'.`,
	Example: `  rail0 payouts resend <payout_id>
  rail0 payouts resend <payout_id> --wait`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		client := newClient()
		var sent *rail0.Payout
		err := ui.RunAPIWithMessage(actionLabel("Resending…"), func() (any, error) {
			p, err := client.Payouts.Resend(ctx, args[0])
			if err != nil {
				return nil, err
			}
			sent = p
			return submittedPayoutThenWait(ctx, client, p, &sent)
		})
		if err != nil {
			return err
		}
		if waitFlag {
			notePayoutSent(sent)
		}
		return nil
	},
}

var payoutsCancelCmd = &cobra.Command{
	Use:   "cancel <payout_id>",
	Short: "Cancel a pending payout that was never signed",
	Long: `Cancel a pending payout the wallet has not signed: it fails with error_code
"cancelled" and its destination is free for a new one. Nothing moved, so nothing
is lost.

A SIGNED payout cannot be cancelled (409 payout_already_signed): its transaction
may still be mined. Resend it and let it settle.`,
	Example: `  rail0 payouts cancel <payout_id>`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunAPI(func() (any, error) {
			return newClient().Payouts.Cancel(context.Background(), args[0])
		})
	},
}

// txHashPattern is a transaction hash: 0x + 32 bytes of hex.
var txHashPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`)

var payoutsReportHashCmd = &cobra.Command{
	Use:   "report-hash <payout_id> <transaction_hash>",
	Short: "Report a transfer you broadcast yourself",
	Long: `Tell the gateway the hash of a payout transfer you signed and broadcast yourself
(a wallet that signs and sends in one step), instead of 'payouts submit'.

The hash must be THIS transfer — from the wallet, to the token, this amount to
this destination: a hash a node already knows that is anything else is refused
(foreign_transfer_hash), and one another payout records is 409 payout_hash_in_use.
A hash no node has seen yet is accepted and settled from its receipt.

Also the way back for a payout that failed submission_timeout or dropped because
the transaction that went out was sped up or replaced: report its hash and the
payout returns to submitted.`,
	Example: `  rail0 payouts report-hash <payout_id> 0x5c50…`,
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !txHashPattern.MatchString(args[1]) {
			return fmt.Errorf("invalid transaction hash %q: expected 0x followed by 64 hex characters", args[1])
		}
		return ui.RunAPI(func() (any, error) {
			return newClient().Payouts.Submitted(context.Background(), args[0], args[1])
		})
	},
}

func init() {
	addPayoutListFlags(payoutsCmd)
	addPayoutListFlags(payoutsListCmd)

	addWaitFlags(payoutsResendCmd)

	payoutsEventsCmd.Flags().IntVar(&payoutEventsPage, "page", 0, "Page number (1-based)")
	payoutsEventsCmd.Flags().IntVar(&payoutEventsPerPage, "per-page", 0, "Events per page (max 100)")

	payoutsCmd.AddCommand(
		payoutsListCmd,
		payoutsGetCmd,
		payoutsSendCmd,
		payoutsPrepareCmd,
		payoutsSubmitCmd,
		payoutsResendCmd,
		payoutsCancelCmd,
		payoutsReportHashCmd,
		payoutsEventsCmd,
		payoutDestinationsCmd,
	)
}
