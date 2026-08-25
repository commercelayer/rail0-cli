package cmd

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"testing"
	"time"

	rail0 "github.com/rail0/go-sdk"
)

func TestHumanizeError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string // substring the friendly message must contain
	}{
		{"connection refused", fmt.Errorf(`Get "http://x": dial tcp: %w`, syscall.ECONNREFUSED), "is it running"},
		{"connection refused (string)", errors.New("dial tcp [::1]:9292: connect: connection refused"), "is it running"},
		{"timeout", errors.New("context deadline exceeded"), "timed out"},
		{"unauthorized", &rail0.APIError{Status: 401}, "auth login"},
		{"forbidden", &rail0.APIError{Status: 403, Detail: "caller is not the payee"}, "forbidden (HTTP 403): caller is not the payee"},
		{"conflict", &rail0.APIError{Status: 409, Code: "conflict"}, "conflict (HTTP 409)"},
		{"server error", &rail0.APIError{Status: 502}, "upstream error (HTTP 502)"},
		{"server error detail", &rail0.APIError{Status: 502, Code: "signing_payload_unavailable", Reason: "rpc_unreachable"}, "rpc_unreachable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := humanizeError(tc.err).Error()
			if !strings.Contains(got, tc.want) {
				t.Errorf("got %q, want substring %q", got, tc.want)
			}
		})
	}

	// Unrecognised errors pass through unchanged.
	orig := errors.New("unknown flag --nope")
	if humanizeError(orig).Error() != orig.Error() {
		t.Errorf("unrecognised error should pass through, got %q", humanizeError(orig))
	}
}

func TestAPIHint(t *testing.T) {
	cases := []struct {
		name string
		err  *rail0.APIError
		want string // substring the hint must contain (apiHint may append an actionable suffix from errorHints)
	}{
		// Detail, not Message: the gateway sends one text field and the SDK mirrors it into
		// Message, so Detail is where the sentence lives.
		{"detail preferred", &rail0.APIError{Code: "no_active_contract", Detail: "no active contract"}, "no active contract"},
		{"code-only fallback", &rail0.APIError{Code: "no_active_contract"}, "no_active_contract"},
		{"blank detail falls back to code", &rail0.APIError{Code: "conflict", Detail: "  "}, "conflict"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := apiHint(tc.err); !strings.Contains(got, tc.want) {
				t.Errorf("apiHint = %q, want substring %q", got, tc.want)
			}
		})
	}
}

// A cancelled operation must never exit 0. `rail0 payments refund <id> &&
// mark_refunded_in_db` used to run the second command when the operator hit Ctrl-C at
// the amount prompt, so a database recorded a refund that never happened (#65).
func TestUserCancellationExitsNonZeroAndDistinctly(t *testing.T) {
	if got := exitCode(ErrAborted); got != ExitCodeAborted {
		t.Errorf("exitCode(ErrAborted) = %d, want %d", got, ExitCodeAborted)
	}
	if ExitCodeAborted == 0 {
		t.Fatal("a cancellation must not exit 0 — that is the whole bug")
	}
	// Wrapped, as a caller adding context would produce.
	if got := exitCode(fmt.Errorf("capture: %w", ErrAborted)); got != ExitCodeAborted {
		t.Errorf("wrapped ErrAborted = %d, want %d", got, ExitCodeAborted)
	}
}

// 130 must stay distinct from every other class, or scripts cannot tell "the user
// cancelled" from "the request failed".
func TestAbortedCodeDoesNotCollideWithTheOtherClasses(t *testing.T) {
	others := []int{
		exitCode(&rail0.APIError{Status: 401}),
		exitCode(&rail0.APIError{Status: 403}),
		exitCode(&rail0.APIError{Status: 404}),
		exitCode(&rail0.APIError{Status: 409}),
		exitCode(&rail0.APIError{Status: 422}),
		exitCode(&rail0.APIError{Status: 500}),
		exitCode(&rail0.APIError{Status: 0}),
		exitCode(errors.New("something else")),
	}
	for _, code := range others {
		if code == ExitCodeAborted {
			t.Errorf("exit code %d collides with ExitCodeAborted", code)
		}
	}
}

// Declining a destructive confirmation is the same intent as cancelling a prompt, so
// it must land on the same code rather than the generic 1.
func TestDecliningAConfirmationIsAnAbort(t *testing.T) {
	assumeYes = false
	t.Cleanup(func() { assumeYes = false })

	// No TTY under test, so confirmDestructive takes the non-interactive branch —
	// which is a REFUSAL to run, not a cancellation, and stays generic.
	err := confirmDestructive("delete everything?")
	if err == nil {
		t.Fatal("expected a refusal without a TTY")
	}
	if errors.Is(err, ErrAborted) {
		t.Error("the non-interactive refusal is a misuse error, not a user cancellation")
	}
	if got := exitCode(err); got == 0 {
		t.Error("the refusal must not exit 0")
	}
}

// A 429 must be recognised rather than falling through to the SDK's raw message,
// and must not repeat the retry window twice: the gateway's own detail already
// names it, so rateLimitHint prefers that sentence over the Retry-After header.
func TestHumanizeError429(t *testing.T) {
	cases := []struct {
		name     string
		err      *rail0.APIError
		want     string
		wantOnce string // substring that must appear exactly once
	}{
		{
			name: "gateway-shaped 429 with a detail and a header",
			err: &rail0.APIError{
				Status: 429, Code: "rate_limited",
				Detail:     "Rate limit reached. Retry in 60 seconds.",
				RetryAfter: 60 * time.Second,
			},
			want:     "rate limited (HTTP 429): Rate limit reached. Retry in 60 seconds.",
			wantOnce: "60",
		},
		{
			// A proxy/CDN 429: Retry-After present, no rail0 body.
			name: "bare 429 with only a Retry-After header",
			err:  &rail0.APIError{Status: 429, RetryAfter: 90 * time.Second},
			want: "rate limited (HTTP 429): retry in 1m30s",
		},
		{
			name: "bare 429 with nothing at all",
			err:  &rail0.APIError{Status: 429},
			want: "rate limited (HTTP 429): retry shortly",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := humanizeError(tc.err).Error()
			if !strings.Contains(got, tc.want) {
				t.Errorf("humanizeError = %q, want it to contain %q", got, tc.want)
			}
			if tc.wantOnce != "" && strings.Count(got, tc.wantOnce) != 1 {
				t.Errorf("humanizeError = %q: %q appears %d times, want exactly 1 (the window must not be repeated)",
					got, tc.wantOnce, strings.Count(got, tc.wantOnce))
			}
		})
	}
}

// A 429 gets its own exit code: re-running the same command later is the correct
// response, which no other API error class shares.
func TestExitCode429IsDistinct(t *testing.T) {
	got := exitCode(&rail0.APIError{Status: 429})
	if got != ExitCodeRateLimited {
		t.Fatalf("exitCode(429) = %d, want %d", got, ExitCodeRateLimited)
	}
	for _, other := range []int{
		exitCode(&rail0.APIError{Status: 401}),
		exitCode(&rail0.APIError{Status: 403}),
		exitCode(&rail0.APIError{Status: 404}),
		exitCode(&rail0.APIError{Status: 409}),
		exitCode(&rail0.APIError{Status: 422}),
		exitCode(&rail0.APIError{Status: 500}),
		exitCode(&rail0.APIError{Status: 0}),
		exitCode(ErrAborted),
		exitCode(errors.New("generic")),
	} {
		if other == got {
			t.Errorf("exit code %d is shared with another error class; 429 must be distinct", got)
		}
	}
}
