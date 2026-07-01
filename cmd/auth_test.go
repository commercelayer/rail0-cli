package cmd

import "testing"

func TestSIWEHost(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://api.rail0.xyz/", "api.rail0.xyz"},
		{"https://api.rail0.xyz", "api.rail0.xyz"},
		{"http://localhost:9292", "localhost:9292"},
		{"http://localhost:9292/", "localhost:9292"},
	}
	for _, tc := range cases {
		if got := siweHost(tc.in); got != tc.want {
			t.Errorf("siweHost(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
