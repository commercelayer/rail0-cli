package cmd

import (
	"context"
	"errors"
	"fmt"
	"math/big"
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
	payoutSendNoRefund    bool
	payoutSendNoGas       bool
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
transaction hash; it turns "confirmed" once its block is final ('payouts get <id>').
With -w/--wait the command instead follows it until "confirmed" (the result) or
"failed" (an error); see --wait below.

--amount is a decimal in token units (250.00), or "all": the wallet's balance minus
the reserves kept back, computed by the gateway. --proof sends the proof transfer
instead (1 token unit, raised to the destination's minimum) — the step before
'payouts destinations confirm'. An unverified destination takes the proof only.

Reserves: every payout keeps two reserves back in the wallet by default, decided
when it is prepared ('payouts destinations reserves <id>' shows them beforehand):
  - the refund reserve: the refundable balance of the wallet's payments in this
    token still inside their refund window (a refund is pulled from the wallet);
  - the gas reserve, only on a chain whose gas is paid in this very token (Arc's
    USDC): the gas for those refunds and for the captures the wallet still owes.
    On such a chain the transfer's own gas allowance is kept back too.
They bound "all" and a typed amount alike. --no-refund-reserve / --no-gas-reserve
skip one for this payout only: a later refund, or a later capture or refund
without the gas to pay for it, may then FAIL. The payout records what it kept back
(refund_reserve, gas_reserve).

Before signing, the command shows what will be sent — amount, token, deposit
address, network, from, and the reserves kept back — and asks to confirm (y/N);
--yes skips the prompt and is required without a terminal. With "all" or --proof the gateway sets the amount: to
see the exact figure before signing, run 'payouts prepare' and then
'payouts submit <id>', which shows it.

The key is the destination's wallet's: the CLI signs in with it. Given as a raw 0x
hex key, as @name for one saved with 'rail0 keys add', through RAIL0_PRIVATE_KEY,
or omitted to be prompted for (hidden).

Refused: payout_below_minimum (below the destination's minimum),
insufficient_balance, payout_destination_unverified (send the proof and confirm
first), payout_destination_disabled, payout_already_signed (a signed payout is
pending: 'payouts resend <id>'). A node refusal (502) leaves the payout pending and
signed: 'payouts resend <id>'.

--wait polls until the payout is confirmed or failed. Confirmed comes at the
chain's finality, so it can take longer than the first receipt. Without --timeout
the deadline follows the chain: 10× its measured p90 settlement ('rail0 chains'),
kept within 5m–15m. A wait that runs out exits 11: the transfer may still be in
flight — check it with 'payouts get <id>', never send it again.`,
	Example: `  rail0 payouts send --destination <destination_id> --proof -p @merchant-key --wait
  rail0 payouts send --destination <destination_id> --amount 250.00 -p @merchant-key
  rail0 payouts send --destination <destination_id> --amount all -p @merchant-key --yes
  rail0 payouts send --destination <destination_id> --amount all --no-refund-reserve -p @merchant-key`,
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
		if err := confirmDestructive(payoutSendPrompt(ctx, dest, req, payoutReservesForPrompt(ctx, client, req), signer)); err != nil {
			return err
		}
		var sent *rail0.Payout
		err = ui.RunAPIWithMessage(actionLabel("Sending payout…"), func() (any, error) {
			p, err := client.Payouts.PrepareAndSubmit(ctx, req, privKey)
			if err != nil {
				return nil, payoutSignError(err)
			}
			sent = p
			return submittedPayoutThenWait(ctx, client, p, &sent)
		})
		if err != nil {
			return err
		}
		notePayoutSent(sent)
		return nil
	},
}

// payoutPrepareRequest builds the prepare request from --destination, --amount,
// --proof and the reserve opt-outs (shared by send and prepare), refusing a bad
// combination before any request: a proof's amount is the gateway's and it keeps no
// reserve back, and a payout needs an amount — a decimal (checked syntactically here,
// against the token's decimals by the caller) or "all".
//
// keep_refund_reserve / keep_gas_reserve are sent only as false, on the matching
// --no-… flag: left out, the gateway keeps both (its default), so the CLI never sends
// a "true" that would read as a choice the merchant did not make.
func payoutPrepareRequest() (rail0.PreparePayoutRequest, error) {
	req := rail0.PreparePayoutRequest{DestinationID: payoutSendDestination, Purpose: rail0.PayoutPurposePayout}
	switch {
	case payoutSendDestination == "":
		return req, newUsageError("usage_error", "--destination is required")
	case payoutSendProof && payoutSendAmount != "":
		return req, newUsageError("usage_error", "--proof sends the gateway's proof amount; drop --amount")
	case payoutSendProof && (payoutSendNoRefund || payoutSendNoGas):
		return req, newUsageError("usage_error", "--no-refund-reserve / --no-gas-reserve apply to a payout, not to the proof")
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
	if payoutSendNoRefund {
		no := false
		req.KeepRefundReserve = &no
	}
	if payoutSendNoGas {
		no := false
		req.KeepGasReserve = &no
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
// says how instead of a number — and names the minimum / reserves that number depends
// on. A payout's reserves come from a fresh GET …/reserves (res, nil when it could not
// be read): the figures the prepare is about to keep back, or "skipped" for the ones
// the merchant opted out of, with what that risks.
func payoutSendPrompt(ctx context.Context, d *rail0.PayoutDestination, req rail0.PreparePayoutRequest, res *rail0.PayoutReserves, signer string) string {
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
		amount = "ALL — the wallet's balance minus the reserves kept back"
		if res != nil {
			amount += " (about " + payoutAmountLabel(ctx, availableAfter(res, req), d.ChainID, token, d.TokenSymbol) + " now)"
		}
	default:
		amount = req.Amount + " " + symbol
	}
	to := string(d.Address)
	if d.Label != "" {
		to += " (" + d.Label + ")"
	}
	prompt := fmt.Sprintf("Send this payout? A transfer is final once mined.\n"+
		"  amount   %s\n"+
		"  token    %s\n"+
		"  to       %s\n"+
		"  network  %s\n"+
		"  from     %s",
		amount, tokenLabel(d.TokenSymbol, token), to, chainLabel(d.ChainID), signer)
	if req.Purpose == rail0.PayoutPurposeProof {
		return prompt
	}
	label := func(units string) string { return payoutAmountLabel(ctx, units, d.ChainID, token, d.TokenSymbol) }
	refund, gas := "kept back", "kept back"
	if res != nil {
		refund = fmt.Sprintf("%s kept back (%s)", label(res.RefundReserve),
			plural(res.RefundablePayments, "refundable payment", "refundable payments"))
		gas = label(res.GasReserve) + " kept back"
	}
	if payoutSendNoRefund {
		refund = "SKIPPED (--no-refund-reserve) — a later refund may fail for lack of funds"
	}
	if payoutSendNoGas {
		gas = "SKIPPED (--no-gas-reserve) — a later refund or capture may fail for lack of gas"
	}
	prompt += "\n  refund reserve  " + refund
	// The gas reserve exists only where the chain's gas is this very token; elsewhere
	// it is 0 and a line about it would only confuse. Unknown (no read): say it anyway.
	if res == nil || res.NativeGas {
		prompt += "\n  gas reserve     " + gas
	}
	return prompt
}

// availableAfter is what "all" would move now: the gateway's `available` when both
// reserves are kept, else balance minus the reserves still kept and the gas allowance
// (always kept back on a native-gas chain), floored at 0 — the gateway's own formula.
// Base units; a figure the gateway sent malformed falls back to `available`.
func availableAfter(r *rail0.PayoutReserves, req rail0.PreparePayoutRequest) string {
	if req.KeepRefundReserve == nil && req.KeepGasReserve == nil {
		return r.Available
	}
	n, ok := new(big.Int).SetString(r.Balance, 10)
	if !ok {
		return r.Available
	}
	sub := []string{r.GasAllowance}
	if req.KeepRefundReserve == nil || *req.KeepRefundReserve {
		sub = append(sub, r.RefundReserve)
	}
	if req.KeepGasReserve == nil || *req.KeepGasReserve {
		sub = append(sub, r.GasReserve)
	}
	for _, s := range sub {
		if s == "" {
			continue
		}
		m, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return r.Available
		}
		n.Sub(n, m)
	}
	if n.Sign() < 0 {
		return "0"
	}
	return n.String()
}

// payoutReservesForPrompt reads the reserves a payout to the destination would keep
// back, for the confirmation prompt only — best effort: a failed read leaves the
// prompt without figures (nil) rather than stopping a send the gateway may accept.
// A proof keeps nothing back, so it reads nothing.
func payoutReservesForPrompt(ctx context.Context, client *rail0.Client, req rail0.PreparePayoutRequest) *rail0.PayoutReserves {
	if req.Purpose == rail0.PayoutPurposeProof {
		return nil
	}
	r, err := client.PayoutDestinations.Reserves(ctx, req.DestinationID)
	if err != nil {
		return nil
	}
	return r
}

// payoutSentHint is the next step after a submit: a proof needs the venue check and
// 'confirm'; a payout only needs watching until it is mined. After a --wait that saw
// it confirmed, a payout needs nothing more ("") and a proof only the venue check.
func payoutSentHint(p *rail0.Payout) string {
	confirmed := p.Status == rail0.PayoutStatusConfirmed
	switch {
	case p.Purpose == rail0.PayoutPurposeProof && confirmed:
		return fmt.Sprintf("\nProof confirmed on-chain. Once the venue shows it credited:\n"+
			"  rail0 payouts destinations confirm %s", p.DestinationID)
	case p.Purpose == rail0.PayoutPurposeProof:
		return fmt.Sprintf("\nProof submitted. Once 'rail0 payouts get %s' shows it confirmed AND the venue shows it credited:\n"+
			"  rail0 payouts destinations confirm %s", p.ID, p.DestinationID)
	case confirmed:
		return ""
	}
	return fmt.Sprintf("\nSubmitted. Follow it with 'rail0 payouts get %s' (submitted → confirmed), or pass --wait.", p.ID)
}

// notePayoutSent prints payoutSentHint for the payout a send/submit/resend ended with,
// unless --quiet or there is nothing left to do.
func notePayoutSent(p *rail0.Payout) {
	if ui.IsQuiet() || p == nil {
		return
	}
	if h := payoutSentHint(p); h != "" {
		ui.Notice("hint", map[string]any{"payout_id": p.ID}, h)
	}
}

// submittedPayoutThenWait is submitThenWait for a payout: without --wait it returns the
// payout the gateway answered (submitted), with --wait it follows it to confirmed or
// failed (waitForPayout). `last` is updated to the latest payout seen, so the caller's
// hint speaks about the outcome rather than the submit.
func submittedPayoutThenWait(ctx context.Context, client *rail0.Client, p *rail0.Payout, last **rail0.Payout) (any, error) {
	if !waitFlag {
		return p, nil
	}
	final, err := waitForPayout(ctx, client, p)
	if err != nil {
		return nil, err
	}
	*last = final
	return final, nil
}

// waitForPayout polls a payout the gateway has just been handed until it is confirmed
// (returned) or failed (a decision: payoutFailedError), on the same loop and schedule
// as every payment wait (pollUntil) — transient read errors are retried, Ctrl-C and a
// deadline say the transfer may still be in flight and name 'payouts get'.
//
// The deadline is the payment one for the payout's chain (waitTimeoutFor): a payout
// becomes confirmed only once the chain calls its block final, which is exactly the
// settlement the chain's measured p90 describes — so a wait can run past the first
// receipt. The first read waits one interval, like a payment submit's: the broadcast
// was just queued and an immediate read could only say "submitted".
func waitForPayout(ctx context.Context, client *rail0.Client, p *rail0.Payout) (*rail0.Payout, error) {
	res, err := pollUntil(ctx, waitTimeoutFor(p.ChainID), fmt.Sprintf("rail0 payouts get %s", p.ID), true,
		func(ctx context.Context) (any, bool, error) {
			cur, err := client.Payouts.Get(ctx, p.ID)
			if err != nil {
				return nil, false, err
			}
			switch cur.Status {
			case rail0.PayoutStatusConfirmed:
				return cur, true, nil
			case rail0.PayoutStatusFailed:
				return nil, false, stopPolling{payoutFailedError(cur)}
			}
			return nil, false, nil
		})
	if err != nil {
		return nil, err
	}
	return res.(*rail0.Payout), nil
}

// payoutFailedError renders a failed payout like opFailedError renders a reverted
// transaction: the gateway's detail (or its code) plus what to do. submission_timeout
// and dropped are not final — the transfer that did go out may have been sped up or
// replaced — so their message names report-hash, the way back to submitted.
func payoutFailedError(p *rail0.Payout) error {
	reason := p.ErrorDetail
	if reason == "" {
		reason = p.ErrorCode
	}
	if reason == "" {
		reason = "failed"
	}
	msg := fmt.Sprintf("payout %s failed: %s", p.ID, reason)
	switch p.ErrorCode {
	case "submission_timeout", "dropped":
		msg += fmt.Sprintf(" — not final: if the transfer went out under another hash (sped up or replaced), "+
			"report it with 'rail0 payouts report-hash %s <hash>'", p.ID)
	default:
		if h := hintFor(p.ErrorCode, ""); h != "" {
			msg += " — " + h
		}
	}
	return errors.New(msg)
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
"all" or a proof comes to —, the reserves it keeps back in the wallet
(refund_reserve, gas_reserve) and unsigned_transaction, the EIP-1559 transaction to
sign. Nothing is signed or sent.

Both reserves are kept by default; --no-refund-reserve / --no-gas-reserve skip one
for this payout (see 'payouts send --help': a later refund or capture may then
fail).

Then either 'payouts submit <id>' (verify, sign with -p, submit), or sign it with
a wallet of your own and report the hash with 'payouts report-hash <id> <hash>'.
Check unsigned_transaction before signing it elsewhere: 'to' must be the token,
'data' transfer(destination address, amount), value 0.

A prepare replaces the destination's pending payout while it is unsigned; leave it
and it stays pending until replaced or cancelled ('payouts cancel <id>'). The
session must be the destination's wallet ('rail0 auth login').`,
	Example: `  rail0 payouts prepare --destination <destination_id> --amount all
  rail0 payouts prepare --destination <destination_id> --amount all --no-gas-reserve
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
touches it. The command shows the amount, token, deposit address and network —
and, for a payout, the reserves it keeps back in the wallet — and asks to confirm (y/N); --yes skips it and is required without a terminal.

Only a pending, unsigned payout can be signed. One already signed is 'resend's:
its transaction may already be in a mempool, and signing again would risk a second
transfer.

-w/--wait follows the payout until confirmed or failed, as in 'payouts send'.`,
	Example: `  rail0 payouts submit <payout_id> -p @merchant-key
  rail0 payouts submit <payout_id> -p @merchant-key --wait`,
	Args: cobra.ExactArgs(1),
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
		if err := confirmDestructive(payoutSubmitPrompt(ctx, payout, signer)); err != nil {
			return err
		}
		var sent *rail0.Payout
		err = ui.RunAPIWithMessage(actionLabel("Sending payout…"), func() (any, error) {
			p, err := client.Payouts.SignAndSubmit(ctx, payout, privKey)
			if err != nil {
				return nil, payoutSignError(err)
			}
			sent = p
			return submittedPayoutThenWait(ctx, client, p, &sent)
		})
		if err != nil {
			return err
		}
		notePayoutSent(sent)
		return nil
	},
}

// payoutSubmitPrompt is the confirmation before signing an existing payout, from the
// payout's own decoded intent. The SDK then holds the transaction to the destination
// as read again from the gateway, so a payout whose fields disagree with its
// destination is refused after this prompt, before signing. A payout also shows the
// reserves its prepare kept back (refund_reserve, gas_reserve): "0" is an opt-out —
// or, for the gas reserve, a chain whose gas is not this token.
func payoutSubmitPrompt(ctx context.Context, p *rail0.Payout, signer string) string {
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
	prompt := fmt.Sprintf("Sign and send this %s? A transfer is final once mined.\n"+
		"  amount   %s\n"+
		"  token    %s\n"+
		"  to       %s\n"+
		"  network  %s\n"+
		"  from     %s",
		what, amount, tokenLabel(p.TokenSymbol, string(p.Token)), p.Address, chainLabel(p.ChainID), signer)
	if p.RefundReserve != "" {
		prompt += "\n  refund reserve  " + payoutAmountLabel(ctx, p.RefundReserve, p.ChainID, string(p.Token), p.TokenSymbol) + " kept back"
	}
	if p.GasReserve != "" && p.GasReserve != "0" {
		prompt += "\n  gas reserve     " + payoutAmountLabel(ctx, p.GasReserve, p.ChainID, string(p.Token), p.TokenSymbol) + " kept back"
	}
	return prompt
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
		"Decimal in token units (e.g. 250.00), or 'all' for the balance minus the reserves kept back")
	cmd.Flags().BoolVar(&payoutSendProof, "proof", false, "Send the proof transfer (the gateway sets its amount) instead of a payout")
	cmd.Flags().BoolVar(&payoutSendNoRefund, "no-refund-reserve", false,
		"Do not keep the refund reserve back for this payout (a later refund may fail)")
	cmd.Flags().BoolVar(&payoutSendNoGas, "no-gas-reserve", false,
		"Do not keep the gas reserve back for this payout (native-gas chains; a later refund or capture may fail)")
	cmd.MarkFlagsMutuallyExclusive("amount", "proof")
}

func init() {
	addPayoutAmountFlags(payoutsSendCmd)
	addPayoutKeyFlags(payoutsSendCmd, &payoutSendKey)
	addYesFlag(payoutsSendCmd)
	addWaitFlags(payoutsSendCmd)

	addPayoutAmountFlags(payoutsPrepareCmd)

	addPayoutKeyFlags(payoutsSubmitCmd, &payoutSubmitKey)
	addYesFlag(payoutsSubmitCmd)
	addWaitFlags(payoutsSubmitCmd)
}
