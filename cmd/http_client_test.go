package cmd

import (
	"testing"
	"time"
)

// httpTimeout drives ClientOptions.Timeout for both constructors; garbage or
// non-positive values must fall back to the SDK default (0) rather than error
// a command that was otherwise fine.
func TestHTTPTimeout(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want time.Duration
	}{
		{"unset falls back to the SDK default", "", 0},
		{"seconds are honored", "10", 10 * time.Second},
		{"garbage is ignored", "ten", 0},
		{"non-positive is ignored", "0", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("RAIL0_HTTP_TIMEOUT", tc.env)
			if got := httpTimeout(); got != tc.want {
				t.Errorf("httpTimeout() = %v, want %v", got, tc.want)
			}
		})
	}
}
