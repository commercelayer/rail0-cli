package cmd

import (
	"errors"
	"regexp"
	"strings"
	"testing"
)

func TestSIWEHost(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://api.rail0.xyz/", "api.rail0.xyz"},
		{"https://api.rail0.xyz", "api.rail0.xyz"},
		// Host-only: the port must be stripped so it matches the gateway's
		// expected SIWE domain ("localhost" in dev), not "localhost:9292".
		{"http://localhost:9292", "localhost"},
		{"http://localhost:9292/", "localhost"},
		{"http://127.0.0.1:9292", "127.0.0.1"},
		{"localhost:9292", "localhost"}, // scheme-less fallback
	}
	for _, tc := range cases {
		if got := siweHost(tc.in); got != tc.want {
			t.Errorf("siweHost(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// A JWT-shaped value, used to prove it never reaches the notice or the summary.
const fakeJWT = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIweGFh" +
	"YWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWEiLCJleHAiOjk5OTk5OTk5OTl9.c2lnbmF0dXJl"

// `auth login` used to print `Set RAIL0_TOKEN=<jwt>` when the token could not be
// saved, putting a live bearer credential into terminal scrollback, CI logs and any
// `2>` redirect (#64). The trigger is mundane — a read-only filesystem, a sandboxed
// HOME — so this is the path most likely to leak, and the least likely to be noticed.
func TestTokenSaveFailureNoticeNeverContainsTheToken(t *testing.T) {
	notice := tokenSaveFailureNotice(errors.New("open /nonexistent/token: read-only file system"))

	if strings.Contains(notice, fakeJWT) {
		t.Error("the notice must never carry the raw token")
	}
	// Nothing JWT-shaped at all: the check above only catches this exact fixture,
	// while a future edit might interpolate a different value.
	if regexp.MustCompile(`eyJ[A-Za-z0-9_-]{8,}`).MatchString(notice) {
		t.Errorf("the notice contains something JWT-shaped: %q", notice)
	}
	// It must still be actionable — the failure and the way out. The way out is no
	// longer "--token, then export RAIL0_TOKEN": that variable was removed (#60), so
	// printing the token buys nothing and the notice points at the cache directory
	// instead. Asserting the recovery route here, not just the absence of a secret,
	// is what caught the two changes drifting apart during the rebase.
	for _, want := range []string{"could not save token", "read-only file system", "sign in again"} {
		if !strings.Contains(notice, want) {
			t.Errorf("notice is missing %q: %q", want, notice)
		}
	}
}
