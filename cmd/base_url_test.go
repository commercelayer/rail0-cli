package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestValidateBaseURL(t *testing.T) {
	cases := []struct {
		name, in string
		wantErr  string // "" = must be accepted
	}{
		// Accepted.
		{"https production", "https://api.rail0.xyz", ""},
		{"https with port", "https://rail0-gateway-staging.fly.dev:8443", ""},
		{"http localhost is the dev default", "http://localhost:9292", ""},
		{"http 127.0.0.1", "http://127.0.0.1:9292", ""},
		{"http IPv6 loopback", "http://[::1]:9292", ""},
		{"http .localhost subdomain", "http://gw.localhost:9292", ""},
		{"https localhost", "https://localhost:9292", ""},

		// Rejected: plaintext to a real host. This is the case the issue is about.
		{"http to a public host", "http://api.rail0.xyz", "plaintext http://"},
		{"http to an arbitrary host", "http://evil.example.com", "plaintext http://"},
		{"http to a public IP", "http://198.51.100.7:9292", "plaintext http://"},
		// 127.0.0.1 is loopback; 127.x is too, but a lookalike host name is not.
		{"http to a localhost lookalike", "http://localhost.evil.com", "plaintext http://"},

		// Rejected: not a usable URL at all.
		{"empty", "", "must be an http(s) URL"},
		{"no scheme", "api.rail0.xyz", "must be an http(s) URL"},
		{"no host", "https://", "must be an http(s) URL"},
		{"wrong scheme", "ftp://api.rail0.xyz", "must be an http(s) URL"},
		{"file scheme", "file:///etc/passwd", "must be an http(s) URL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBaseURL(tc.in)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("validateBaseURL(%q) = %v, want accepted", tc.in, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateBaseURL(%q) = nil, want error containing %q", tc.in, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("validateBaseURL(%q) = %v, want it to mention %q", tc.in, err, tc.wantErr)
			}
		})
	}
}

// The env var path is what the issue reports as unchecked: effectiveSetting used to
// return it verbatim, so it reached the client without ever meeting validateBaseURL.
func TestCheckBaseURLRejectsAPlaintextEnvVar(t *testing.T) {
	t.Setenv("RAIL0_BASE_URL", "http://evil.example.com")
	if err := checkBaseURL(paymentsCmd); err == nil {
		t.Fatal("checkBaseURL accepted a plaintext RAIL0_BASE_URL for a gateway command")
	}
}

// Same for the flag path.
func TestCheckBaseURLRejectsAPlaintextFlag(t *testing.T) {
	prev := baseURL
	t.Cleanup(func() { baseURL = prev })
	baseURL = "http://evil.example.com"
	if err := checkBaseURL(paymentsCmd); err == nil {
		t.Fatal("checkBaseURL accepted a plaintext --base-url for a gateway command")
	}
}

// The recovery path: with a bad base URL in the environment, the commands that
// exist to INSPECT and FIX it must still run. Otherwise the CLI locks the user out
// of its own repair tools.
func TestCheckBaseURLExemptsTheLocalOnlyCommands(t *testing.T) {
	t.Setenv("RAIL0_BASE_URL", "http://evil.example.com")
	for _, name := range []string{"config", "env", "keys", "aliases", "version", "token", "init"} {
		cmd, _, err := rootCmd.Find([]string{name})
		if err != nil {
			t.Fatalf("command %q not found in the tree: %v", name, err)
		}
		if err := checkBaseURL(cmd); err != nil {
			t.Errorf("checkBaseURL blocked %q, which never contacts the gateway: %v", name, err)
		}
	}
}

// TAB completion runs through cobra's hidden __complete command, which cobra adds at
// Execute time. A bad base URL used to make every completion return its error.
func TestCheckBaseURLLeavesShellCompletionAlone(t *testing.T) {
	t.Setenv("RAIL0_BASE_URL", "http://evil.example.com")
	for _, name := range []string{cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd} {
		if err := checkBaseURL(&cobra.Command{Use: name}); err != nil {
			t.Errorf("checkBaseURL blocked %q: %v", name, err)
		}
	}
}

// The exemption is keyed on the whole tree, so a nested leaf resolves to its
// first-level ancestor rather than its own name.
func TestTopLevelName(t *testing.T) {
	cases := []struct{ args, want string }{
		{"payments", "payments"},
		{"payments capture", "payments"},
		{"config set", "config"},
		{"keys add", "keys"},
	}
	for _, tc := range cases {
		cmd, _, err := rootCmd.Find(strings.Fields(tc.args))
		if err != nil {
			t.Fatalf("Find(%q): %v", tc.args, err)
		}
		if got := topLevelName(cmd); got != tc.want {
			t.Errorf("topLevelName(%q) = %q, want %q", tc.args, got, tc.want)
		}
	}
	if got := topLevelName(rootCmd); got != "rail0" {
		t.Errorf("topLevelName(rootCmd) = %q, want rail0", got)
	}
}

// Guards the assumption checkBaseURL relies on: it is wired as the ROOT
// PersistentPreRunE, and cobra runs only the closest one it finds walking up from
// the executed command. A subcommand defining its own would silently disable the
// base-url check for that whole tree.
func TestNoSubcommandShadowsTheRootPersistentPreRunE(t *testing.T) {
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		for _, sub := range c.Commands() {
			if sub.PersistentPreRunE != nil || sub.PersistentPreRun != nil {
				t.Errorf("%q defines its own PersistentPreRun(E), which shadows the root base-url check", sub.CommandPath())
			}
			walk(sub)
		}
	}
	walk(rootCmd)
}
