package cmd

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"testing"

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
		{"forbidden", &rail0.APIError{Status: 403, Message: "caller is not the payee"}, "forbidden (HTTP 403): caller is not the payee"},
		{"conflict", &rail0.APIError{Status: 409, Code: "conflict"}, "conflict (HTTP 409)"},
		{"server error", &rail0.APIError{Status: 502}, "server error (HTTP 502)"},
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
