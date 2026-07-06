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
	return msg
}
