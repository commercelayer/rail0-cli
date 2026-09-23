package cmd

import (
	"testing"

	"github.com/rail0/cli/internal/config"
	"github.com/rail0/cli/internal/token"
)

// isolate points the config dir at a temp dir and clears the base-url env
// override so tests exercise the file/default layers deterministically.
func isolate(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("RAIL0_BASE_URL", "")
}

func TestEnvSwitchWritesSettings(t *testing.T) {
	isolate(t)

	if err := switchEnv("staging", false); err != nil {
		t.Fatalf("switch staging: %v", err)
	}
	cfg, _ := config.Load()
	if got := cfg.Settings["base-url"]; got != "https://rail0-gateway-staging.fly.dev" {
		t.Errorf("staging base-url not persisted: %q", got)
	}
	// The switch flows through the normal resolution: origin is the config file.
	if v, origin := effectiveSetting(configKeys["base-url"]); v != "https://rail0-gateway-staging.fly.dev" || origin != "config file" {
		t.Errorf("effective after switch: %q (%s)", v, origin)
	}
	if env := detectEnv(); env != "staging" {
		t.Errorf("detectEnv after switch: %q", env)
	}
}

func TestEnvDetectDefaultIsProduction(t *testing.T) {
	isolate(t)
	// Nothing set: the effective base-url is the built-in default, which equals
	// production's, so `current` reports production.
	if env := detectEnv(); env != "production" {
		t.Errorf("fresh config should detect production, got %q", env)
	}
}

func TestEnvDetectCustom(t *testing.T) {
	isolate(t)
	cfg, _ := config.Load()
	cfg.Settings = map[string]string{"base-url": "http://example.test"}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if env := detectEnv(); env != "custom" {
		t.Errorf("unmatched base-url should be custom, got %q", env)
	}
}

func TestEnvUnknownName(t *testing.T) {
	if _, err := lookupEnv("prod"); err == nil {
		t.Error("unknown environment must error")
	}
	if _, err := lookupEnv("staging"); err != nil {
		t.Errorf("known environment rejected: %v", err)
	}
}

func TestEnvCommandTree(t *testing.T) {
	// Bare `env` lists / switches via its own RunE and takes at most one arg.
	if envCmd.RunE == nil {
		t.Error("env should run bare (list) with a non-nil RunE")
	}
	if sub, _, err := envCmd.Find([]string{"current"}); err != nil || sub == nil || sub.Name() != "current" {
		t.Errorf("env current: not found (%v)", err)
	}
	if envCmd.PersistentFlags().Lookup("details") == nil {
		t.Error("env should have a persistent --details flag")
	}
	if envCmd.PersistentFlags().ShorthandLookup("d") == nil {
		t.Error("env --details should have a -d shorthand")
	}
}

func TestEnvSwitchDetailsGating(t *testing.T) {
	// The switch confirmation lists parameters only with -d; bare it is just
	// status + env name.
	none := sessionRevocation{}
	if _, ok := switchResult("staging", false, none, false)["settings"]; ok {
		t.Error("switch without -d must omit settings")
	}
	res := switchResult("staging", true, none, false)
	if _, ok := res["settings"]; !ok {
		t.Error("switch with -d must include settings")
	}
}

// -d must mean the same thing on all three forms of `env`. The bare listing used
// to print every environment's settings unconditionally, so the persistent flag
// changed `env <name>` and `env current` but did nothing here — and a reader had
// no way to tell whether the parameters in front of them were the flag's doing
// or the command's. Bare answers "which environments can I pick?"; -d adds what
// each one declares.
func TestEnvListDetailsGating(t *testing.T) {
	isolate(t)

	bare := environmentRows(false)
	if len(bare) != len(environments) {
		t.Fatalf("listing must name every environment: got %d, want %d", len(bare), len(environments))
	}
	for _, row := range bare {
		if _, ok := row["settings"]; ok {
			t.Errorf("listing without -d must omit settings, got %v", row)
		}
		if _, ok := row["env"]; !ok {
			t.Errorf("listing must always name the environment, got %v", row)
		}
		if _, ok := row["current"]; !ok {
			t.Errorf("listing must always mark the current one, got %v", row)
		}
	}

	for _, row := range environmentRows(true) {
		if _, ok := row["settings"]; !ok {
			t.Errorf("listing with -d must include settings, got %v", row)
		}
	}
}

// A switch reports what became of the session on the gateway it LEFT, in the
// result rather than in a log line — so -q/--json callers see it too. Whether the
// old JWT is dead or merely abandoned on disk is part of what the switch did, and
// the difference matters: an abandoned token stays valid on that gateway until it
// expires.
func TestEnvSwitchReportsThePreviousSession(t *testing.T) {
	cases := []struct {
		name       string
		revocation sessionRevocation
		want       string
		wantDetail bool
	}{
		{"no stored session", sessionRevocation{}, "none", false},
		{"revoked on the gateway", sessionRevocation{Had: true, Revoked: true}, "revoked", false},
		{
			"kept but not revoked",
			sessionRevocation{Had: true, Why: "could not reach the gateway to revoke it"},
			"abandoned",
			true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := switchResult("staging", false, tc.revocation, false)
			if got := res["previous_session"]; got != tc.want {
				t.Errorf("previous_session = %v, want %q", got, tc.want)
			}
			_, hasDetail := res["previous_session_detail"]
			if hasDetail != tc.wantDetail {
				t.Errorf("previous_session_detail present = %v, want %v", hasDetail, tc.wantDetail)
			}
		})
	}
}

// Switching to the environment you are ALREADY on used to revoke the session it
// would have been leaving — which is the one still in use. Nothing is orphaned in
// that case (the base URL does not move), so there is nothing to revoke.
//
// `rail0 init` made this the default path rather than an edge case: it pre-selects
// the current environment and always calls through switchEnv, so accepting the
// default ended the guided setup logged out.
func TestEnvSwitchToTheSameEnvironmentKeepsTheSession(t *testing.T) {
	isolate(t)

	// Land on staging, then store a session for it.
	if err := switchEnv("staging", false); err != nil {
		t.Fatalf("switch staging: %v", err)
	}
	baseURL, _ := effectiveSetting(mustKey(t, envIdentityKey))
	if baseURL == "" {
		t.Fatal("no base-url after switching to staging")
	}
	if err := token.Save(baseURL, "header.payload.signature"); err != nil {
		t.Fatalf("save token: %v", err)
	}

	// Switching to staging again must leave it alone.
	if err := switchEnv("staging", false); err != nil {
		t.Fatalf("re-switch staging: %v", err)
	}
	if got := token.Load(baseURL); got == "" {
		t.Error("re-switching to the current environment deleted its session")
	}
}

// The guard must not weaken the real case: a switch that DOES move the base URL
// still ends the session it leaves behind.
func TestEnvSwitchToAnotherEnvironmentStillClearsTheSession(t *testing.T) {
	isolate(t)

	if err := switchEnv("staging", false); err != nil {
		t.Fatalf("switch staging: %v", err)
	}
	stagingURL, _ := effectiveSetting(mustKey(t, envIdentityKey))
	if err := token.Save(stagingURL, "header.payload.signature"); err != nil {
		t.Fatalf("save token: %v", err)
	}

	// No gateway is reachable here, so the revocation cannot be confirmed — but the
	// local token must be gone either way, which is the property that matters: a
	// bearer credential for a gateway nobody is watching any more.
	if err := switchEnv("development", false); err != nil {
		t.Fatalf("switch development: %v", err)
	}
	if got := token.Load(stagingURL); got != "" {
		t.Error("switching away left the previous gateway's session on disk")
	}
}

// mustKey resolves a setting name to the descriptor effectiveSetting takes.
func mustKey(t *testing.T, name string) configKey {
	t.Helper()
	k, err := lookupKey(name)
	if err != nil {
		t.Fatalf("lookupKey(%q): %v", name, err)
	}
	return k
}
