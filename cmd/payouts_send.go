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

// The payout commands that sign (send, submit) and the one that only prepares. Every
// signature goes through the SDK's payout signer (PrepareAndSubmit / SignAndSubmit),
// which checks the gateway-built transaction is exactly transfer(destination, amount)
// on the destination's token and chain, value 0, before the key touches it — there is
// no path here that signs a payout transaction unchecked.

var (
	payoutSendDestination string
	payoutSendAmount      string
	payoutSendProof       bool
	payoutSendKey         string
	payoutSubmitKey       string
)

// ── payouts send ─────────────────────────────────────────────────────────────

var payoutsSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Prepare, verify, sign and submit a payout in one step",
	Long: `Pay out to a destination: the gateway prepares the ERC-20 transfer, the CLI
checks it is exactly that transfer, signs it locally with -p and hands it to the
gateway, which broadcasts it. The result is the payout, "submitted" with its
transaction hash; it turns "confirmed" once mined ('payouts get <id>').

--amount is a decimal in token units (250.00), or "all": the wallet's balance minus
the destination's refund reserve, computed by the gateway. --proof sends the proof
transfer instead (1 token unit, raised to the destination's minimum) — the step
before 'payouts destinations confirm'. An unverified destination takes the proof
only.

Before signing, the command shows what will be sent — amount, token, deposit
address, network, from — and asks to confirm (y/N); --yes skips the prompt and is
required without a terminal. With "all" or --proof the gateway sets the amount: to
see the exact figure before signing, run 'payouts prepare' and then
'payouts submit <id>', which shows it.

The key is the destination's wallet's: the CLI signs in with it. Given as a raw 0x
hex key, as @name for one saved with 'rail0 keys add', through RAIL0_PRIVATE_KEY,
or omitted to be prompted for (hidden).

Refused: payout_below_minimum (below the destination's minimum),
insufficient_balance, payout_destination_unverified (send the proof and confirm
first), payout_destination_disabled, payout_already_signed (a signed payout is
pending: 'payouts resend <id>'). A node refusal (502) leaves the payout pending and
signed: 'payouts resend <id>'.`,
	Example: `  rail0 payouts send --destination <destination_id> --proof -p @merchant-key
  rail0 payouts send --destination <destination_id> --amount 250.00 -p @merchant-key
  rail0 payouts send --destination <destination_id> --amount all -p @merchant-key --yes`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		req, err := payoutPrepareRequest()
		if err != nil {
			return err
		}
		keyHex, err := signingKeyOrPrompt(payoutSendKey)
		if err != nil {
			return err
		}
		signer, err := addressFromKey(keyHex)
		if err != nil {
			return err
		}
		privKey, err := rail0.HexToPrivateKey(keyHex)
		if err != nil {
			return fmt.Errorf("invalid private key: %w", err)
		}
		ctx := context.Background()
		client := newClient()
		// Payouts are the SESSION wallet's: the gateway prepares from it and accepts
		// only its signature. Signing in with the key that will sign makes the two the
		// same wallet (a no-op when already signed in as it).
		if err := ensureSession(ctx, client, keyHex, signer); err != nil {
			return err
		}
		dest, err := client.PayoutDestinations.Get(ctx, req.DestinationID)
		if err != nil {
			return err
		}
		if err := checkDestinationSendable(dest, req.Purpose); err != nil {
			return err
		}
		if req.Amount != "" && req.Amount != rail0.PayoutAmountAll {
			if decimals, ok := tokenDecimals(ctx, dest.ChainID, string(dest.Token)); ok {
				if err := assertAmountPrecision(req.Amount, decimals); err != nil {
					return err
				}
			}
		}
		if err := confirmDestructive(payoutSendPrompt(ctx, dest, req, signer)); err != nil {
			return err
		}
		var sent *rail0.Payout
		err = ui.RunAPIWithMessage("Sending payout…", func() (any, error) {
			p, err := client.Payouts.PrepareAndSubmit(ctx, req, privKey)
			if err != nil {
				return nil, payoutSignError(err)
			}
			sent = p
			return p, nil
		})
		if err != nil {
			return err
		}
		if !ui.IsQuiet() && sent != nil {
			ui.Notice("hint", map[string]any{"payout_id": sent.ID}, payoutSentHint(sent))
		}
		return nil
	},
}

// payoutPrepareRequest builds the prepare request from --destination, --amount and
// --proof (shared by send and prepare), refusing a bad combination before any request:
// a proof's amount is the gateway's, and a payout needs one — a decimal (checked
// syntactically here, against the token's decimals by the caller) or "all".
func payoutPrepareRequest() (rail0.PreparePayoutRequest, error) {
	req := rail0.PreparePayoutRequest{DestinationID: payoutSendDestination, Purpose: rail0.PayoutPurposePayout}
	switch {
	case payoutSendDestination == "":
		return req, newUsageError("usage_error", "--destination is required")
	case payoutSendProof && payoutSendAmount != "":
		return req, newUsageError("usage_error", "--proof sends the gateway's proof amount; drop --amount")
	case payoutSendProof:
		req.Purpose = rail0.PayoutPurposeProof
	case payoutSendAmount == "":
		return req, newUsageError("usage_error", "--amount is required: a decimal in token units, or 'all' (or --proof for the proof transfer)")
	case strings.EqualFold(payoutSendAmount, rail0.PayoutAmountAll):
		req.Amount = rail0.PayoutAmountAll
	default:
		if err := validateAmount(payoutSendAmount); err != nil {
			return req, err
		}
		req.Amount = payoutSendAmount
	}
	return req, nil
}

// checkDestinationSendable refuses, before the confirmation prompt, a send the gateway
// is certain to refuse: to a disabled destination, or a payout to one not yet verified.
// Asking "send 250 USDC?" and then failing on a state the destination already showed
// would make the user confirm something that was never going to happen.
func checkDestinationSendable(d *rail0.PayoutDestination, purpose string) error {
	switch {
	case d.Status == rail0.PayoutDestinationStatusDisabled:
		return fmt.Errorf("destination %s is disabled — enable it first ('rail0 payouts destinations enable %s')", d.ID, d.ID)
	case d.Status == rail0.PayoutDestinationStatusUnverified && purpose != rail0.PayoutPurposeProof:
		return fmt.Errorf("destination %s is not verified yet: send the proof ('--proof'), then "+
			"'rail0 payouts destinations confirm %s' once the venue shows it credited", d.ID, d.ID)
	}
	return nil
}

// payoutSendPrompt is the confirmation shown before a send signs anything: what moves,
// where to, on which network, from which wallet. The destination's details come from a
// fresh read of the destination, the same record the SDK then holds the transaction to.
//
// For "all" and a proof the gateway computes the amount at prepare time, so the prompt
// says how instead of a number — and names the reserve / minimum that number depends on.
func payoutSendPrompt(ctx context.Context, d *rail0.PayoutDestination, req rail0.PreparePayoutRequest, signer string) string {
	token := string(d.Token)
	symbol := d.TokenSymbol
	if symbol == "" {
		symbol = token
	}
	var amount string
	switch {
	case req.Purpose == rail0.PayoutPurposeProof:
		amount = "the proof amount (1 " + symbol + ", raised to the minimum deposit"
		if d.MinAmount != "" {
			amount += " of " + payoutAmountLabel(ctx, string(d.MinAmount), d.ChainID, token, d.TokenSymbol)
		}
		amount += ")"
	case req.Amount == rail0.PayoutAmountAll:
		reserve := string(d.ReserveAmount)
		if reserve == "" {
			reserve = string(d.SuggestedReserve)
		}
		amount = "ALL — the wallet's balance minus the refund reserve of " +
			payoutAmountLabel(ctx, reserve, d.ChainID, token, d.TokenSymbol)
	default:
		amount = req.Amount + " " + symbol
	}
	to := string(d.Address)
	if d.Label != "" {
		to += " (" + d.Label + ")"
	}
	return fmt.Sprintf("Send this payout? A transfer is final once mined.\n"+
		"  amount   %s\n"+
		"  token    %s\n"+
		"  to       %s\n"+
		"  network  %s\n"+
		"  from     %s",
		amount, tokenLabel(d.TokenSymbol, token), to, chainLabel(d.ChainID), signer)
}

// payoutSentHint is the next step after a submit: a proof needs the venue check and
// 'confirm'; a payout only needs watching until it is mined.
func payoutSentHint(p *rail0.Payout) string {
	if p.Purpose == rail0.PayoutPurposeProof {
		return fmt.Sprintf("\nProof submitted. Once 'rail0 payouts get %s' shows it confirmed AND the venue shows it credited:\n"+
			"  rail0 payouts destinations confirm %s", p.ID, p.DestinationID)
	}
	return fmt.Sprintf("\nSubmitted. Follow it with 'rail0 payouts get %s' (submitted → confirmed).", p.ID)
}

// payoutSignError keeps a gateway error as it is (the CLI's error layer words it and
// adds the hint) and drops the SDK's "rail0: " prefix from a local refusal, so it reads
// "refusing to sign: …" like every other pre-sign check of the CLI.
func payoutSignError(err error) error {
	var apiErr *rail0.APIError
	if errors.As(err, &apiErr) {
		return err
	}
	return withoutSDKPrefix(err)
}

// ── payouts prepare ──────────────────────────────────────────────────────────

var payoutsPrepareCmd = &cobra.Command{
	Use:   "prepare",
	Short: "Prepare a payout without signing it (shows the exact amount)",
	Long: `Ask the gateway to build a payout's unsigned transfer, and print the pending
payout: the exact amount (base units and amount_formatted) — the way to see what
"all" or a proof comes to — and unsigned_transaction, the EIP-1559 transaction to
sign. Nothing is signed or sent.

Then either 'payouts submit <id>' (verify, sign with -p, submit), or sign it with
a wallet of your own and report the hash with 'payouts report-hash <id> <hash>'.
Check unsigned_transaction before signing it elsewhere: 'to' must be the token,
'data' transfer(destination address, amount), value 0.

A prepare replaces the destination's pending payout while it is unsigned; leave it
and it stays pending until replaced or cancelled ('payouts cancel <id>'). The
session must be the destination's wallet ('rail0 auth login').`,
	Example: `  rail0 payouts prepare --destination <destination_id> --amount all
  rail0 payouts prepare --destination <destination_id> --proof`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		req, err := payoutPrepareRequest()
		if err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return newClient().Payouts.Prepare(context.Background(), req)
		})
	},
}

// ── payouts submit ───────────────────────────────────────────────────────────

var payoutsSubmitCmd = &cobra.Command{
	Use:   "submit <payout_id>",
	Short: "Verify, sign and submit a prepared payout",
	Long: `Sign a pending payout prepared earlier ('payouts prepare') and hand it to the
gateway, which broadcasts it.

The transaction is checked against the destination, read again from the gateway —
exactly transfer(destination address, payout amount) on the destination's token
and chain, value 0, from your key's wallet — and refused otherwise before the key
touches it. The command shows the amount, token, deposit address and network and
asks to confirm (y/N); --yes skips it and is required without a terminal.

Only a pending, unsigned payout can be signed. One already signed is 'resend's:
its transaction may already be in a mempool, and signing again would risk a second
transfer.`,
	Example: `  rail0 payouts submit <payout_id> -p @merchant-key`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		keyHex, err := signingKeyOrPrompt(payoutSubmitKey)
		if err != nil {
			return err
		}
		signer, err := addressFromKey(keyHex)
		if err != nil {
			return err
		}
		privKey, err := rail0.HexToPrivateKey(keyHex)
		if err != nil {
			return fmt.Errorf("invalid private key: %w", err)
		}
		ctx := context.Background()
		client := newClient()
		if err := ensureSession(ctx, client, keyHex, signer); err != nil {
			return err
		}
		payout, err := client.Payouts.Get(ctx, args[0])
		if err != nil {
			return err
		}
		switch {
		case payout.Signed:
			return fmt.Errorf("payout %s is already signed: re-broadcast it with 'rail0 payouts resend %s'", payout.ID, payout.ID)
		case payout.Status != rail0.PayoutStatusPending:
			return fmt.Errorf("payout %s is %s, not pending: there is nothing to sign", payout.ID, payout.Status)
		}
		if err := confirmDestructive(payoutSubmitPrompt(payout, signer)); err != nil {
			return err
		}
		var sent *rail0.Payout
		err = ui.RunAPIWithMessage("Sending payout…", func() (any, error) {
			p, err := client.Payouts.SignAndSubmit(ctx, payout, privKey)
			if err != nil {
				return nil, payoutSignError(err)
			}
			sent = p
			return p, nil
		})
		if err != nil {
			return err
		}
		if !ui.IsQuiet() && sent != nil {
			ui.Notice("hint", map[string]any{"payout_id": sent.ID}, payoutSentHint(sent))
		}
		return nil
	},
}

// payoutSubmitPrompt is the confirmation before signing an existing payout, from the
// payout's own decoded intent. The SDK then holds the transaction to the destination
// as read again from the gateway, so a payout whose fields disagree with its
// destination is refused after this prompt, before signing.
func payoutSubmitPrompt(p *rail0.Payout, signer string) string {
	amount := p.AmountFormatted
	if amount == "" {
		amount = string(p.Amount) + " base units"
	}
	if p.TokenSymbol != "" {
		amount += " " + p.TokenSymbol
	}
	what := "payout"
	if p.Purpose == rail0.PayoutPurposeProof {
		what = "proof transfer"
	}
	return fmt.Sprintf("Sign and send this %s? A transfer is final once mined.\n"+
		"  amount   %s\n"+
		"  token    %s\n"+
		"  to       %s\n"+
		"  network  %s\n"+
		"  from     %s",
		what, amount, tokenLabel(p.TokenSymbol, string(p.Token)), p.Address, chainLabel(p.ChainID), signer)
}

// addPayoutKeyFlags registers -p/--private-key (and the hidden --pk) for a payout
// signing command. Not addSigningFlags: that also registers --idempotency-key, which
// the payout routes do not take (one pending payout per destination already makes a
// re-run replace or refuse, never duplicate).
func addPayoutKeyFlags(cmd *cobra.Command, key *string) {
	cmd.Flags().StringVarP(key, "private-key", "p", "",
		"The destination wallet's key: 0x hex, @name for an OS-keychain key ('rail0 keys add'), or RAIL0_PRIVATE_KEY")
	cmd.Flags().StringVar(key, "pk", "", "Alias for --private-key")
	_ = cmd.Flags().MarkHidden("pk")
}

// addPayoutAmountFlags registers the flags send and prepare share, bound to the same
// variables.
func addPayoutAmountFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&payoutSendDestination, "destination", "d", "", "Destination id ('rail0 payouts destinations')")
	cmd.Flags().StringVarP(&payoutSendAmount, "amount", "a", "",
		"Decimal in token units (e.g. 250.00), or 'all' for the balance minus the refund reserve")
	cmd.Flags().BoolVar(&payoutSendProof, "proof", false, "Send the proof transfer (the gateway sets its amount) instead of a payout")
	cmd.MarkFlagsMutuallyExclusive("amount", "proof")
}

func init() {
	addPayoutAmountFlags(payoutsSendCmd)
	addPayoutKeyFlags(payoutsSendCmd, &payoutSendKey)
	addYesFlag(payoutsSendCmd)

	addPayoutAmountFlags(payoutsPrepareCmd)

	addPayoutKeyFlags(payoutsSubmitCmd, &payoutSubmitKey)
	addYesFlag(payoutsSubmitCmd)
}
