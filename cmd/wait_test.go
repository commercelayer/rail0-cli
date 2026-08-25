package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	rail0 "github.com/rail0/go-sdk"
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
	got, err := pollUntil(context.Background(), time.Minute, "rail0 payments get 0xabc",
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
	_, err := pollUntil(context.Background(), time.Minute, "rail0 payments get 0xabc",
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
	_, err := pollUntil(context.Background(), time.Minute, "rail0 payments get 0xabc",
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
	_, err := pollUntil(context.Background(), 5*time.Millisecond, "rail0 payments get 0xdeadbeef",
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
	_, err := pollUntil(ctx, time.Minute, "rail0 payments get 0xabc",
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

// The status list backing both --until validation and its shell completion must
// exclude the legacy status the gateway no longer produces: accepting it would
// accept a wait that can never finish.
func TestPaymentStatusesExcludeTheLegacyPartiallyRefunded(t *testing.T) {
	for _, s := range paymentStatuses {
		if s == "partially_refunded" {
			t.Error("partially_refunded is no longer produced; --until on it can never complete")
		}
	}
	for _, s := range []string{"captured", "refunded", "failed", "authorized"} {
		if !contains(paymentStatuses, s) {
			t.Errorf("paymentStatuses is missing %q", s)
		}
	}
	for _, s := range []string{"voided", "released", "refunded", "failed"} {
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

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
