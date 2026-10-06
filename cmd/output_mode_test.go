package cmd

import "testing"

// Combined output flags never error: one wins silently in the fixed order
// quiet > pretty > json, which the README documents.
func TestOutputFlagModePrecedence(t *testing.T) {
	t.Cleanup(func() { jsonOut, prettyOut, quietOut = false, false, false })

	cases := []struct {
		json, pretty, quiet bool
		want                string
	}{
		{want: ""},
		{json: true, want: "json"},
		{pretty: true, want: "pretty"},
		{quiet: true, want: "quiet"},
		{json: true, pretty: true, want: "pretty"},
		{json: true, quiet: true, want: "quiet"},
		{pretty: true, quiet: true, want: "quiet"},
		{json: true, pretty: true, quiet: true, want: "quiet"},
	}
	for _, c := range cases {
		jsonOut, prettyOut, quietOut = c.json, c.pretty, c.quiet
		if got := outputFlagMode(); got != c.want {
			t.Errorf("json=%v pretty=%v quiet=%v: got %q, want %q", c.json, c.pretty, c.quiet, got, c.want)
		}
	}
}
