package cmd

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"

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
			return errors.New("not authorized (HTTP 401) — run 'rail0 auth login' first")
		case apiErr.Status == 403:
			return fmt.Errorf("forbidden (HTTP 403)%s", detail(apiHint(apiErr)))
		case apiErr.Status == 404:
			return fmt.Errorf("not found (HTTP 404)%s", detail(apiHint(apiErr)))
		case apiErr.Status == 409:
			return fmt.Errorf("already exists / conflict (HTTP 409)%s", detail(apiHint(apiErr)))
		case apiErr.Status == 422:
			return fmt.Errorf("request rejected (HTTP 422)%s", detail(apiHint(apiErr)))
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

// apiHint returns the gateway's human message, or its machine-readable code when
// there's no message — so even bodies like {"status":"no_active_contract"} (no
// message field) still produce an informative hint instead of nothing. The
// optional `reason` sub-cause is appended when the message doesn't already carry it.
func apiHint(e *rail0.APIError) string {
	msg := e.Message
	if strings.TrimSpace(msg) == "" {
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
	// contract reverts (surfaced as contract_revert, or on a failed transaction)
	"not_payee":                 "only the merchant (payee) may do this — sign with the payee's key",
	"not_payer":                 "only the buyer (payer) may do this — sign with the payer's key",
	"not_payer_or_payee":        "only the payer or the payee may do this",
	"refund_expired":            "the refund window has closed (refundExpiry passed) — refund/dispute is no longer possible",
	"authorization_not_expired": "release opens only after authorizationExpiry — wait until it passes",
	"already_captured":          "already (partially) captured — use 'release' for the remainder, not 'void'",
	"token_not_accepted":        "the token isn't in this deployment's allowlist",
	"payment_already_exists":    "a payment with this id already exists on-chain",
}

// hintFor returns the actionable hint for an error code (preferred) or its
// reason sub-cause (used when a contract_revert carries the specific error there).
func hintFor(code, reason string) string {
	if h := errorHints[code]; h != "" {
		return h
	}
	return errorHints[reason]
}

// exitCode maps an error to a distinct process exit status so scripts can branch
// on the class of failure: 3 auth, 4 forbidden, 5 not-found, 6 conflict,
// 7 rejected (422), 8 upstream (5xx), 9 network, 1 otherwise.
func exitCode(err error) int {
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
