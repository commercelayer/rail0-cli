package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/term"
	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	accountDisputesStatus  string
	accountDisputesSort    string
	accountDisputesPage    int
	accountDisputesPerPage int
)

// `rail0 disputes` — the ACCOUNT-level list, distinct from `rail0 payments disputes
// <rail0Id>`, which is one payment's open/close history. The gateway has had both for
// a while and the CLI exposed only the second, so the question a merchant actually
// asks — "is anything disputed right now?" — had no answer short of walking every
// payment. (#110)
//
// Bare = list, following the noun→verb shape of `wallets`, `aliases` and `keys`.
var disputesCmd = &cobra.Command{
	Use:   "disputes",
	Short: "List every dispute on your payments (requires login)",
	Long: `List the disputes across the payments your wallet is party to, as payer or
payee, each with its parent payment embedded.

Open AND closed: a closed dispute stops setting the 'disputed' flag on
'payments list' (which reports current state) but is still listed here, so the
history stays visible. Filter with --status to see only one of the two.

Distinct from 'rail0 payments disputes <rail0Id>', which is the open/close history
of ONE payment.`,
	Example: `  rail0 disputes
  rail0 disputes --status open
  rail0 disputes --sort -opened_at --per-page 50`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return ui.RunAPI(func() (any, error) {
			ctx := context.Background()
			resp, err := newReadClient().Disputes.List(ctx, rail0.ListDisputesParams{
				Status:  accountDisputesStatus,
				Sort:    accountDisputesSort,
				Page:    accountDisputesPage,
				PerPage: accountDisputesPerPage,
			})
			if err != nil {
				return nil, err
			}
			return resp.Data, nil // bare array; pagination is in headers
		})
	},
}

func init() {
	disputesCmd.Flags().StringVar(&accountDisputesStatus, "status", "", "Filter by dispute status: open or closed")
	disputesCmd.Flags().StringVar(&accountDisputesSort, "sort", "", "Sort fields (e.g. -opened_at)")
	disputesCmd.Flags().IntVar(&accountDisputesPage, "page", 0, "Page number (1-based)")
	disputesCmd.Flags().IntVar(&accountDisputesPerPage, "per-page", 0, "Results per page")

	// A finite set, so it completes: `--status ope<TAB>` beats reading --help, and a
	// typo here costs a round-trip to find out.
	_ = disputesCmd.RegisterFlagCompletionFunc("status",
		fixedCompletions("open", "closed"))

	disputesReasonsCmd.Flags().StringVar(&disputeReasonsKind, "kind", "", "Only one list: open, close or system")
	_ = disputesReasonsCmd.RegisterFlagCompletionFunc("kind",
		fixedCompletions("open", "close", "system"))
	disputesCmd.AddCommand(disputesReasonsCmd)
}

// ── dispute reasons ─────────────────────────────────────────────────────────

// disputeReasonList is one side of the gateway's dispute-reason dictionary
// (rail0-gateway#381): the codes a dispute can be opened with, or closed with. The
// two are separate lists — `other` sits in both with different descriptions — so
// every check names which one it applies. The entries come from the SDK, which
// generates them from the gateway's OpenAPI; nothing is listed here by hand.
type disputeReasonList struct {
	kind   string // "open" / "close", as the errors and the picker name it
	list   []rail0.DisputeReasonInfo
	lookup func(string) (rail0.DisputeReasonInfo, bool)
}

var (
	openDisputeReasons  = disputeReasonList{"open", rail0.DisputeOpenReasons, rail0.LookupDisputeOpenReason}
	closeDisputeReasons = disputeReasonList{"close", rail0.DisputeCloseReasons, rail0.LookupDisputeCloseReason}
)

func (l disputeReasonList) codes() []string {
	out := make([]string, len(l.list))
	for i, e := range l.list {
		out[i] = string(e.Code)
	}
	return out
}

// zeroBytes32 is the all-zero word: the gateway reads it as "no reason", exactly as
// it does an omitted reason, so the CLI accepts it as an explicit way to say none.
var zeroBytes32 = "0x" + strings.Repeat("0", 64)

// resolve checks a --reason value locally with the rule the gateway applies (a code,
// or exactly its bytes32 in any hex case) and returns the CODE to send, so a typo
// fails here with the valid list rather than as a 422 after the session round-trip.
// A blank value or the zero bytes32 means no reason and resolves to "" (the SDK then
// leaves the field out). full_refund gets its own message on the close side: it IS
// a close reason, just one only the protocol records, and "unknown" would send the
// user looking for a typo.
func (l disputeReasonList) resolve(input string) (rail0.DisputeReason, error) {
	v := strings.TrimSpace(input)
	if v == "" || strings.EqualFold(v, zeroBytes32) {
		return "", nil
	}
	if info, ok := l.lookup(v); ok {
		return info.Code, nil
	}
	if l.kind == "close" {
		for _, e := range rail0.DisputeSystemCloseReasons {
			if string(e.Code) == v || strings.EqualFold(e.Bytes32, v) {
				return "", fmt.Errorf("%s is recorded by the protocol itself (a full refund closes the dispute) and can't be chosen — use one of: %s, or leave --reason out for no reason",
					e.Code, strings.Join(l.codes(), ", "))
			}
		}
	}
	return "", fmt.Errorf("unknown %s reason %q — use one of: %s, or leave --reason out for no reason (see 'rail0 disputes reasons')",
		l.kind, input, strings.Join(l.codes(), ", "))
}

// orPick returns the reason for flag. The reason is optional (rail0-gateway#382):
// a given flag is resolved; otherwise a terminal offers the list with "No reason"
// first, so Enter goes on without one; a script with no flag simply sends none.
// "" means no reason.
func (l disputeReasonList) orPick(flag string) (rail0.DisputeReason, error) {
	if flag != "" {
		return l.resolve(flag)
	}
	if !term.IsTerminal(os.Stdin.Fd()) {
		return "", nil
	}
	opts := make([]huh.Option[string], 0, len(l.list)+1)
	opts = append(opts, huh.NewOption("No reason", ""))
	for _, e := range l.list {
		opts = append(opts, huh.NewOption(fmt.Sprintf("%s — %s", e.Code, e.Description), string(e.Code)))
	}
	var chosen string
	if err := huh.NewSelect[string]().
		Title(fmt.Sprintf("Reason to %s the dispute (optional)", l.kind)).
		Options(opts...).
		Value(&chosen).
		Run(); err != nil {
		abortOrReturn(err)
		return "", err
	}
	return rail0.DisputeReason(chosen), nil
}

// reasonLabel is how a confirmation prompt names the chosen reason.
func reasonLabel(r rail0.DisputeReason) string {
	if r == "" {
		return "no reason"
	}
	return string(r)
}

var disputeReasonsKind string

// disputeReasonRow is one line of `rail0 disputes reasons`. Bytes32 is only filled
// for machine output: on a terminal the 66-character hash drowns the two columns a
// person reads, while a script matching on-chain values needs it.
type disputeReasonRow struct {
	Kind        string `json:"kind"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Bytes32     string `json:"bytes32,omitempty"`
}

var disputesReasonsCmd = &cobra.Command{
	Use:   "reasons",
	Short: "List the dispute reasons: the codes --reason accepts",
	Long: `List the dispute-reason dictionary: the codes 'rail0 payments dispute' and
'rail0 payments dispute close' accept for --reason, with their descriptions.

There are two lists. A dispute is OPENED with an 'open' reason and CLOSED with a
'close' reason; 'other' is in both. The 'system' reason (full_refund) is never
chosen: the protocol records it when a full refund closes an open dispute, and it
is listed so a dispute's close_reason_code can be read.

On-chain each reason is the bytes32 keccak256("rail0.dispute.<code>"); --json (or
piped output) adds it to every row. The list is the one this CLI was built with —
the gateway's is authoritative.`,
	Example: `  rail0 disputes reasons
  rail0 disputes reasons --kind close
  rail0 disputes reasons --json`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		rows, err := disputeReasonRows(disputeReasonsKind, ui.IsMachineOutput())
		if err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) { return rows, nil })
	},
}

// disputeReasonRows flattens the dictionaries (or the one kind asks for) into rows,
// open first, then close, then system — the order a user meets them in.
func disputeReasonRows(kind string, withBytes32 bool) ([]disputeReasonRow, error) {
	lists := []struct {
		kind string
		list []rail0.DisputeReasonInfo
	}{
		{"open", rail0.DisputeOpenReasons},
		{"close", rail0.DisputeCloseReasons},
		{"system", rail0.DisputeSystemCloseReasons},
	}
	var rows []disputeReasonRow
	matched := false
	for _, l := range lists {
		if kind != "" && kind != l.kind {
			continue
		}
		matched = true
		for _, e := range l.list {
			row := disputeReasonRow{Kind: l.kind, Code: string(e.Code), Description: e.Description}
			if withBytes32 {
				row.Bytes32 = e.Bytes32
			}
			rows = append(rows, row)
		}
	}
	if !matched {
		return nil, fmt.Errorf("--kind must be open, close or system, got %q", kind)
	}
	return rows, nil
}
