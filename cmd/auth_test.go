package cmd

import "testing"

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
