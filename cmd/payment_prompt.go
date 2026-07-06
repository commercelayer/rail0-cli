package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/term"
	rail0 "github.com/rail0/go-sdk"
)

// opActionableStatuses maps an operation to the payment statuses it can act on,
// mirroring the gateway's state guards — used to filter the interactive payment
// picker so it only offers payments the operation can actually run against.
// close-dispute isn't status-driven (it filters on the disputed flag), handled
// separately in pickFromList.
var opActionableStatuses = map[string][]string{
	"authorize": {"signed"},
	"charge":    {"signed"},
	"capture":   {"authorized", "partially_captured", "partially_refunded"},
	"void":      {"authorized"},
	"release":   {"authorized"},
	"refund":    {"charged", "captured", "partially_captured", "partially_refunded"},
	"dispute":   {"charged", "captured", "partially_captured", "partially_refunded"},
}

// pickPaymentID resolves the payment an operation acts on. A positional argument
// is used as-is. Otherwise, on a TTY, it fetches the caller's payments, keeps the
// ones op can act on, and shows a picker; when none match (or the list can't be
// fetched — e.g. release run by the un-authenticated payer) it falls back to a
// free-text prompt. Non-interactive with no argument → error.
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
	params := rail0.ListPaymentsParams{PerPage: 100} // first page is plenty for a picker
	if op == "close dispute" {
		disputed := true
		params.Disputed = &disputed
	}
	resp, err := client.Payments.List(ctx, params)
	if err != nil || resp == nil || len(resp.Data) == 0 {
		return "", false
	}

	allowed := map[string]bool{}
	for _, s := range opActionableStatuses[op] {
		allowed[s] = true
	}

	var opts []huh.Option[string]
	for _, p := range resp.Data {
		if len(allowed) > 0 && !allowed[p.Status] {
			continue
		}
		label := fmt.Sprintf("%s… · %s · amount %s", shortID(string(p.Rail0Id)), p.Status, p.Amount)
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
	if !term.IsTerminal(os.Stdin.Fd()) {
		if optional {
			return "", nil
		}
		return "", fmt.Errorf("%s is required", label)
	}
	var v string
	in := huh.NewInput().Title(label).Description(desc).Value(&v)
	if !optional {
		in = in.Validate(func(s string) error {
			if s == "" {
				return fmt.Errorf("%s is required", label)
			}
			return nil
		})
	}
	if err := in.Run(); err != nil {
		abortOrReturn(err)
		return "", err
	}
	return v, nil
}

// abortOrReturn exits cleanly on Ctrl-C/ESC (as the create form does); any other
// error is left for the caller to handle.
func abortOrReturn(err error) {
	if errors.Is(err, huh.ErrUserAborted) {
		fmt.Fprintln(os.Stderr)
		os.Exit(0)
	}
}

func shortID(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
