package cmd

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"

	rail0 "github.com/rail0/go-sdk"
)

// humanizeError turns low-level transport and API errors into clear, actionable
// CLI messages. Anything it doesn't recognise (e.g. cobra usage errors) is
// returned unchanged.
func humanizeError(err error) error {
	if err == nil {
		return nil
	}
	base := resolveBaseURL()

	// API responded with a non-2xx — map the common statuses to a hint.
	var apiErr *rail0.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.Status == 401:
			// The gateway distinguishes missing_authorization from token_expired and
			// invalid_token; say which one rather than collapsing all three.
			return fmt.Errorf("not authorized (HTTP 401)%s — run 'rail0 auth login' first", detail(apiHint(apiErr)))
		case apiErr.Status == 403:
			return fmt.Errorf("forbidden (HTTP 403)%s", detail(apiHint(apiErr)))
		case apiErr.Status == 404:
			return fmt.Errorf("not found (HTTP 404)%s", detail(apiHint(apiErr)))
		case apiErr.Status == 409:
			return fmt.Errorf("already exists / conflict (HTTP 409)%s", detail(apiHint(apiErr)))
		case apiErr.Status == 422:
			return fmt.Errorf("request rejected (HTTP 422)%s", detail(apiHint(apiErr)))
		case apiErr.Status == 429:
			return fmt.Errorf("rate limited (HTTP 429)%s", detail(rateLimitHint(apiErr)))
		case apiErr.Status == 501:
			return fmt.Errorf("the gateway cannot handle this request (HTTP 501)%s", detail(apiHint(apiErr)))
		case apiErr.Status >= 500:
			// Surface the gateway's own detail (e.g. signing_payload_unavailable /
			// rpc_unreachable) instead of masking every 5xx as "try again".
			return fmt.Errorf("the RAIL0 gateway reported an upstream error (HTTP %d)%s", apiErr.Status, detail(apiHint(apiErr)))
		default:
			return err // keep the SDK's message for anything else
		}
	}

	// No API response — a transport problem.
	var dnsErr *net.DNSError
	switch {
	case errors.As(err, &dnsErr):
		return fmt.Errorf("can't resolve the API host for %s — check --base-url / RAIL0_BASE_URL", base)
	case errors.Is(err, syscall.ECONNREFUSED) || strings.Contains(err.Error(), "connection refused"):
		return fmt.Errorf("can't reach the RAIL0 gateway at %s — is it running? (set --base-url or RAIL0_BASE_URL)", base)
	case isTimeout(err) || strings.Contains(err.Error(), "deadline exceeded"):
		return fmt.Errorf("the request to %s timed out", base)
	}
	return err
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// detail appends ": <msg>" when msg is non-empty, for tacking a server-provided
// reason onto a friendly prefix.
func detail(msg string) string {
	if strings.TrimSpace(msg) == "" {
		return ""
	}
	return ": " + msg
}

// apiHint composes what the user reads after the HTTP status: the gateway's own
// sentence first, then this CLI's extra advice for that code.
//
// The gateway sends code/title/detail, so `Detail` is a sentence written to be shown
// as-is and is always present — it is the text, not a fallback. The local hint is
// appended only when it says something the gateway cannot: which rail0 command to run
// next. Falling back to the bare Code stays for an older gateway, whose bodies could
// carry a code and nothing else.
func apiHint(e *rail0.APIError) string {
	msg := strings.TrimSpace(e.Detail)
	if msg == "" {
		msg = strings.TrimSpace(e.Message)
	}
	if msg == "" {
		msg = e.Code
	}
	if r := strings.TrimSpace(e.Reason); r != "" && !strings.Contains(msg, r) {
		msg = strings.TrimSpace(msg + " (" + r + ")")
	}
	if h := hintFor(e.Code, e.Reason); h != "" {
		msg = strings.TrimSpace(msg + " — " + h)
	}
	return msg
}

// rateLimitHint composes what follows "rate limited (HTTP 429)".
//
// The gateway shapes its own 429 through API::Errors and its detail already names the
// window — "Rate limit reached. Retry in 60 seconds." — so that sentence is preferred
// and the parsed Retry-After is deliberately NOT appended on top of it: the same
// number twice in one line reads like two different facts.
//
// The header is the fallback for a 429 the gateway did not produce. An upstream proxy,
// CDN or load balancer sends Retry-After with no rail0 error body, and that is exactly
// the case where the CLI has nothing else to tell the user. (#49)
func rateLimitHint(e *rail0.APIError) string {
	if msg := strings.TrimSpace(apiHint(e)); msg != "" {
		return msg
	}
	if e.RetryAfter > 0 {
		return fmt.Sprintf("retry in %s", e.RetryAfter.Round(time.Second))
	}
	return "retry shortly"
}

// errorHints maps a gateway/contract error code to an actionable next step, so a
// rejected request explains what to do — mirroring the reasons the rail0-admin UI
// shows on a locked action. Keyed by the machine code (state guards and, for a
// contract_revert, the snake_case Solidity error).
var errorHints = map[string]string{
	// payment-state guards (HTTP 422)
	"amount_exceeds_capturable":    "amount is above the capturable balance — see capturableAmount in 'rail0 payments get <id>'",
	"amount_exceeds_refundable":    "amount is above the refundable balance — see refundableAmount in 'rail0 payments get <id>'",
	"not_capturable":               "the payment must be 'authorized' or 'partially_captured' to capture",
	"not_voidable":                 "void is only allowed while 'authorized' with nothing captured — use 'release' for the remainder after a capture",
	"not_releasable":               "release opens only after authorizationExpiry — check it with 'rail0 payments get <id>'",
	"not_refundable":               "nothing is refundable — the payment must be charged/captured and within the refund window",
	"not_signable":                 "the payment must be 'unsigned' to sign",
	"already_signed":               "the payer signature is already stored — the payee can act now",
	"no_signature":                 "the payer hasn't signed yet — run 'rail0 payments create' / 'payments sign' first",
	"wrong_mode":                   "this operation doesn't match the payment's mode (authorize vs charge)",
	"already_disputed":             "a dispute is already open — close it with 'rail0 payments dispute close <id>' first",
	"not_disputed":                 "there is no open dispute to close",
	"nothing_to_dispute":           "a dispute needs a merchant-held (refundable) balance",
	"transaction_not_overwritable": "a transaction for this operation is already in flight — wait for it with 'rail0 payments wait <id>'",
	"signer_mismatch":              "the signing key doesn't match the payment's payer/payee — use the right wallet's key",
	"config_hash_mismatch":         "this payment and its on-chain deployment disagree — check it with 'rail0 payments get <id>'; it can't be operated as recorded",
	"payment_not_on_chain":         "the contract has no record of this payment — check its transactions with 'rail0 payments transactions <id>'",
	"unsupported_contract_version": "this payment's RAIL0 deployment isn't one the gateway supports — nothing to do client-side, the gateway needs upgrading",
	// token-level reverts: the token refused, not RAIL0
	"insufficient_token_balance":  "the paying wallet is short of the token — check it with 'rail0 wallets balances <id-or-address>'",
	"invalid_token_signature":     "the EIP-3009 signature didn't recover to the paying wallet — wrong key, chain, token or amount; re-run the command with the right -p key",
	"authorization_already_used":  "that authorization was already spent — each is single-use, create a fresh payment",
	"authorization_not_yet_valid": "the authorization isn't valid yet (validAfter is in the future)",
	"token_account_blocked":       "the token issuer has blocklisted one of the wallets in this transfer",
	"token_paused":                "the token contract is paused by its issuer — nothing can settle right now",
	// broadcast rejections: never reached the chain
	"insufficient_gas_funds":  "the sending wallet can't cover gas — fund it with the chain's native token ('rail0 chains' shows which)",
	"nonce_too_low":           "that nonce is already used on-chain — re-run the command to prepare a fresh transaction",
	"replacement_underpriced": "another transaction with that nonce is pending — wait for it with 'rail0 payments wait <id>'",
	"gas_price_too_low":       "the fee is below what the node accepts — re-run the command to pick up current fees",
	"already_known":           "the node already has this transaction — wait for it with 'rail0 payments wait <id>'",
	"rpc_unavailable":         "no RPC endpoint answered — the transaction was not submitted, retry in a moment",
	// authorization: which party the session is missing
	"not_the_payee":     "only the merchant (payee) can do this — 'rail0 auth login' with the payee's key",
	"not_the_payer":     "only the buyer (payer) can do this — 'rail0 auth login' with the payer's key",
	"not_a_participant": "only the payer and the payee can read a payment — sign in as one of them",
	// contract reverts (surfaced as contract_revert, or on a failed transaction)
	"not_payee":                  "only the merchant (payee) may do this — sign with the payee's key",
	"not_payer":                  "only the buyer (payer) may do this — sign with the payer's key",
	"not_payer_or_payee":         "only the payer or the payee may do this",
	"refund_expired":             "the refund window has closed (refundExpiry passed) — refund/dispute is no longer possible",
	"authorization_not_expired":  "release opens only after authorizationExpiry — wait until it passes",
	"already_captured":           "already (partially) captured — use 'release' for the remainder, not 'void'",
	"unsupported_payment_method": "the payee (merchant) doesn't accept this token/chain — check 'rail0 payment-methods --address <payee>'",
	"unknown_token":              "the token isn't configured on this chain — see 'rail0 tokens'",
	"no_active_contract":         "no active RAIL0 contract on that chain — see 'rail0 chains'",
	"missing_param":              "a required parameter is missing from the request",
	"forbidden":                  "not permitted for this session — on create the payer must be the signed-in address ('rail0 auth login' with that key)",
	"payment_already_exists":     "a payment with this id already exists on-chain",
}

// hintFor returns the actionable hint for an error code (preferred) or its
// reason sub-cause (used when a contract_revert carries the specific error there).
func hintFor(code, reason string) string {
	if h := errorHints[code]; h != "" {
		return h
	}
	return errorHints[reason]
}

// ErrAborted marks an operation the USER cancelled — Ctrl-C or ESC at a prompt, or
// declining a destructive confirmation. Distinct from a failure: nothing went wrong,
// the answer was "no".
//
// It still must not exit 0. `rail0 payments refund <id> && mark_refunded_in_db` used
// to run the second command when the operator hit Ctrl-C at the amount prompt, so a
// database was marked refunded with no refund behind it. (#65)
var ErrAborted = errors.New("aborted")

// ExitCodeAborted is the status for a user cancellation. 130 is the POSIX convention
// for a process terminated by SIGINT (128 + 2) — what a shell reports for Ctrl-C — so
// scripts and CI already recognise it, and it cannot collide with the API-derived
// codes below.
const ExitCodeAborted = 130

// ExitCodeRateLimited is the status for an HTTP 429. Distinct so a script can tell
// "slow down and retry" apart from a real failure — the one API error class where
// re-running the identical command later is the correct response. (#49)
const ExitCodeRateLimited = 10

// exitCode maps an error to a distinct process exit status so scripts can branch
// on the class of failure: 3 auth, 4 forbidden, 5 not-found, 6 conflict,
// 7 rejected (422), 8 upstream (5xx), 9 network, 10 rate-limited,
// 130 user-cancelled, 1 otherwise.
func exitCode(err error) int {
	// Checked before the API/transport classification: a cancellation is not a
	// failure of the request, and may not even have made one.
	if errors.Is(err, ErrAborted) {
		return ExitCodeAborted
	}

	var apiErr *rail0.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.Status == 0:
			return 9 // no HTTP response — a network-level failure
		case apiErr.Status == 401:
			return 3
		case apiErr.Status == 403:
			return 4
		case apiErr.Status == 404:
			return 5
		case apiErr.Status == 409:
			return 6
		case apiErr.Status == 422:
			return 7
		case apiErr.Status == 429:
			return ExitCodeRateLimited
		case apiErr.Status >= 500:
			return 8
		}
	}
	// Transport-level failure (no usable HTTP response). Mirror humanizeError's
	// detection, including the string fallbacks — the SDK wraps some network
	// errors so they no longer unwrap to net.Error / ECONNREFUSED.
	var dnsErr *net.DNSError
	var ne net.Error
	s := err.Error()
	if errors.As(err, &dnsErr) || errors.As(err, &ne) || errors.Is(err, syscall.ECONNREFUSED) ||
		isTimeout(err) ||
		strings.Contains(s, "connection refused") ||
		strings.Contains(s, "no such host") ||
		strings.Contains(s, "deadline exceeded") {
		return 9
	}
	return 1
}
