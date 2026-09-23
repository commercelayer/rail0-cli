package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	if _, ok := switchResult("staging", false, none, false, nil)["settings"]; ok {
		t.Error("switch without -d must omit settings")
	}
	res := switchResult("staging", true, none, false, nil)
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
			res := switchResult("staging", false, tc.revocation, false, nil)
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

// An environment is a complete answer to "what am I talking to", so after a
// switch the env-managed settings must be exactly the target's. Applying one
// used to only WRITE its own keys and never remove any, and production declares
// just base-url — so arriving there from development left chain-id 5042002 and
// token USDC armed, pointing a production gateway at an Arc Testnet default that
// nothing had written at that moment and nothing reported.
func TestEnvSwitchClearsSettingsTheTargetDoesNotDefine(t *testing.T) {
	isolate(t)
	assumeYes = true
	t.Cleanup(func() { assumeYes = false })

	if err := switchEnv("development", false); err != nil {
		t.Fatalf("switch development: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Settings["chain-id"] == "" || cfg.Settings["token"] == "" {
		t.Fatalf("development must have written chain-id and token, got %v", cfg.Settings)
	}

	if err := switchEnv("production", false); err != nil {
		t.Fatalf("switch production: %v", err)
	}
	cfg, err = config.Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	for _, key := range []string{"chain-id", "token"} {
		if v, ok := cfg.Settings[key]; ok && v != "" {
			t.Errorf("%s survived the switch to production as %q", key, v)
		}
	}
	if got := cfg.Settings["base-url"]; got != config.DefaultBaseURL {
		t.Errorf("base-url = %q, want the production gateway", got)
	}
}

// Nothing the target DOES declare is treated as stale, so a switch between two
// environments that declare the same keys never prompts.
func TestEnvSwitchBetweenLikeEnvironmentsClearsNothing(t *testing.T) {
	isolate(t)

	if err := switchEnv("development", false); err != nil {
		t.Fatalf("switch development: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if stale := staleAfterSwitch(cfg, environments[1]); len(stale) > 0 {
		t.Errorf("development -> staging must clear nothing, got %v", stale)
	}
}

// The confirmation gates the whole switch, not just the deletion — and it is
// asked BEFORE revokeCurrentSession, which cannot be undone. Declining must
// therefore leave the config untouched AND the session alive: a switch you said
// no to must not have logged you out of where you chose to stay.
//
// `go test` has no TTY, so confirmDestructive refuses on its own here, which is
// the same path a script without --yes takes.
func TestEnvSwitchDeclinedChangesNothing(t *testing.T) {
	isolate(t)
	assumeYes = true
	if err := switchEnv("development", false); err != nil {
		t.Fatalf("switch development: %v", err)
	}
	assumeYes = false

	baseURL, _ := effectiveSetting(mustKey(t, envIdentityKey))
	if err := token.Save(baseURL, "header.payload.signature"); err != nil {
		t.Fatalf("save token: %v", err)
	}

	if err := switchEnv("production", false); err == nil {
		t.Fatal("a switch that would clear settings must refuse without confirmation")
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Settings["base-url"] != "http://localhost:9292" {
		t.Errorf("the declined switch moved base-url to %q", cfg.Settings["base-url"])
	}
	if cfg.Settings["chain-id"] != "5042002" {
		t.Errorf("the declined switch cleared chain-id (%q)", cfg.Settings["chain-id"])
	}
	if token.Load(baseURL) == "" {
		t.Error("the declined switch revoked the session it was going to leave")
	}
}

// The session store treats `http://localhost:9292/` and `http://localhost:9292` as
// one gateway; detectEnv must too, or switching to the environment you are already
// on reads as "leaving custom" and revokes the session still in use.
func TestEnvDetectIgnoresATrailingSlashAndCase(t *testing.T) {
	isolate(t)
	dev, _ := lookupEnv("development")
	cfg := &config.Config{Settings: map[string]string{"base-url": dev.settings["base-url"] + "/"}}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	if env := detectEnv(); env != "development" {
		t.Errorf("detectEnv with a trailing slash = %q, want development", env)
	}
}

// With RAIL0_BASE_URL set the switch cannot take effect — the variable outranks the
// file — so it must be refused before anything runs, instead of revoking the session
// of the gateway every later command keeps using.
func TestEnvSwitchRefusesWhileTheBaseURLIsOverridden(t *testing.T) {
	isolate(t)
	t.Setenv("RAIL0_BASE_URL", "https://rail0-gateway-staging.fly.dev")
	if err := saveTokenForTest("header.payload.sig"); err != nil {
		t.Fatalf("seed token: %v", err)
	}

	err := switchEnv("development", false)
	if err == nil || !strings.Contains(err.Error(), "RAIL0_BASE_URL") {
		t.Fatalf("switch under an override: err = %v, want a refusal naming RAIL0_BASE_URL", err)
	}
	if token.Load("https://rail0-gateway-staging.fly.dev") == "" {
		t.Error("the refused switch deleted the session of the gateway still in use")
	}
	cfg, _ := config.Load()
	if cfg.Settings["base-url"] != "" {
		t.Errorf("a refused switch must not write the config, got %q", cfg.Settings["base-url"])
	}

	// An override that already points at the target changes nothing: allowed.
	if err := switchEnv("staging", false); err != nil {
		t.Errorf("switch to the environment the override names: %v", err)
	}
}

// The --base-url FLAG outranks the file just as RAIL0_BASE_URL does, so it must
// refuse a switch that could not take effect — and say which override to remove.
func TestEnvSwitchRefusesWhileTheBaseURLFlagIsSet(t *testing.T) {
	isolate(t)
	baseURL = "https://rail0-gateway-staging.fly.dev"
	t.Cleanup(func() { baseURL = "" })

	err := switchEnv("development", false)
	if err == nil || !strings.Contains(err.Error(), "--base-url") {
		t.Fatalf("switch under --base-url: err = %v, want a refusal naming the flag", err)
	}
	if cfg, _ := config.Load(); cfg.Settings["base-url"] != "" {
		t.Errorf("a refused switch must not write the config, got %q", cfg.Settings["base-url"])
	}
	// The same URL, spelled with a trailing slash and upper case, names the target.
	baseURL = "HTTPS://rail0-gateway-staging.fly.dev/"
	if err := switchEnv("staging", false); err != nil {
		t.Errorf("an override naming the target (normalised) must be allowed: %v", err)
	}
}

// A switch whose config save FAILS must leave the session of the gateway you are still
// on alone. It used to revoke first and save second, so a failed save logged you out of
// the environment you had not left.
func TestEnvSwitchThatCannotSaveKeepsTheSession(t *testing.T) {
	isolate(t)
	isolateTokenStore(t)
	if os.Geteuid() == 0 {
		t.Skip("root writes a read-only directory")
	}
	staging := "https://rail0-gateway-staging.fly.dev"
	if err := (&config.Config{Settings: map[string]string{"base-url": staging}}).Save(); err != nil {
		t.Fatal(err)
	}
	if err := token.Save(staging, makeJWT(t, map[string]any{"sub": "0xme", "exp": time.Now().Add(time.Hour).Unix()})); err != nil {
		t.Fatal(err)
	}
	path, _ := config.Path()
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	if err := switchEnv("development", false); err == nil {
		t.Fatal("the switch must fail when the config cannot be written")
	}
	if token.Load(staging) == "" {
		t.Error("a switch that did not happen revoked the session of the gateway still in use")
	}
}
