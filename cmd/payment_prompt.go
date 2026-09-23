package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/term"
	rail0 "github.com/commercelayer/rail0-go"
)

// opFilter constrains the interactive picker to payments an operation can act on,
// mirroring the gateway's state/mode guards, so it never offers an incompatible
// payment. mode/disputed are pushed to the gateway's list filters; statuses is
// applied client-side (the gateway's status filter is single-valued).
type opFilter struct {
	statuses []string // allowed payment statuses (empty = any)
	mode     string   // required mode ("authorize"/"charge"; "" = any)
	disputed *bool    // required disputed flag (nil = any)
}

func boolPointer(b bool) *bool { return &b }

// `partially_refunded` appears below and is NOT dead, despite `payments get`'s help
// describing it as "legacy; no longer produced". Both statements are true: no new
// payment reaches that status (a partial refund leaves the status unchanged), but the
// gateway deliberately keeps it in its CAPTURABLE_FROM / REFUNDABLE_FROM sets so
// operations on HISTORICAL rows still resolve. Since these filters exist to mirror
// those guards, dropping it here would not remove dead code — it would make the
// picker hide a payment the gateway would happily act on, forcing the id to be pasted
// by hand. Remove it only together with the gateway's sets. (#52)
var opFilters = map[string]opFilter{
	// authorize/charge both act on a signed payment, but only in their own mode.
	"authorize": {statuses: []string{"signed"}, mode: "authorize"},
	"charge":    {statuses: []string{"signed"}, mode: "charge"},
	// capture and release both act on CAPTURABLE_FROM, which includes `expired`: a
	// capture the chain accepts across the boundary, and — above all — the release
	// that status exists for. Release offered only `authorized`, whose window is
	// usually still OPEN, and hid both `expired` and a partial capture's stranded
	// escrow. void mirrors its own AASM source, authorized or expired.
	"capture": {statuses: []string{"authorized", "expired", "partially_captured", "partially_refunded"}},
	"void":    {statuses: []string{"authorized", "expired"}},
	"release": {statuses: []string{"authorized", "expired", "partially_captured", "partially_refunded"}},
	"refund":  {statuses: []string{"charged", "captured", "partially_captured", "partially_refunded"}},
	// dispute opens a dispute (only when none is open); close resolves the open one.
	"dispute":       {statuses: []string{"charged", "captured", "partially_captured", "partially_refunded"}, disputed: boolPointer(false)},
	"close dispute": {disputed: boolPointer(true)},
}

// pickPaymentID resolves the payment an operation acts on. A positional argument
// is used as-is. Otherwise, on a TTY, it fetches the caller's payments, keeps the
// ones op can act on, and shows a picker; when none match (or the list can't be
// fetched — e.g. release run by the un-authenticated payer) it falls back to a
// free-text prompt. Non-interactive with no argument → error.
//
// Whatever it returns goes straight into the request path: the gateway resolves
// EITHER identifier form there — the payment's UUID or its logical rail0_id (0x + 64
// hex) — so no client-side resolution step is needed, with or without a session.
// (The CLI historically pre-resolved a rail0_id to its UUID through an authenticated
// list filter; resolvePaymentID was the passthrough left behind when that stopped
// being necessary, and is now gone. #72)
func pickPaymentID(ctx context.Context, client *rail0.Client, args []string, op string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	if !term.IsTerminal(os.Stdin.Fd()) {
		return "", fmt.Errorf("a payment id (rail0_id or UUID) is required")
	}
	if id, ok := pickFromList(ctx, client, op); ok {
		return id, nil
	}
	// No actionable list available — let the user paste an id.
	return promptValue("Payment", "rail0_id (0x…, 64 hex) or UUID", false)
}

// pickFromList fetches the caller's payments, filters to op's actionable set
// (close-dispute → disputed payments), and shows a Select. Returns (id, true) on
// a pick, or ("", false) when the list is empty/unavailable so the caller can
// fall back to a free-text prompt.
func pickFromList(ctx context.Context, client *rail0.Client, op string) (string, bool) {
	f := opFilters[op]
	params := rail0.ListPaymentsParams{PerPage: 100} // first page is plenty for a picker
	params.Mode = f.mode                             // "" = no filter
	params.Disputed = f.disputed                     // nil = no filter
	resp, err := client.Payments.List(ctx, params)
	if err != nil || resp == nil || len(resp.Data) == 0 {
		return "", false
	}

	allowed := map[string]bool{}
	for _, s := range f.statuses {
		allowed[s] = true
	}

	var opts []huh.Option[string]
	for _, p := range resp.Data {
		if len(allowed) > 0 && !allowed[p.Status] {
			continue
		}
		// Human decimal, not base units: this is the line where the operator chooses
		// WHICH payment to act on, so a figure at an unstated scale is worse here
		// than anywhere else in the CLI.
		label := fmt.Sprintf("%s… · %s · %s", shortID(string(p.Rail0Id)), p.Status,
			amountLabel(ctx, string(p.Amount), p.ChainID, string(p.Token)))
		opts = append(opts, huh.NewOption(label, string(p.Rail0Id)))
	}
	if len(opts) == 0 {
		return "", false
	}

	var chosen string
	err = huh.NewSelect[string]().
		Title(fmt.Sprintf("Select a payment to %s", op)).
		Options(opts...).
		Value(&chosen).
		Run()
	if err != nil {
		abortOrReturn(err)
		return "", false
	}
	return chosen, true
}

// promptValue asks for a single value on a TTY. optional allows a blank answer
// (returns ""); a required blank re-prompts. Non-interactive: "" for optional,
// else an error.
func promptValue(label, desc string, optional bool) (string, error) {
	return promptValidated(label, desc, optional, nil)
}

// promptValidated is promptValue with an extra domain check, applied by the huh field
// itself so a bad value is rejected INLINE — before the prompt is submitted, and long
// before any network call. Used for the capture/refund amount, where the alternative
// was learning about a typo from the gateway after a session round-trip and a payment
// fetch. (#61)
func promptValidated(label, desc string, optional bool, check func(string) error) (string, error) {
	if !term.IsTerminal(os.Stdin.Fd()) {
		if optional {
			return "", nil
		}
		return "", fmt.Errorf("%s is required", label)
	}
	var v string
	in := huh.NewInput().Title(label).Description(desc).Value(&v)
	in = in.Validate(func(s string) error {
		if s == "" {
			if optional {
				return nil
			}
			return fmt.Errorf("%s is required", label)
		}
		if check != nil {
			return check(s)
		}
		return nil
	})
	if err := in.Run(); err != nil {
		abortOrReturn(err)
		return "", err
	}
	return v, nil
}

// abortOrReturn ends the process on Ctrl-C/ESC (as the create form does); any other
// error is left for the caller to handle.
//
// It exits NON-ZERO. It used to exit 0, which told a caller the operation had
// succeeded: `rail0 payments refund <id> && mark_refunded_in_db` ran the second
// command when the operator cancelled at the amount prompt. Exiting here rather than
// returning an error is deliberate — Ctrl-C should end the command immediately, not
// fall through to the free-text fallback that `pickFromList` returning false
// triggers. (#65)
func abortOrReturn(err error) {
	if errors.Is(err, huh.ErrUserAborted) {
		fmt.Fprintln(os.Stderr)
		os.Exit(ExitCodeAborted)
	}
}

func shortID(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
