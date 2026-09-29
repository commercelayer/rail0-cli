package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/spf13/cobra"
)

// The exact scenario from #66: a second partial capture. The gateway lists
// transactions newest-first, but capture #2's row is not in the listing yet (the
// 202 has been answered, the row not yet visible to this read), so index 0 is the
// ALREADY CONFIRMED capture #1. Keying on index reported that stale confirmation as
// this attempt's success.
func TestPickWaitTargetIgnoresAnEarlierConfirmedAttempt(t *testing.T) {
	listing := []rail0.Transaction{
		{ID: "tx_capture_1", Operation: "capture", Status: "confirmed"},
	}

	if got := pickWaitTarget(listing, "tx_capture_2"); got != nil {
		t.Fatalf("pickWaitTarget returned %+v for an id not in the listing; want nil so the caller polls again", got)
	}

	// Once the row does appear, it is the one followed — not index 0.
	listing = append([]rail0.Transaction{{ID: "tx_capture_2", Operation: "capture", Status: "submitted"}}, listing...)
	got := pickWaitTarget(listing, "tx_capture_2")
	if got == nil {
		t.Fatal("pickWaitTarget returned nil for an id present in the listing")
	}
	if got.ID != "tx_capture_2" {
		t.Errorf("followed %q, want tx_capture_2", got.ID)
	}
	if got.Status != "submitted" {
		t.Errorf("status = %q, want submitted (the confirmed row is a DIFFERENT attempt)", got.Status)
	}
}

// Order-independence: the row is found wherever the gateway happens to place it.
func TestPickWaitTargetMatchesRegardlessOfPosition(t *testing.T) {
	listing := []rail0.Transaction{
		{ID: "tx_3", Status: "confirmed"},
		{ID: "tx_2", Status: "failed"},
		{ID: "tx_1", Status: "confirmed"},
	}
	for _, want := range []string{"tx_1", "tx_2", "tx_3"} {
		got := pickWaitTarget(listing, want)
		if got == nil || got.ID != want {
			t.Errorf("pickWaitTarget(_, %q) = %+v, want the row with that id", want, got)
		}
	}
}

// Without an id the previous newest-first behaviour is preserved, so a caller with
// no submit response to key on is unaffected.
func TestPickWaitTargetFallsBackToNewestFirst(t *testing.T) {
	if got := pickWaitTarget(nil, ""); got != nil {
		t.Errorf("pickWaitTarget(nil, \"\") = %+v, want nil", got)
	}
	listing := []rail0.Transaction{{ID: "newest"}, {ID: "older"}}
	got := pickWaitTarget(listing, "")
	if got == nil || got.ID != "newest" {
		t.Errorf("pickWaitTarget(_, \"\") = %+v, want the newest row", got)
	}
}

// submittedTransactionID must read the id out of what the submit methods actually
// return, and degrade to "" (newest-first) on anything else rather than panicking
// after the operation has already been broadcast.
func TestSubmittedTransactionID(t *testing.T) {
	if got := submittedTransactionID(&rail0.Transaction{ID: "tx_42"}); got != "tx_42" {
		t.Errorf("submittedTransactionID(*Transaction) = %q, want tx_42", got)
	}
	for _, other := range []any{nil, "a string", rail0.Transaction{ID: "not a pointer"}, &rail0.Payment{}} {
		if got := submittedTransactionID(other); got != "" {
			t.Errorf("submittedTransactionID(%T) = %q, want \"\"", other, got)
		}
	}
}

// ── the polling loop (#107) ──────────────────────────────────────────────────
//
// Everything below is about what happens AFTER an operation has been
// broadcast, which is why a wrong answer here is expensive: the funds have
// already moved (or are about to), and the only question is what the CLI tells
// the operator — or the script — about it.

// fastPolls shortens the interval so a test exercises the loop instead of
// sleeping through it.
func fastPolls(t *testing.T) {
	t.Helper()
	original := waitPollInterval
	waitPollInterval = time.Millisecond
	t.Cleanup(func() { waitPollInterval = original })
}

// A single 429 or 502 while polling used to abort the wait, so a command exited
// non-zero for an operation that was in flight and probably fine.
func TestPollUntilToleratesTransientErrorsThenSucceeds(t *testing.T) {
	fastPolls(t)
	calls := 0
	got, err := pollUntil(context.Background(), time.Minute, "rail0 payments get 0xabc", false,
		func(context.Context) (any, bool, error) {
			calls++
			switch calls {
			case 1:
				return nil, false, &rail0.APIError{Status: 429}
			case 2:
				return nil, false, errors.New("dial tcp: connection reset by peer")
			case 3:
				return nil, false, &rail0.APIError{Status: 502}
			}
			return "settled", true, nil
		})
	if err != nil {
		t.Fatalf("a wait must survive transient reads: %v", err)
	}
	if got != "settled" {
		t.Errorf("result = %v, want the settled value", got)
	}
	if calls != 4 {
		t.Errorf("checked %d times, want 4 (three tolerated failures, then success)", calls)
	}
}

// The mirror image: a 4xx is the gateway's verdict, not a blip, so polling it
// for two minutes only delays the message.
func TestPollUntilAbortsOnANonTransientError(t *testing.T) {
	fastPolls(t)
	calls := 0
	_, err := pollUntil(context.Background(), time.Minute, "rail0 payments get 0xabc", false,
		func(context.Context) (any, bool, error) {
			calls++
			return nil, false, &rail0.APIError{Status: 404, Detail: "payment not found"}
		})
	if err == nil {
		t.Fatal("a 404 must end the wait")
	}
	if calls != 1 {
		t.Errorf("checked %d times, want 1 — a 404 is not worth retrying", calls)
	}
}

// A decision (a reverted transaction, a terminal status) must surface at once
// and must NOT be mistaken for a failed read.
func TestPollUntilSurfacesADecisionImmediately(t *testing.T) {
	fastPolls(t)
	decision := errors.New("capture failed on-chain: AuthorizationExpired")
	calls := 0
	_, err := pollUntil(context.Background(), time.Minute, "rail0 payments get 0xabc", false,
		func(context.Context) (any, bool, error) {
			calls++
			return nil, false, stopPolling{decision}
		})
	if !errors.Is(err, decision) {
		t.Fatalf("error = %v, want the decision unwrapped", err)
	}
	if calls != 1 {
		t.Errorf("checked %d times, want 1", calls)
	}
}

// Giving up must not imply failure, and must hand over the command that
// answers the question — plus the blip it spent itself on.
func TestPollUntilGiveUpNamesTheCheckCommandAndTheLastError(t *testing.T) {
	fastPolls(t)
	_, err := pollUntil(context.Background(), 5*time.Millisecond, "rail0 payments get 0xdeadbeef", false,
		func(context.Context) (any, bool, error) {
			return nil, false, &rail0.APIError{Status: 503, Detail: "upstream busy"}
		})
	if err == nil {
		t.Fatal("expected a give-up error")
	}
	for _, want := range []string{"may still be in flight", "rail0 payments get 0xdeadbeef", "last read error"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("give-up message %q is missing %q", err.Error(), want)
		}
	}
}

// A cancelled context is a user interrupt: exit 130, and say the operation is
// still running rather than dying silently.
func TestPollUntilInterruptedReportsTheOperationIsInFlight(t *testing.T) {
	fastPolls(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := pollUntil(ctx, time.Minute, "rail0 payments get 0xabc", false,
		func(context.Context) (any, bool, error) { return nil, false, nil })
	if !errors.Is(err, ErrAborted) {
		t.Fatalf("error = %v, want ErrAborted so the exit code is %d", err, ExitCodeAborted)
	}
	if !strings.Contains(err.Error(), "still in flight") {
		t.Errorf("interrupt message %q must say the operation is still running", err.Error())
	}
	if code := exitCode(err); code != ExitCodeAborted {
		t.Errorf("exitCode = %d, want %d", code, ExitCodeAborted)
	}
}

func TestTransientPollErrorClassification(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"429 rate limit", &rail0.APIError{Status: 429}, true},
		{"502 upstream", &rail0.APIError{Status: 502}, true},
		{"503 upstream", &rail0.APIError{Status: 503}, true},
		{"404 the gateway's verdict", &rail0.APIError{Status: 404}, false},
		{"401 session gone", &rail0.APIError{Status: 401}, false},
		{"422 rejected", &rail0.APIError{Status: 422}, false},
		{"no HTTP answer at all", errors.New("dial tcp: i/o timeout"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := transientPollError(tc.err); got != tc.want {
				t.Errorf("transientPollError = %v, want %v", got, tc.want)
			}
		})
	}
}

// --until on a payment that ended elsewhere: the old loop burned the whole
// deadline and then said "timed out", hiding a settled outcome behind a message
// that reads like slowness.
func TestTerminalMismatchErrorExplainsTheOutcome(t *testing.T) {
	err := terminalMismatchError(&rail0.Payment{
		Status:           "voided",
		LastErrorCode:    "authorization_expired",
		LastErrorMessage: "the authorization window closed",
	}, "captured")
	for _, want := range []string{"voided", "terminal", "captured", "the authorization window closed"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q is missing %q", err.Error(), want)
		}
	}
}

// `expired` is the status that would otherwise cost the operator the whole deadline AND
// the right answer: no capture can land, so waiting is over — but the escrow is still
// on-chain, so "terminal" must not read as "the funds are settled".
func TestExpiredIsTerminalButNamesTheRecovery(t *testing.T) {
	if !terminalStatuses["expired"] {
		t.Fatal("expired must stop the wait: nothing will move it to captured")
	}
	err := terminalMismatchError(&rail0.Payment{ID: "0xabc", Status: "expired"}, "captured")
	for _, want := range []string{"expired", "terminal", "recoverable", "release", "0xabc"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q is missing %q", err.Error(), want)
		}
	}
}

// The list backing --until must exclude the legacy status the gateway no longer
// produces: accepting it would accept a wait that can never finish.
func TestWaitableStatusesExcludeTheLegacyPartiallyRefunded(t *testing.T) {
	if contains(waitableStatuses, "partially_refunded") {
		t.Error("partially_refunded is no longer produced; --until on it can never complete")
	}
	for _, s := range []string{"captured", "refunded", "authorized", "expired"} {
		if !contains(waitableStatuses, s) {
			t.Errorf("waitableStatuses is missing %q", s)
		}
	}
	for _, s := range []string{"voided", "released", "refunded", "expired"} {
		if !terminalStatuses[s] {
			t.Errorf("%q must be terminal", s)
		}
	}
	for _, s := range []string{"authorized", "partially_captured", "captured", "charged"} {
		if terminalStatuses[s] {
			t.Errorf("%q is not terminal — a refund or capture can still follow", s)
		}
	}
}

// `failed` is a TRANSACTION status. It spent a release inside the payment
// vocabulary, which cost twice: the --status completions on `payments list` and
// `analytics` offered a value the gateway answers 400 to (its filter is validated
// against Payment.aasm.states), and `--until failed` passed validation and then
// waited out the whole deadline on a status no payment can reach — the exact
// failure that validation exists to prevent. Pinned here so it cannot drift back.
func TestFailedIsNotAPaymentStatus(t *testing.T) {
	if contains(paymentStatuses, "failed") {
		t.Error("failed is a transaction status; the gateway rejects it as a payment filter")
	}
	if contains(waitableStatuses, "failed") {
		t.Error("no payment ever reaches `failed`; --until on it can only time out")
	}
	if terminalStatuses["failed"] {
		t.Error("terminalStatuses must not carry a status no payment can hold")
	}
}

// The filter vocabulary must match the gateway's own (Payment.aasm.states) exactly:
// a value it omits cannot be filtered on from the CLI, and one it invents is a 400.
// `partially_refunded` is the entry that separates this list from waitableStatuses —
// no longer produced, but real on historical rows, so still queryable.
func TestPaymentStatusesMatchTheGatewayVocabulary(t *testing.T) {
	gateway := []string{
		"unsigned", "signed", "authorized", "charged", "captured",
		"partially_captured", "expired", "voided", "released",
		"refunded", "partially_refunded",
	}
	for _, s := range gateway {
		if !contains(paymentStatuses, s) {
			t.Errorf("paymentStatuses is missing %q, which the gateway can report", s)
		}
	}
	for _, s := range paymentStatuses {
		if !contains(gateway, s) {
			t.Errorf("paymentStatuses carries %q, which the gateway's filter rejects", s)
		}
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// A `pending` row holds the wait open only when it is signed and waiting for its
// broadcast (redrivable). An abandoned prepare — never signed — will never move, and
// counting it made `payments wait` sit out its deadline over nothing.
func TestAnyInFlightIgnoresAnAbandonedPrepare(t *testing.T) {
	cases := []struct {
		name string
		txs  []rail0.Transaction
		want bool
	}{
		{"abandoned prepare", []rail0.Transaction{{Status: "pending"}, {Status: "confirmed"}}, false},
		{"signed, awaiting broadcast", []rail0.Transaction{{Status: "pending", Redrivable: true}}, true},
		{"submitting", []rail0.Transaction{{Status: "submitting"}}, true},
		{"submitted", []rail0.Transaction{{Status: "confirmed"}, {Status: "submitted"}}, true},
		{"all settled", []rail0.Transaction{{Status: "confirmed"}, {Status: "failed"}}, false},
	}
	for _, c := range cases {
		if got := anyInFlight(c.txs); got != c.want {
			t.Errorf("%s: anyInFlight = %v, want %v", c.name, got, c.want)
		}
	}
}

// --until stops as soon as the target is out of reach, not only on a terminal status.
func TestStatusCanReach(t *testing.T) {
	cases := []struct {
		from, to string
		want     bool
	}{
		{"signed", "captured", true},
		{"authorized", "refunded", true},
		{"partially_captured", "captured", true},
		{"expired", "released", true},
		{"unsigned", "charged", true},
		// the two scenarios that used to cost the whole deadline
		{"captured", "authorized", false},
		{"charged", "captured", false},
		{"voided", "captured", false},
		{"expired", "captured", false},
		// a status the CLI does not know yet keeps the wait going
		{"some_future_status", "captured", true},
	}
	for _, c := range cases {
		if got := statusCanReach(c.from, c.to); got != c.want {
			t.Errorf("statusCanReach(%q, %q) = %v, want %v", c.from, c.to, got, c.want)
		}
	}
	msg := terminalMismatchError(&rail0.Payment{Status: "charged"}, "captured").Error()
	if strings.Contains(msg, "terminal") || !strings.Contains(msg, "can no longer become captured") {
		t.Errorf("a non-terminal mismatch must not claim terminality: %q", msg)
	}
}

// `payments wait --until authorized` on a payment already captured stops at the first
// poll with the reason, instead of sitting out the whole deadline and reporting a
// timeout.
func TestPaymentsWaitUntilStopsOnAnUnreachableTarget(t *testing.T) {
	g := newLifecycleGateway(t) // serves the payment as `captured`
	fastPolls(t)
	waitUntil, waitTimeout = "authorized", time.Minute
	t.Cleanup(func() { waitUntil, waitTimeout = "", defaultWaitTimeout })

	started := time.Now()
	var err error
	_ = captureStdout(t, func() { err = paymentsWaitCmd.RunE(paymentsWaitCmd, []string{lcRail0ID}) })
	if err == nil || !strings.Contains(err.Error(), "can no longer become authorized") {
		t.Fatalf("err = %v, want the unreachable-target stop", err)
	}
	if time.Since(started) > 5*time.Second {
		t.Errorf("took %s: the wait must stop at the first poll", time.Since(started))
	}
	if n := g.sent("GET /payments/" + lcRail0ID); n != 1 {
		t.Errorf("polled %d times, want 1", n)
	}
}

// `expired` is terminal for a capture but not for the escrow: a release or a void can
// still land. `--until released` (or voided) on an expired payment used to fail at
// the first poll because terminalStatuses was consulted before statusCanReach; it
// must keep polling until the release shows up. `--until captured` still stops.
func TestPaymentsWaitUntilReleasedOutlastsExpired(t *testing.T) {
	for _, target := range []string{"released", "voided"} {
		t.Run(target, func(t *testing.T) {
			isolate(t)
			isolateTokenStore(t)
			fastPolls(t)
			var polls int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				polls++
				status := "expired"
				if polls >= 3 {
					status = target
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "pay-1", "rail0_id": lcRail0ID, "status": status})
			}))
			defer srv.Close()
			t.Setenv("RAIL0_BASE_URL", srv.URL)
			waitUntil, waitTimeout = target, time.Minute
			t.Cleanup(func() { waitUntil, waitTimeout = "", defaultWaitTimeout })

			var err error
			out := captureStdout(t, func() { err = paymentsWaitCmd.RunE(paymentsWaitCmd, []string{lcRail0ID}) })
			if err != nil {
				t.Fatalf("--until %s on an expired payment must keep waiting, got %v", target, err)
			}
			if polls != 3 || !strings.Contains(out, `"`+target+`"`) {
				t.Errorf("polls = %d, out = %s; want the wait to end on %s at the third poll", polls, out, target)
			}
		})
	}

	isolate(t)
	isolateTokenStore(t)
	fastPolls(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "pay-1", "rail0_id": lcRail0ID, "status": "expired"})
	}))
	defer srv.Close()
	t.Setenv("RAIL0_BASE_URL", srv.URL)
	waitUntil, waitTimeout = "captured", time.Minute
	t.Cleanup(func() { waitUntil, waitTimeout = "", defaultWaitTimeout })
	var err error
	_ = captureStdout(t, func() { err = paymentsWaitCmd.RunE(paymentsWaitCmd, []string{lcRail0ID}) })
	if err == nil || !strings.Contains(err.Error(), "terminal") || !strings.Contains(err.Error(), "release") {
		t.Errorf("--until captured on an expired payment must still stop, naming the recovery; got %v", err)
	}
}

// The schedule: flat for the fast phase — the chains that settle in seconds must see
// no change — then half again per poll, never past the cap.
func TestNextPollDelayBacksOffAfterTheFastPhaseUpToTheCap(t *testing.T) {
	if got := nextPollDelay(10*time.Second, waitPollInterval); got != waitPollInterval {
		t.Errorf("inside the fast phase: delay = %s, want the base %s", got, waitPollInterval)
	}
	delay := waitPollInterval
	var seen []time.Duration
	for range 8 {
		delay = nextPollDelay(time.Minute, delay)
		seen = append(seen, delay)
	}
	want := []time.Duration{3 * time.Second, 4500 * time.Millisecond, 6750 * time.Millisecond,
		10 * time.Second, 10 * time.Second, 10 * time.Second, 10 * time.Second, 10 * time.Second}
	if !slices.Equal(seen, want) {
		t.Errorf("after the fast phase: delays = %v, want %v", seen, want)
	}
}

// A 15-minute wait on a chain whose settlement never comes used to cost 451 reads at
// a flat 2s. Counted by walking the schedule, so the budget is pinned without sleeping.
func TestPollScheduleBoundsAFullWait(t *testing.T) {
	elapsed, delay, reads := time.Duration(0), waitPollInterval, 1
	for elapsed < defaultWaitTimeout {
		delay = nextPollDelay(elapsed, delay)
		elapsed += delay
		reads++
	}
	if reads > 110 {
		t.Errorf("a full %s wait makes %d reads; want at most 110 (it was 451 at a flat 2s)",
			defaultWaitTimeout, reads)
	}
}

// After a submit the first read waits one interval: the 202 means queued, so a read
// milliseconds later can only say "not yet". Without delayFirst it still goes out at
// once — a replayed row, or `payments wait`, may already be settled.
func TestPollUntilDelayFirstSkipsTheImmediateRead(t *testing.T) {
	original := waitPollInterval
	waitPollInterval = 50 * time.Millisecond
	t.Cleanup(func() { waitPollInterval = original })

	for _, delayFirst := range []bool{false, true} {
		started := time.Now()
		var firstAt time.Duration
		_, err := pollUntil(context.Background(), time.Minute, "rail0 payments get 0xabc", delayFirst,
			func(context.Context) (any, bool, error) {
				firstAt = time.Since(started)
				return "settled", true, nil
			})
		if err != nil {
			t.Fatalf("delayFirst=%v: %v", delayFirst, err)
		}
		if delayed := firstAt >= waitPollInterval; delayed != delayFirst {
			t.Errorf("delayFirst=%v: first read after %s (interval %s)", delayFirst, firstAt, waitPollInterval)
		}
	}
}

// The pause before the last read is cut to the deadline instead of overshooting it by
// a whole interval — which, at the 10s cap, would stretch a wait past its --timeout.
func TestPollUntilDoesNotSleepPastTheDeadline(t *testing.T) {
	original := waitPollInterval
	waitPollInterval = time.Hour
	t.Cleanup(func() { waitPollInterval = original })

	started := time.Now()
	_, err := pollUntil(context.Background(), 50*time.Millisecond, "rail0 payments get 0xabc", false,
		func(context.Context) (any, bool, error) { return nil, false, nil })
	if !errors.Is(err, ErrWaitGaveUp) {
		t.Fatalf("err = %v, want ErrWaitGaveUp", err)
	}
	if took := time.Since(started); took > 5*time.Second {
		t.Errorf("gave up after %s with a 50ms timeout; the pause must stop at the deadline", took)
	}
}

// A 429 carries Retry-After: the next read waits that long instead of the (shorter)
// poll interval, which would only earn another 429 from the same spent budget.
func TestPollUntilHonoursRetryAfterOnA429(t *testing.T) {
	fastPolls(t)
	const retryAfter = 80 * time.Millisecond
	started := time.Now()
	var secondAt time.Duration
	calls := 0
	_, err := pollUntil(context.Background(), time.Minute, "rail0 payments get 0xabc", false,
		func(context.Context) (any, bool, error) {
			calls++
			if calls == 1 {
				return nil, false, &rail0.APIError{Status: http.StatusTooManyRequests, RetryAfter: retryAfter}
			}
			secondAt = time.Since(started)
			return "settled", true, nil
		})
	if err != nil {
		t.Fatalf("pollUntil: %v", err)
	}
	if secondAt < retryAfter {
		t.Errorf("second read after %s; want at least the Retry-After of %s", secondAt, retryAfter)
	}
}

// `payments wait` without --until reads the payment's EMBEDDED transactions — every one
// of them. The list endpoint it used to read served its first page (25 rows), so the
// in-flight 30th operation of a long-lived payment was invisible and the wait ended
// early. Once nothing is in flight it returns a FRESH read of the payment: the gateway
// serves the row and its transactions from two queries, so the read that shows the last
// confirmation can carry the payment's previous state.
func TestPaymentsWaitReadsEveryEmbeddedTransactionAndReturnsAFreshPayment(t *testing.T) {
	isolate(t)
	isolateTokenStore(t)
	fastPolls(t)
	waitUntil, waitTimeout = "", time.Minute
	t.Cleanup(func() { waitUntil, waitTimeout = "", defaultWaitTimeout })

	var mu sync.Mutex
	gets, listCalls := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/transactions") {
			listCalls++
			_, _ = w.Write([]byte(`[]`))
			return
		}
		gets++
		txs := make([]map[string]any, 30)
		for i := range txs {
			txs[i] = map[string]any{"id": fmt.Sprintf("tx-%d", i), "operation": "capture", "status": "confirmed"}
		}
		status := "authorized"
		switch {
		case gets <= 2:
			txs[29]["status"] = "submitted" // the newest, past any first page
		case gets >= 4:
			status = "captured" // the fresh read after the settled one
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "pay-1", "rail0_id": "0xabc", "status": status, "transactions": txs})
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RAIL0_BASE_URL", srv.URL)

	var err error
	out := captureStdout(t, func() { err = paymentsWaitCmd.RunE(paymentsWaitCmd, []string{"0xabc"}) })
	if err != nil {
		t.Fatalf("payments wait: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if listCalls != 0 {
		t.Errorf("GET …/transactions called %d times; the payment embeds every row", listCalls)
	}
	if gets != 4 {
		t.Errorf("GET /payments/:id called %d times; want 2 in flight, 1 settled, 1 fresh", gets)
	}
	if !strings.Contains(out, `"captured"`) {
		t.Errorf("output = %s; want the fresh read (captured), not the settled one", out)
	}
}

// The settlement-derived default: 10× the chain's measured p90, never under 5m nor
// over the fixed 15m it replaces, and exactly that 15m when there is no measurement.
func TestSettlementWaitTimeout(t *testing.T) {
	secs := func(n int) *int { return &n }
	cases := []struct {
		name string
		p90  *int
		want time.Duration
	}{
		{"unmeasured chain keeps the fixed default", nil, defaultWaitTimeout},
		{"a non-positive figure is no measurement", secs(0), defaultWaitTimeout},
		{"a tiny p90 is floored at 5m", secs(3), 5 * time.Minute},
		{"exactly at the floor", secs(30), 5 * time.Minute},
		{"in between it is 10× p90", secs(45), 7*time.Minute + 30*time.Second},
		{"exactly at the ceiling", secs(90), 15 * time.Minute},
		{"a huge p90 is capped at the old default", secs(660), defaultWaitTimeout},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := settlementWaitTimeout(tc.p90); got != tc.want {
				t.Errorf("settlementWaitTimeout = %s, want %s", got, tc.want)
			}
		})
	}
}

// seedChainCatalog makes chainCatalog answer `chains` without a gateway. The Once is
// spent for good (it is process-wide), but the list is restored afterwards so a later
// test sees what it would have.
func seedChainCatalog(t *testing.T, chains ...rail0.Blockchain) {
	t.Helper()
	chainCatalogOnce.Do(func() {})
	previous := chainCatalogList
	chainCatalogList = chains
	t.Cleanup(func() { chainCatalogList = previous })
}

// The default is read from the payment's chain in the cached catalog; --timeout,
// when passed, wins whatever the chain says — including a value equal to the default,
// which is why "passed" is tracked rather than inferred from the value.
func TestWaitTimeoutFor(t *testing.T) {
	p90 := 12
	seedChainCatalog(t,
		rail0.Blockchain{ChainID: 5042002, Settlement: rail0.BlockchainSettlement{P90Seconds: &p90, SampleSize: 40}},
		rail0.Blockchain{ChainID: 421614, Settlement: rail0.BlockchainSettlement{SampleSize: 3}},
	)
	t.Cleanup(func() { waitTimeout, waitTimeoutExplicit = defaultWaitTimeout, false })
	waitTimeout, waitTimeoutExplicit = defaultWaitTimeout, false

	if got := waitTimeoutFor(5042002); got != 5*time.Minute {
		t.Errorf("measured chain: %s, want 5m (10× 12s, floored)", got)
	}
	if got := waitTimeoutFor(421614); got != defaultWaitTimeout {
		t.Errorf("chain below the minimum sample: %s, want the fixed default", got)
	}
	if got := waitTimeoutFor(1); got != defaultWaitTimeout {
		t.Errorf("chain outside the catalog: %s, want the fixed default", got)
	}
	if got := waitTimeoutFor(0); got != defaultWaitTimeout {
		t.Errorf("unknown chain: %s, want the fixed default", got)
	}

	// Through the flag, as a user sets it.
	fs := paymentsWaitCmd.Flags()
	if err := fs.Set("timeout", "15m"); err != nil {
		t.Fatalf("set --timeout: %v", err)
	}
	if got := waitTimeoutFor(5042002); got != 15*time.Minute {
		t.Errorf("explicit --timeout 15m: %s — an explicit value must win over the chain", got)
	}
	if err := fs.Set("timeout", "90s"); err != nil {
		t.Fatalf("set --timeout: %v", err)
	}
	if got := waitTimeoutFor(421614); got != 90*time.Second {
		t.Errorf("explicit --timeout 90s: %s", got)
	}
}

// The help still names the fixed default, and --timeout still takes a duration.
func TestTimeoutFlagDefault(t *testing.T) {
	for _, c := range []*cobra.Command{paymentsWaitCmd, paymentsCaptureCmd, paymentsRedriveCmd} {
		f := c.Flags().Lookup("timeout")
		if f == nil {
			t.Fatalf("%s has no --timeout", c.CommandPath())
		}
		if f.DefValue != "15m0s" || f.Value.Type() != "duration" {
			t.Errorf("%s --timeout: default %q type %q, want 15m0s duration", c.CommandPath(), f.DefValue, f.Value.Type())
		}
	}
	if err := (timeoutFlag{}).Set("soon"); err == nil {
		t.Error("--timeout soon should be rejected as a duration")
	}
}

// `payments wait` sizes its default from the chain the FIRST read names, and that read
// doubles as the first poll: a payment already settled costs the same two reads (the
// poll and the fresh one) it always did.
func TestPaymentsWaitSizesTheDeadlineFromTheFirstRead(t *testing.T) {
	isolate(t)
	isolateTokenStore(t)
	fastPolls(t)
	p90 := 20
	seedChainCatalog(t, rail0.Blockchain{ChainID: 5042002, Settlement: rail0.BlockchainSettlement{P90Seconds: &p90, SampleSize: 25}})
	waitUntil, waitTimeout, waitTimeoutExplicit = "", defaultWaitTimeout, false
	t.Cleanup(func() { waitUntil, waitTimeout, waitTimeoutExplicit = "", defaultWaitTimeout, false })

	var mu sync.Mutex
	gets := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gets++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "pay-1", "chain_id": 5042002, "status": "captured"})
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RAIL0_BASE_URL", srv.URL)

	var err error
	captureStdout(t, func() { err = paymentsWaitCmd.RunE(paymentsWaitCmd, []string{"0xabc"}) })
	if err != nil {
		t.Fatalf("payments wait: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if gets != 2 {
		t.Errorf("GET /payments/:id called %d times; want 2 (the first read is the first poll)", gets)
	}
}
