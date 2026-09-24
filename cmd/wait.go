package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	waitFlag    bool
	waitTimeout time.Duration
	waitUntil   string
)

// Polling cadence: every waitPollInterval for the first waitPollFastPhase, then each
// interval half again as long as the one before, up to waitPollMaxInterval.
//
// A flat 2s made a wait's cost proportional to the chain's finality: ~3 reads on Arc,
// ~65 on Base or OP Sepolia, ~330 on Arbitrum Sepolia and 451 for a wait that ran out
// its 15 minutes — each against the session's 300/min budget, so ten parallel `-w`
// captures on a slow chain throttled the session they all share. Settlement that has
// not come in the first half minute is waiting on the chain's safe/finalized block,
// which is minutes away and not 2s closer on the next read; the backoff costs at most
// waitPollMaxInterval of extra latency at the end of such a wait, and nothing on the
// chains that settle in seconds.
//
// Vars, not consts, so the tests can drive the polling loop without sleeping for real
// seconds.
var (
	waitPollInterval    = 2 * time.Second
	waitPollFastPhase   = 30 * time.Second
	waitPollMaxInterval = 10 * time.Second
)

// nextPollDelay is the pause before the next read, given how long the wait has run and
// the previous pause (see waitPollInterval for the schedule).
func nextPollDelay(elapsed, previous time.Duration) time.Duration {
	if elapsed < waitPollFastPhase || previous < waitPollInterval {
		return waitPollInterval
	}
	return min(previous*3/2, max(waitPollMaxInterval, waitPollInterval))
}

// Default polling deadline for --wait.
//
// It was 2 minutes, chosen when every chain gated its notify at ONE confirmation, so a
// capture confirmed in seconds. The gateway and indexer now gate on the chain's own
// `safe`/`finalized` block (commercelayer/rail0-indexer#91), and the measured lag of
// that block from the head is:
//
//	Arc Testnet        safe = head        ~0
//	Polygon Amoy       finalized ≈ head   ~0
//	OP Sepolia         safe               ~1.8 min
//	Base Sepolia       safe               ~2.0 min
//	Arbitrum Sepolia   safe               ~11.0 min   (2640 blocks, L1 batch cadence)
//
// So the old default could not be reached on Arbitrum Sepolia at all, and on Base/OP the
// tag's lag alone consumed the whole budget before the notifier's tick and the gateway's
// apply. Every `--wait` there exited non-zero for an operation that was simply not
// settled yet — and a script reads non-zero as failure, so the natural retry of a capture
// attempts a SECOND capture. That is the cost this number carries.
//
// 15 minutes covers the slowest chain served today with margin. It is deliberately a
// single number rather than a per-chain one: the wait a chain needs is a fact the GATEWAY
// knows (`finality_tag`, `required_confirmations`) but does not publish in seconds and
// cannot today — neither it nor the seeds carry a block time — and the public catalog
// omits both fields anyway. Guessing per-chain here would put chain knowledge in the
// client, which is what #193 and the catalog exist to avoid. See
// commercelayer/rail0-gateway#200.
//
// The trade is honest and one-sided: on a fast chain a genuinely stuck operation now
// takes 15 minutes to be reported instead of 2. Nothing is lost by waiting — the progress
// line keeps saying the wait is alive, Ctrl-C says what is still running, and the give-up
// message names the check command. Whereas a deadline shorter than settlement reports a
// healthy operation as a failure, which is the error that costs money.
const defaultWaitTimeout = 15 * time.Minute

// How often a progress line goes to stderr. Off a TTY (--json, -q, CI) there is no
// spinner, so a wait used to be COMPLETELY silent for up to two minutes with no way to
// tell it apart from a hang. One line per ~10s is enough to show life and quiet enough
// not to bury the result. (#107) Measured in time rather than polls now that the
// interval backs off: every 5th poll would have been one line per ~50s late in a wait.
const waitProgressEvery = 10 * time.Second

// The gateway's payment-status vocabulary — `Payment.aasm.states`, which is exactly
// what its `status` query parameter is validated against. So this is the list behind
// the --status completions on `payments list` and `analytics`, where a value outside
// it earns a 400 from Grape rather than an empty result.
//
// `failed` is deliberately NOT here, and it used to be. It is a TRANSACTION status,
// never a payment one, and the mistake cost twice: the completions offered a value
// the gateway rejects outright, and `--until failed` passed validation and then sat
// out the entire deadline waiting for a status no payment can reach — precisely the
// failure the validation below exists to prevent. The transaction vocabulary still
// lives where it belongs, on tx.Status in waitForOp.
var paymentStatuses = []string{
	"unsigned", "signed", "authorized", "partially_captured",
	"captured", "charged", "voided", "released", "refunded",
	"partially_refunded", "expired",
}

// The subset a `--until` target may name: every payment status except
// `partially_refunded`. The gateway no longer produces that one — it is retained for
// rows written before the rule changed — so accepting it as a wait target would
// accept a wait that can never finish. It stays in the vocabulary above because
// those historical rows are real and must remain filterable.
//
// Two lists rather than one because they answer different questions: "what can I
// wait for" is not "what can I filter on". Sharing a single list is what let a
// transaction status leak into three flags at once.
var waitableStatuses = withoutLegacyStatuses(paymentStatuses)

func withoutLegacyStatuses(all []string) []string {
	out := make([]string, 0, len(all))
	for _, s := range all {
		if s != "partially_refunded" {
			out = append(out, s)
		}
	}
	return out
}

// `expired` is terminal HERE, and the distinction is worth stating because it is not
// terminal on-chain: a never-captured authorization whose window lapsed can still be
// RELEASED, which closes the payment as `released`. What is over is the waiting — no
// capture will ever land — so a loop that treated it as "not yet" would sit out the full
// deadline on a payment that can never settle, and exit as a timeout rather than telling
// the operator the escrow is recoverable. terminalMismatchError says how. `payments wait
// --until` decides on statusCanReach instead, so `--until released` or `--until voided`
// on an expired payment keeps waiting for the release/void that can still land.
//
// `failed` was in here too, for the same wrong reason as above — a payment never
// carries it, so the entry could only ever be dead weight encoding a false belief.
var terminalStatuses = map[string]bool{
	"voided": true, "released": true, "refunded": true, "expired": true,
}

// statusSuccessors is the payment state machine as the gateway's AASM declares it
// (Payment: sign, authorize, charge, capture, expire, void, release, refund), one
// step at a time. `--until` uses it to stop as soon as the target is out of reach
// rather than only on a terminal status: `--until authorized` on a payment already
// `captured`, or `--until captured` on a charge-mode payment (which ends `charged`),
// used to sit out the whole deadline and then report a timeout.
//
// One deliberate omission: the capture a chain accepts across the expiry boundary
// (expired → captured) is left out, for the reason `expired` is in terminalStatuses —
// it is the exception, and waiting on it would cost the full deadline in the common
// case where nothing lands.
var statusSuccessors = map[string][]string{
	"unsigned":           {"signed"},
	"signed":             {"authorized", "charged"},
	"authorized":         {"partially_captured", "captured", "expired", "voided", "released"},
	"partially_captured": {"partially_captured", "captured", "refunded"},
	"partially_refunded": {"partially_captured", "captured", "refunded"},
	"expired":            {"voided", "released"},
	"charged":            {"refunded"},
	"captured":           {"refunded"},
}

// statusCanReach reports whether a payment in `from` can still come to be `to`. A
// status this map does not know (one the gateway adds later) is assumed able to reach
// anything: the cost of that guess is a wait that runs to its deadline, where the
// opposite guess would stop a wait that might have succeeded.
func statusCanReach(from, to string) bool {
	if from == to {
		return true
	}
	if _, known := statusSuccessors[from]; !known && !terminalStatuses[from] {
		return true
	}
	seen := map[string]bool{from: true}
	queue := []string{from}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range statusSuccessors[cur] {
			if next == to {
				return true
			}
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	return false
}

// stopPolling marks an error as a DECISION rather than a failed read: the poll
// saw a definitive outcome (a reverted transaction, a terminal status that is
// not the one asked for) and the loop must surface it now. Without this marker
// the loop could not tell such an error from a transport blip, whose whole
// point is to be retried. (#107)
type stopPolling struct{ err error }

func (s stopPolling) Error() string { return s.err.Error() }
func (s stopPolling) Unwrap() error { return s.err }

// transientPollError reports whether a failed poll is worth retrying. A 4xx
// other than 429 is the gateway's verdict — the payment does not exist, or the
// session is gone — and retrying it for two minutes only delays the message.
// Anything else (no HTTP answer at all, a 5xx, a 429) is a blip: the operation
// is ALREADY broadcast, so the honest response is to keep asking until the
// deadline rather than to report a failure that never happened.
func transientPollError(err error) bool {
	var apiErr *rail0.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status >= 500 || apiErr.Status == http.StatusTooManyRequests
	}
	return true
}

// addWaitFlags registers --wait/-w and --timeout on an async on-chain action, so
// the command can block until the operation confirms (or fails) instead of
// returning at the gateway's 202.
func addWaitFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&waitFlag, "wait", "w", false, "Block until the operation confirms on-chain (or fails)")
	cmd.Flags().DurationVar(&waitTimeout, "timeout", defaultWaitTimeout,
		"Max time to wait when --wait is set — the default covers the slowest chain\x27s finality")
}

// actionLabel appends a "waiting" hint to the spinner label when --wait is set,
// so the user sees the command has moved from broadcasting to awaiting the chain.
func actionLabel(base string) string {
	if waitFlag {
		return strings.TrimSuffix(base, "…") + " (waiting for confirmation)…"
	}
	return base
}

// submitThenWait runs the submit step and, when --wait is set, polls until the
// operation's transaction confirms — returning the settled payment — or fails.
// Without --wait it returns the raw submit response (the 202 body), preserving
// the previous behaviour.
//
// The submit response is no longer discarded: every submit endpoint answers 202 with
// the transaction row it just created, so its id is what the wait follows. See
// pickWaitTarget for what that fixes. (#66)
func submitThenWait(ctx context.Context, client *rail0.Client, rail0Id, op string, submit func() (any, error)) (any, error) {
	res, err := submit()
	if err != nil {
		return nil, err
	}
	if !waitFlag {
		return res, nil
	}
	return waitForOp(ctx, client, rail0Id, op, submittedTransactionID(res), true, waitTimeout)
}

// submittedTransactionID extracts the transaction id from a submit response. All
// eight submit methods return *rail0.Transaction, so the assertion holds; it returns
// "" on any other shape so waitForOp degrades to its previous newest-first behaviour
// rather than failing a command whose operation has ALREADY been broadcast.
func submittedTransactionID(res any) string {
	if tx, ok := res.(*rail0.Transaction); ok {
		return tx.ID
	}
	return ""
}

// replayedPastPending reports whether a keyed prepare handed back a transaction the
// FIRST attempt already moved past `pending` — signed, broadcast, maybe settled.
//
// That is exactly what a re-run with --idempotency-key gets: the gateway replays the
// original row whatever its status (200 instead of 201). Signing and submitting it
// again is always wrong — submit only accepts a `pending` row, so it answered 404 and
// the re-run the help recommends after a timed-out --wait failed every time, with a
// message about a wrong id. A replayed row that is STILL pending (the first attempt
// never got as far as the submit) is not caught here: finishing it is correct.
//
// Except a pending row that is `redrivable`: the gateway already HOLDS its signed
// transaction — the first attempt's submit landed and the broadcast is under way (or
// was lost, which is what redrive is for). A re-run seconds after the first one gets
// exactly that row; signing it again queued behind the gateway's broadcast lock and
// answered 404 once the row had moved on. It is past the caller's part, so it is
// followed like any other replay.
func replayedPastPending(tx *rail0.Transaction) bool {
	if idempotencyKey == "" || tx == nil || tx.ID == "" || tx.Status == "" {
		return false
	}
	return tx.Status != "pending" || tx.Redrivable
}

// resumeReplayed follows a replayed transaction instead of re-broadcasting it: with
// --wait it waits on THAT transaction's id, exactly as the first attempt would have;
// without it returns the row, whose status says where the operation stands.
//
// A replayed row that already FAILED is the exception, with or without --wait: the
// outcome is final, so it is returned as the op-failed error the first attempt's
// --wait would have produced. Returning the row printed it and exited 0, which a
// script reads as "the capture went through". The hint names the way forward,
// because the key is what brought this row back: the same key can only ever replay
// it, so a retry needs a new one.
func resumeReplayed(ctx context.Context, client *rail0.Client, rail0Id, op string, tx *rail0.Transaction) (any, error) {
	ui.Progressf("idempotency key already used: %s transaction %s is %s — nothing re-broadcast", op, tx.ID, tx.Status)
	if tx.Status == "failed" {
		return nil, fmt.Errorf("%w (replayed by the idempotency key; to retry, pass a NEW --idempotency-key)",
			opFailedError(op, tx))
	}
	if !waitFlag {
		return tx, nil
	}
	// Polled at once (afterSubmit false): unlike a fresh submit, a replayed row may
	// already be settled, and the first read is what finds that out.
	return waitForOp(ctx, client, rail0Id, op, tx.ID, false, waitTimeout)
}

// pollUntil is the one polling loop behind both --wait and `payments wait`. It
// calls `check` on the waitPollInterval schedule (nextPollDelay) until it reports
// done, a decision stops it, the deadline passes, or the user interrupts. With
// `delayFirst` the first read waits one interval instead of going out at once — see
// waitForOp for when that is right.
//
// It exists because the two loops used to be separate copies that got three
// things wrong in the same way — and all three matter only AFTER the operation
// has been broadcast, which is exactly when a wrong answer is expensive:
//
//   - ANY read error aborted the wait, so one 429 or one 502 while polling made
//     a command exit non-zero for an operation that was in flight and probably
//     fine. Scripts read that as "the capture failed".
//   - Ctrl-C killed the process with no word on the operation still running.
//     The ctx.Done() branches existed but were dead code: every caller passed
//     context.Background(), which is never cancelled.
//   - Giving up said "timed out" without saying that the operation may well
//     have settled, nor how to find out.
//
// `checkCmd` is the runnable command that answers "what happened?", quoted in
// both the interrupt and the give-up message.
func pollUntil(ctx context.Context, timeout time.Duration, checkCmd string, delayFirst bool,
	check func(context.Context) (any, bool, error),
) (any, error) {
	// Derived here rather than threaded from Execute: the wait loops are the only
	// place where a Ctrl-C has something to say beyond killing the process.
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	started := time.Now()
	deadline := started.Add(timeout)
	var lastTransient error
	lastProgress := started
	delay := waitPollInterval

	// sleep waits `d`, or less when the deadline comes first — the read AT the deadline
	// is the last one, rather than a pause that overshoots it by a whole interval.
	sleep := func(d time.Duration) bool {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(min(d, max(time.Until(deadline), 0))):
			return true
		}
	}
	if delayFirst && !sleep(delay) {
		return nil, interruptedError(checkCmd)
	}

	for {
		var retryAfter time.Duration
		res, done, err := check(ctx)
		switch {
		case ctx.Err() != nil:
			// Interrupted mid-request: the request error (if any) is a symptom of
			// the cancellation, not the story.
			return nil, interruptedError(checkCmd)
		case err != nil:
			var decision stopPolling
			if errors.As(err, &decision) {
				return nil, decision.err
			}
			if !transientPollError(err) {
				return nil, err
			}
			lastTransient = err
			// A 429 says when the budget refills: polling again sooner only earns
			// another 429 and spends the same budget the wait needs.
			var apiErr *rail0.APIError
			if errors.As(err, &apiErr) && apiErr.Status == http.StatusTooManyRequests {
				retryAfter = apiErr.RetryAfter
			}
		case done:
			return res, nil
		}

		if !time.Now().Before(deadline) {
			return nil, gaveUpError(timeout, checkCmd, lastTransient)
		}
		if time.Since(lastProgress) >= waitProgressEvery {
			ui.Progressf("waiting… %s elapsed of %s", time.Since(started).Truncate(time.Second), timeout)
			lastProgress = time.Now()
		}
		delay = nextPollDelay(time.Since(started), delay)
		if !sleep(max(delay, retryAfter)) {
			return nil, interruptedError(checkCmd)
		}
	}
}

// interruptedError wraps ErrAborted (exit 130) and says the thing the bare
// signal death did not: the operation is still running on-chain.
func interruptedError(checkCmd string) error {
	return fmt.Errorf("%w while waiting — the operation is still in flight; check it with '%s'", ErrAborted, checkCmd)
}

// ErrWaitGaveUp marks a wait that ran out of time. It is a DECISION about the wait,
// not a transport failure, and it must be recognisable as such: the message quotes the
// last transient read error, and humanizeError/exitCode used to string-match that text
// ("connection refused", "deadline exceeded") — replacing "may still be in flight" with
// "can't reach the gateway" and exiting 9, the code a script reads as "nothing
// happened, safe to retry". Retrying a capture that was in flight is the one wrong
// answer, so both now return this error untouched and exit 1.
var ErrWaitGaveUp = errors.New("stopped waiting")

// gaveUpError names the deadline, refuses to imply the operation failed, and
// carries the last transport error when the wait spent itself on blips.
func gaveUpError(timeout time.Duration, checkCmd string, lastTransient error) error {
	msg := fmt.Sprintf(" after %s — the operation may still be in flight; check it with '%s'", timeout, checkCmd)
	// Name the usual cause, because the usual cause is not a problem: the gateway is
	// notified only once the chain calls the block settled, which on a slow chain is
	// minutes rather than seconds. Without this the message reads as "something is
	// wrong" for the one case where nothing is.
	msg += " — a wait longer than expected is usually the chain's finality, not a failure; --timeout raises the budget"
	if lastTransient != nil {
		msg += fmt.Sprintf(" (last read error: %v)", lastTransient)
	}
	return fmt.Errorf("%w%s", ErrWaitGaveUp, msg)
}

// terminalMismatchError is the answer to `--until captured` on a payment that
// ended up voided/released/refunded, or moved past the target (see statusCanReach): waiting on is pointless, and the
// old loop spent the full two minutes before reporting a timeout — hiding a
// definitive outcome behind a message that suggested slowness. The payment's
// recorded failure cause is included when it has one. (#107)
func terminalMismatchError(p *rail0.Payment, want string) error {
	msg := fmt.Sprintf("payment is %s — it can no longer become %s", p.Status, want)
	if terminalStatuses[p.Status] {
		msg = fmt.Sprintf("payment is %s, which is terminal — it can no longer become %s", p.Status, want)
	}
	// The one terminal status that still has money to move: say so, because "terminal"
	// otherwise reads as "the funds are settled" and here they are sitting in escrow.
	if p.Status == "expired" {
		msg += ". The escrow is still recoverable: run `rail0 payments release " + p.ID + "`"
	}
	if detail := strings.TrimSpace(p.LastErrorMessage); detail != "" {
		msg += fmt.Sprintf(": %s", detail)
	} else if p.LastErrorCode != "" {
		msg += fmt.Sprintf(": %s", p.LastErrorCode)
	}
	if h := hintFor(p.LastErrorCode, ""); h != "" {
		msg += fmt.Sprintf(" — %s", h)
	}
	return errors.New(msg)
}

// waitForOp polls the payment's transactions until the one this attempt created is
// confirmed (returns the settled payment) or failed (returns its decoded error), or
// the timeout elapses. `txID` identifies that attempt; see pickWaitTarget.
//
// `afterSubmit` is true when the caller has JUST handed the gateway a transaction to
// broadcast (a submit, a redrive): the 202 means queued, the broadcast is asynchronous
// and the gateway hears of the outcome only once the chain calls the block settled, so a
// read milliseconds later can only say "not yet". Skipping it saves one request per
// wait; a failure is still seen one interval (2s) after it happens.
func waitForOp(ctx context.Context, client *rail0.Client, rail0Id, op, txID string, afterSubmit bool,
	timeout time.Duration,
) (any, error) {
	return pollUntil(ctx, timeout, fmt.Sprintf("rail0 payments wait %s", rail0Id), afterSubmit,
		func(ctx context.Context) (any, bool, error) {
			txs, err := client.Payments.Transactions(ctx, rail0Id, rail0.ListTransactionsParams{Operation: op})
			if err != nil {
				return nil, false, err
			}
			tx := pickWaitTarget(txs.Data, txID)
			if tx == nil {
				return nil, false, nil
			}
			switch tx.Status {
			case "confirmed":
				p, err := client.Payments.Get(ctx, rail0Id)
				return p, err == nil, err
			case "failed":
				// A decision, not a read failure: the chain has spoken.
				return nil, false, stopPolling{opFailedError(op, tx)}
			}
			return nil, false, nil
		})
}

// opFailedError renders a reverted transaction: the gateway's decoded reason
// (or its code, or a bare "reverted") plus the local hint for that code.
func opFailedError(op string, tx *rail0.Transaction) error {
	// ErrorDetail, not the ErrorMessage this used to read: the gateway trimmed the error
	// shape to exactly code/title/detail and dropped the aliases, so `error_message` no
	// longer exists on a transaction. Detail is the sentence — it carries the chain's own
	// words when the revert is not one the gateway recognises — and the code is the
	// fallback below, unchanged.
	reason := tx.ErrorDetail
	if reason == "" {
		reason = tx.ErrorCode
	}
	if reason == "" {
		reason = "reverted"
	}
	if h := hintFor(tx.ErrorCode, ""); h != "" {
		return fmt.Errorf("%s failed on-chain: %s — %s", op, reason, h)
	}
	return fmt.Errorf("%s failed on-chain: %s", op, reason)
}

// pickWaitTarget returns the transaction the wait is following, or nil to keep
// polling.
//
// With a txID it matches on that id, which is the whole fix. Trusting list order
// alone was wrong for any operation that can be attempted more than once: on a
// second partial capture, capture #1 is already `confirmed` and sits at index 0,
// while capture #2's row has not been persisted yet — so the first poll read #1,
// reported "confirmed", and the command exited successfully having waited for the
// wrong transaction. The operator was told capture #2 had settled when it had only
// been broadcast.
//
// Returning nil when the id is absent from the listing is deliberate: the row
// certainly exists (the gateway returned it in the 202), so this is a read that
// raced the write, and the right response is to poll again — NOT to fall back to
// index 0, which is precisely the confusion being removed.
//
// The empty-txID path keeps the old newest-first behaviour for callers that have no
// submit response to key on. (#66)
func pickWaitTarget(txs []rail0.Transaction, txID string) *rail0.Transaction {
	if txID != "" {
		for i := range txs {
			if txs[i].ID == txID {
				return &txs[i]
			}
		}
		return nil
	}
	if len(txs) > 0 {
		return &txs[0]
	}
	return nil
}

// anyInFlight reports whether any transaction is still on its way to the chain.
//
// `pending` counts only when it is `redrivable` — signed, held by the gateway, its
// broadcast merely not yet sent. A `pending` row WITHOUT a signed transaction is a
// prepare nobody finished (a verify refusal, a sign error, a failed submit, Ctrl-C):
// nothing will ever move it, the gateway only purges it once the payment's window
// closes, and counting it made `payments wait <id>` sit out its whole deadline and
// then report an operation "still in flight" that was never sent.
func anyInFlight(txs []rail0.Transaction) bool {
	for _, t := range txs {
		switch t.Status {
		case "submitting", "submitted":
			return true
		case "pending":
			if t.Redrivable {
				return true
			}
		}
	}
	return false
}

// ── payments wait ────────────────────────────────────────────────────────────

var paymentsWaitCmd = &cobra.Command{
	Use:   "wait <rail0Id>",
	Short: "Block until a payment settles (or reaches a given status)",
	Long: `Poll a payment until it settles, then print its final state.

By default it waits until no transaction is still in flight (submitted, or signed
and pending broadcast). A prepare that was never signed is not in flight — nothing
will ever send it — so it does not hold the wait open.
With --until it instead waits for a specific payment status, and stops with an
error as soon as the payment can no longer reach it — a charge-mode payment never
becomes 'captured', a captured one never becomes 'authorized' again. An
'expired' payment is over for a capture but not for its escrow: waiting for
'released' or 'voided' keeps going.`,
	Example: `  rail0 payments wait 0xrail0Id…
  rail0 payments wait 0xrail0Id… --until captured`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validated before any request: an unknown status can never be reached, so
		// `--until capture` (the singular typo) used to cost the full two-minute
		// deadline and then report a timeout, which reads like a slow chain rather
		// than a mistyped flag. (#107)
		if waitUntil != "" && !slices.Contains(waitableStatuses, waitUntil) {
			return fmt.Errorf("unknown --until status %q; expected one of: %s",
				waitUntil, strings.Join(waitableStatuses, ", "))
		}
		client := newReadClient()
		ctx := context.Background()
		rail0Id := args[0]
		return ui.RunAPIWithMessage("Waiting…", func() (any, error) {
			// Polled at once: the payment may already be where the caller wants it.
			return pollUntil(ctx, waitTimeout, fmt.Sprintf("rail0 payments get %s", rail0Id), false,
				func(ctx context.Context) (any, bool, error) {
					if waitUntil != "" {
						p, err := client.Payments.Get(ctx, rail0Id)
						if err != nil {
							return nil, false, err
						}
						if p.Status == waitUntil {
							return p, true, nil
						}
						// Past the point where the target is reachable: stop now
						// instead of polling out the deadline over an outcome that is
						// settled. statusCanReach alone decides, and already stops on
						// voided/released/refunded (they have no successors). Checking
						// terminalStatuses first was wrong for `expired`: terminal for
						// a capture, but it still moves to released or voided, so
						// `--until released` on an expired payment failed at once.
						if !statusCanReach(p.Status, waitUntil) {
							return nil, false, stopPolling{terminalMismatchError(p, waitUntil)}
						}
						return nil, false, nil
					}
					// The payment embeds EVERY transaction; the list endpoint this used
					// to read returned its first page (25 rows), so a payment with more
					// could report settled while a later operation was still in flight.
					p, err := client.Payments.Get(ctx, rail0Id)
					if err != nil {
						return nil, false, err
					}
					if anyInFlight(p.Transactions) {
						return nil, false, nil
					}
					// Settled — but return a FRESH read, not this one. The gateway
					// commits a confirmation and the payment's new state together, yet
					// serves the payment row and its transactions from two queries: a
					// confirm landing between them shows the last transaction settled
					// beside a payment still in its previous state.
					p, err = client.Payments.Get(ctx, rail0Id)
					return p, err == nil, err
				})
		})
	},
}

func init() {
	paymentsWaitCmd.Flags().StringVar(&waitUntil, "until", "",
		"Wait until the payment reaches this status ("+strings.Join(waitableStatuses, ", ")+")")
	// The set is finite and known locally, so completing it costs nothing and
	// removes the typo class the validation above now rejects.
	_ = paymentsWaitCmd.RegisterFlagCompletionFunc("until",
		func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return waitableStatuses, cobra.ShellCompDirectiveNoFileComp
		})
	paymentsWaitCmd.Flags().DurationVar(&waitTimeout, "timeout", defaultWaitTimeout,
		"Max time to wait — the default covers the slowest chain\x27s finality")
	paymentsCmd.AddCommand(paymentsWaitCmd)
}
