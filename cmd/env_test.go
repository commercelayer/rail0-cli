package cmd

import (
	"testing"

	"github.com/rail0/cli/internal/config"
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
	if _, ok := switchResult("staging", false)["settings"]; ok {
		t.Error("switch without -d must omit settings")
	}
	res := switchResult("staging", true)
	if _, ok := res["settings"]; !ok {
		t.Error("switch with -d must include settings")
	}
}
