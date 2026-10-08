package cmd

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"testing"
	"time"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/spf13/cobra"
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

// The payer/payee wording of a 404 is right only for payments: a webhook, wallet,
// wallet token or account belongs to the session's account, and those commands do
// not take -p at all, so the advice sent the user after the wrong thing.
func TestNotFoundWordingDependsOnTheResource(t *testing.T) {
	const partyAdvice = "payer/payee of this payment"
	notFound := &rail0.APIError{Status: 404, Code: "not_found", Detail: "webhook not found"}

	for _, c := range []*cobra.Command{
		paymentsGetCmd, paymentsCaptureCmd, paymentsDisputeCloseCmd,
		paymentsTransactionCmd, paymentsWaitCmd,
	} {
		got := humanizeErrorFor(c, notFound).Error()
		if !strings.Contains(got, partyAdvice) || !strings.Contains(got, "-p") {
			t.Errorf("%s: %q, want the payer/payee advice", c.CommandPath(), got)
		}
	}
	for _, c := range []*cobra.Command{
		webhooksGetCmd, webhooksRedeliverCmd, webhooksEventCallbacksCmd,
		walletsDeleteCmd, walletTokensDisableCmd, nil,
	} {
		got := humanizeErrorFor(c, notFound).Error()
		if strings.Contains(got, partyAdvice) || strings.Contains(got, "-p") {
			t.Errorf("%v: %q, want no payer/payee advice", c, got)
		}
		if !strings.Contains(got, "not found (HTTP 404): webhook not found") ||
			!strings.Contains(got, "not visible to this session") {
			t.Errorf("%v: %q, want the gateway detail and the neutral line", c, got)
		}
	}
	// humanizeError has no command, so it must take the neutral branch.
	if got := humanizeError(notFound).Error(); strings.Contains(got, partyAdvice) {
		t.Errorf("humanizeError: %q, want the neutral wording", got)
	}
}

// The codes the gateway added for deactivation, account scoping, release senders and
// chain lookups each carry a CLI next step, and a 400 validation_failed is shaped
// through apiHint (gateway detail + hint) instead of the SDK's bare line.
func TestHintsForAccountWalletReleaseAndChainCodes(t *testing.T) {
	for code, want := range map[string]string{
		"account_deactivated":           "void, release and refund still work",
		"wallet_deactivated":            "rail0 wallets update <wallet> --active true",
		"last_active_wallet":            "rail0 wallets create",
		"last_admin":                    "operator",
		"not_your_account":              "--account",
		"release_submitter_not_a_party": "payer or the payee",
		"unknown_chain":                 "rail0 chains",
		"validation_failed":             "--help",
	} {
		if got := hintFor(code, ""); !strings.Contains(got, want) {
			t.Errorf("hintFor(%q) = %q, want substring %q", code, got, want)
		}
	}

	e := &rail0.APIError{Status: 400, Code: "validation_failed", Detail: "amount is invalid"}
	got := humanizeError(e).Error()
	if !strings.HasPrefix(got, "invalid request (HTTP 400): amount is invalid — fix the field") {
		t.Errorf("400 = %q, want the detail followed by the validation hint", got)
	}
	if exitCode(e) != 1 {
		t.Errorf("exitCode(400) = %d, want 1 (unchanged)", exitCode(e))
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

// A give-up that spent itself on transport blips quotes the last one. That text must
// not turn the give-up into a network error: "may still be in flight" would become
// "can't reach the gateway", and exit 9 reads to a script as "nothing happened, retry".
func TestAGaveUpWaitIsNeverReportedAsANetworkError(t *testing.T) {
	for _, blip := range []error{
		errors.New("dial tcp 127.0.0.1:9292: connect: connection refused"),
		errors.New("context deadline exceeded"),
	} {
		err := gaveUpError(time.Minute, "rail0 payments wait 0xabc", blip)
		if got := humanizeError(err); !strings.Contains(got.Error(), "may still be in flight") {
			t.Errorf("humanizeError rewrote the give-up (last error %q): %v", blip, got)
		}
		if code := exitCode(err); code != 1 {
			t.Errorf("exit code for a give-up quoting %q = %d, want 1", blip, code)
		}
	}
}
