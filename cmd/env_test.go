package cmd

import (
	"strings"
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

// A switch writes ONE thing — the active environment — and every per-environment
// key then resolves against it: staging's gateway, with staging's default chain and
// token, and the origin says so.
func TestEnvSwitchIsRememberedForEveryCommand(t *testing.T) {
	isolate(t)

	if err := switchEnv("staging", false); err != nil {
		t.Fatalf("switch staging: %v", err)
	}
	cfg, _ := config.Load()
	if cfg.Environment != "staging" {
		t.Errorf("environment not persisted: %q", cfg.Environment)
	}
	if len(cfg.Environments) != 0 || len(cfg.Settings) != 0 {
		t.Errorf("a switch must write nothing but the environment, got %+v", cfg)
	}
	if env := activeEnv(); env != "staging" {
		t.Errorf("activeEnv after switch: %q", env)
	}
	if v, origin := effectiveSetting(configKeys["base-url"]); v != "https://rail0-gateway-staging.fly.dev" || origin != "staging default" {
		t.Errorf("effective base-url after switch: %q (%s)", v, origin)
	}
	if v, origin := effectiveSetting(configKeys["chain"]); v != "arc" || origin != "staging default" {
		t.Errorf("effective chain after switch: %q (%s)", v, origin)
	}
	if v, _ := effectiveSetting(configKeys["token"]); v != "USDC" {
		t.Errorf("effective token after switch: %q", v)
	}
}

// With nothing written the CLI is on production — the environment the published
// binary is for — with production's gateway, chain and token.
func TestEnvDefaultIsProductionWithItsDefaults(t *testing.T) {
	isolate(t)
	if env := activeEnv(); env != "production" {
		t.Errorf("fresh config should be production, got %q", env)
	}
	if v, origin := effectiveSetting(configKeys["base-url"]); v != config.DefaultBaseURL || origin != "production default" {
		t.Errorf("base-url: %q (%s)", v, origin)
	}
	if v, origin := effectiveSetting(configKeys["chain"]); v != "arc" || origin != "production default" {
		t.Errorf("chain: %q (%s)", v, origin)
	}
	if v, _, ok := configuredSetting(configKeys["admin-url"]); ok {
		t.Errorf("production has no admin-url default, got %q", v)
	}
}

// A config file naming an environment the CLI does not know (hand-edited) falls back
// to production rather than resolving every per-environment key to nothing.
func TestEnvUnknownNameInTheFileFallsBackToProduction(t *testing.T) {
	isolate(t)
	if err := (&config.Config{Environment: "prod"}).Save(); err != nil {
		t.Fatal(err)
	}
	unknownEnvWarned = false
	t.Cleanup(func() { unknownEnvWarned = false })
	if env := activeEnv(); env != "production" {
		t.Errorf("unknown environment should fall back to production, got %q", env)
	}
}

func TestEnvUnknownName(t *testing.T) {
	if _, err := lookupEnv("prod"); err == nil {
		t.Error("unknown environment must error")
	}
	if _, err := lookupEnv("staging"); err != nil {
		t.Errorf("known environment rejected: %v", err)
	}
	isolate(t)
	if err := switchEnv("prod", false); err == nil {
		t.Error("switching to an unknown environment must error")
	}
}

// Every environment defaults the chain and the token, so no payment command needs
// -c/-t anywhere — production included: it serves one chain, so arc is a decision,
// not an invention.
func TestEveryEnvironmentDefaultsChainAndToken(t *testing.T) {
	for _, e := range environments {
		for _, key := range []string{"base-url", "chain", "token"} {
			if e.defaults[key] == "" {
				t.Errorf("%s has no default %s", e.name, key)
			}
		}
	}
}

// `env` is for the people who run development and staging: it stays out of the
// help, the completion and the generated reference, while `rail0 env --help` and the
// command itself keep working.
func TestEnvCommandIsHiddenButRunnable(t *testing.T) {
	if !envCmd.Hidden {
		t.Error("env must be hidden")
	}
	if envCmd.IsAvailableCommand() {
		t.Error("a hidden env must not be listed as available (help, docs)")
	}
	if sub, _, err := rootCmd.Find([]string{"env", "current"}); err != nil || sub == nil || sub.Name() != "current" {
		t.Errorf("env current: not found (%v)", err)
	}
	if envCmd.RunE == nil {
		t.Error("env should run bare (list) with a non-nil RunE")
	}
	if envCmd.PersistentFlags().Lookup("details") == nil {
		t.Error("env should have a persistent --details flag")
	}
	if envCmd.PersistentFlags().ShorthandLookup("d") == nil {
		t.Error("env --details should have a -d shorthand")
	}
	var b strings.Builder
	rootCmd.SetOut(&b)
	t.Cleanup(func() { rootCmd.SetOut(nil) })
	if err := rootCmd.UsageFunc()(rootCmd); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), " env ") {
		t.Errorf("root usage lists env:\n%s", b.String())
	}
}

func TestEnvSwitchDetailsGating(t *testing.T) {
	// The switch confirmation lists parameters only with -d; bare it is just
	// status + env name.
	if _, ok := switchResult("staging", false, false)["settings"]; ok {
		t.Error("switch without -d must omit settings")
	}
	res := switchResult("staging", true, false)
	if _, ok := res["settings"]; !ok {
		t.Error("switch with -d must include settings")
	}
	if res["logged_in"] != false {
		t.Errorf("logged_in must be reported, got %v", res)
	}
}

// -d must mean the same thing on all three forms of `env`: bare answers "which
// environments can I pick?"; -d adds what each one is made of — the user's stored
// values over the built-in defaults.
func TestEnvListDetailsGating(t *testing.T) {
	isolate(t)
	cfg := &config.Config{Environment: "staging"}
	cfg.SetEnvSetting("staging", "token", "EURC")
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	bare, err := environmentRows(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(bare) != len(environments) {
		t.Fatalf("listing must name every environment: got %d, want %d", len(bare), len(environments))
	}
	for _, row := range bare {
		if _, ok := row["settings"]; ok {
			t.Errorf("listing without -d must omit settings, got %v", row)
		}
		if row["current"] != (row["env"] == "staging") {
			t.Errorf("current must mark the active environment, got %v", row)
		}
	}

	detailed, _ := environmentRows(true)
	for _, row := range detailed {
		settings, ok := row["settings"].(map[string]string)
		if !ok {
			t.Fatalf("listing with -d must include settings, got %v", row)
		}
		want := "USDC"
		if row["env"] == "staging" {
			want = "EURC" // the stored value over the default
		}
		if settings["token"] != want || settings["chain"] != "arc" {
			t.Errorf("%s: settings %v", row["env"], settings)
		}
	}
}

// Sessions are per gateway and a switch no longer ends any: the session of the
// environment you leave is still there when you come back, so hopping between
// staging and production never costs a sign-in.
func TestEnvSwitchKeepsEverySession(t *testing.T) {
	isolate(t)
	isolateTokenStore(t)

	if err := switchEnv("staging", false); err != nil {
		t.Fatalf("switch staging: %v", err)
	}
	stagingURL := resolveBaseURL()
	if err := token.Save(stagingURL, "header.payload.signature"); err != nil {
		t.Fatalf("save token: %v", err)
	}
	if err := switchEnv("development", false); err != nil {
		t.Fatalf("switch development: %v", err)
	}
	if token.Load(stagingURL) == "" {
		t.Error("switching away removed the previous gateway's session")
	}
	if err := switchEnv("staging", false); err != nil {
		t.Fatalf("switch back: %v", err)
	}
	if token.Load(resolveBaseURL()) == "" {
		t.Error("the staging session is gone after switching back")
	}
}

// Each environment keeps its own stored values: a chain set on development is
// development's, and switching to production and back changes nothing in either.
func TestEnvironmentsKeepTheirOwnSettings(t *testing.T) {
	isolate(t)
	if err := switchEnv("development", false); err != nil {
		t.Fatal(err)
	}
	if err := configSetCmd.RunE(configSetCmd, []string{"chain", "84532"}); err != nil {
		t.Fatal(err)
	}
	if err := switchEnv("production", false); err != nil {
		t.Fatal(err)
	}
	if v, origin := effectiveSetting(configKeys["chain"]); v != "arc" || origin != "production default" {
		t.Errorf("production chain = %q (%s): development's value leaked", v, origin)
	}
	if err := switchEnv("development", false); err != nil {
		t.Fatal(err)
	}
	if v, origin := effectiveSetting(configKeys["chain"]); v != "84532" || origin != originConfigFile {
		t.Errorf("development chain = %q (%s), want the stored 84532", v, origin)
	}
}

// With RAIL0_BASE_URL set the switch cannot take effect — the variable outranks the
// file — so it must be refused before anything is written.
func TestEnvSwitchRefusesWhileTheBaseURLIsOverridden(t *testing.T) {
	isolate(t)
	t.Setenv("RAIL0_BASE_URL", "https://rail0-gateway-staging.fly.dev")

	err := switchEnv("development", false)
	if err == nil || !strings.Contains(err.Error(), "RAIL0_BASE_URL") {
		t.Fatalf("switch under an override: err = %v, want a refusal naming RAIL0_BASE_URL", err)
	}
	cfg, _ := config.Load()
	if cfg.Environment != "" {
		t.Errorf("a refused switch must not write the config, got %q", cfg.Environment)
	}

	// An override that already points at the target changes nothing: allowed.
	if err := switchEnv("staging", false); err != nil {
		t.Errorf("switch to the environment the override names: %v", err)
	}
}

// The --base-url FLAG outranks the file just as RAIL0_BASE_URL does, so it must
// refuse a switch that could not take effect — and say which override to remove.
// The target's gateway is the one the user STORED for it, when there is one.
func TestEnvSwitchRefusesWhileTheBaseURLFlagIsSet(t *testing.T) {
	isolate(t)
	baseURL = "https://rail0-gateway-staging.fly.dev"
	t.Cleanup(func() { baseURL = "" })

	err := switchEnv("development", false)
	if err == nil || !strings.Contains(err.Error(), "--base-url") {
		t.Fatalf("switch under --base-url: err = %v, want a refusal naming the flag", err)
	}
	if cfg, _ := config.Load(); cfg.Environment != "" {
		t.Errorf("a refused switch must not write the config, got %q", cfg.Environment)
	}
	// The same URL, spelled with a trailing slash and upper case, names the target.
	baseURL = "HTTPS://rail0-gateway-staging.fly.dev/"
	if err := switchEnv("staging", false); err != nil {
		t.Errorf("an override naming the target (normalized) must be allowed: %v", err)
	}

	// A development gateway stored on another port IS development's gateway.
	cfg, _ := config.Load()
	cfg.SetEnvSetting("development", "base-url", "http://localhost:9393")
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	baseURL = "http://localhost:9393"
	if err := switchEnv("development", false); err != nil {
		t.Errorf("an override naming the stored development gateway must be allowed: %v", err)
	}
}
