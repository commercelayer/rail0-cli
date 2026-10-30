package cmd

import (
	"context"
	"fmt"
	"strings"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// The merchant's attestation that a payout destination's details are right, and the
// amount conversion its two amount flags need. Shared by `payouts destinations add`
// and `payouts destinations update`.

// payoutConfirmDetails is --confirm-details: the NON-INTERACTIVE way to accept the
// details statement (rail0.PayoutDetailsConfirmationText). One variable for both
// commands, like assumeYes: a process runs one command.
var payoutConfirmDetails bool

// addConfirmDetailsFlag registers --confirm-details on a command that may need the
// attestation.
func addConfirmDetailsFlag(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&payoutConfirmDetails, "confirm-details", false,
		"Accept the details statement without a prompt (needed without a TTY or with --agent); --yes does NOT imply it")
}

// confirmPayoutDetails shows what is about to be registered and the details statement,
// then gets the merchant's explicit consent to it — or refuses.
//
// It is the ONLY place the CLI sets confirm_details, and the gateway records it as the
// merchant's attestation (details_v1, who, when) in the destination's audit trail. So
// the rule is strict: the statement is always shown (stderr, or an agent event), and
// consent is either --confirm-details or a "y" at an interactive y/N prompt whose
// default is NO. Nothing else counts:
//
//   - --yes does not, on purpose. It means "skip the are-you-sure", and a script that
//     passes it everywhere must not attest, by accident, to having checked a deposit
//     page nobody looked at.
//   - with no TTY (or in agent mode) and no flag it refuses locally with a usage error,
//     before any request — rather than sending confirm_details=false and reading the
//     gateway's 422 confirmation_required.
func confirmPayoutDetails(summary string) error {
	ui.Notice("payout_details_confirmation", map[string]any{
		"version": rail0.PayoutDetailsConfirmationVersion,
		"text":    rail0.PayoutDetailsConfirmationText,
	}, summary+"\n\n"+wrapIndented(rail0.PayoutDetailsConfirmationText, 2, 76)+"\n")
	if payoutConfirmDetails {
		return nil
	}
	if !interactive() {
		return newUsageError("confirmation_required",
			"refusing to register payout details without your confirmation: check the address, network, token and minimum "+
				"on the venue's deposit page, then re-run with --confirm-details to accept the statement above (%s)",
			rail0.PayoutDetailsConfirmationVersion)
	}
	return promptYesNo("Do you confirm the statement above?")
}

// wrapIndented word-wraps text to width columns, every line indented by indent spaces,
// for quoting a long statement inside a prompt.
func wrapIndented(text string, indent, width int) string {
	pad := strings.Repeat(" ", indent)
	var lines []string
	line := ""
	for _, w := range strings.Fields(text) {
		if line != "" && len(pad)+len(line)+1+len(w) > width {
			lines = append(lines, pad+line)
			line = ""
		}
		if line != "" {
			line += " "
		}
		line += w
	}
	if line != "" {
		lines = append(lines, pad+line)
	}
	return strings.Join(lines, "\n")
}

// payoutAmountUnits converts a --reserve / --min-amount decimal ("10.00") to the base
// units the gateway stores, with the decimals of the destination's token. "" stays ""
// (the caller decides what an empty value means).
//
// Decimals, like every other amount flag of the CLI (see amountFilters): base units
// here would make "--min-amount 10" mean ten millionths of a USDC — a minimum that
// protects nothing, on the one field whose job is to stop a deposit being lost.
// Fails closed: a token whose decimals cannot be resolved refuses rather than guessing
// a scale.
func payoutAmountUnits(ctx context.Context, flag, value string, chainID int, token string) (string, error) {
	if value == "" {
		return "", nil
	}
	decimals, ok := tokenDecimals(ctx, chainID, token)
	if !ok {
		return "", fmt.Errorf("unknown token %s on %s — its decimals are needed to convert %s (see 'rail0 tokens')",
			token, chainLabel(chainID), flag)
	}
	units, err := rail0.ToBaseUnits(value, decimals)
	if err != nil {
		return "", fmt.Errorf("%s: %w (use a decimal in token units, e.g. 10.00)", flag, err)
	}
	return units, nil
}

// payoutAmountLabel renders a base-unit amount of a destination's token for a prompt:
// "10 USDC", or "none" for an empty one. The raw units are shown when the token's
// decimals are unknown, labelled as such rather than at a guessed scale.
func payoutAmountLabel(ctx context.Context, units string, chainID int, token, symbol string) string {
	if units == "" {
		return "none"
	}
	if symbol == "" {
		symbol = token
	}
	decimals, ok := tokenDecimals(ctx, chainID, token)
	if !ok {
		return units + " base units of " + symbol
	}
	return rail0.FormatAmount(units, decimals) + " " + symbol
}
