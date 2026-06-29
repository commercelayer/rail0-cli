package cmd

import (
	"testing"

	"github.com/rail0/cli/internal/config"
)

func TestConfigPrecedenceAndOrigin(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("RAIL0_BASE_URL", "") // ensure the env doesn't shadow the file

	k := configKeys["base-url"]

	// Nothing set → built-in default.
	if v, origin := effectiveSetting(k); v != config.DefaultBaseURL || origin != "default" {
		t.Fatalf("default: got %q (%s)", v, origin)
	}

	// Persisted in the config file → origin is the file.
	cfg, _ := config.Load()
	cfg.Settings = map[string]string{"base-url": "http://localhost:9292"}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if v, origin := effectiveSetting(k); v != "http://localhost:9292" || origin != "config file" {
		t.Errorf("file: got %q (%s)", v, origin)
	}

	// Env var wins over the file.
	t.Setenv("RAIL0_BASE_URL", "https://env.example")
	if v, origin := effectiveSetting(k); v != "https://env.example" || origin != "env (RAIL0_BASE_URL)" {
		t.Errorf("env: got %q (%s)", v, origin)
	}
}

func TestConfigValidationAndUnknownKey(t *testing.T) {
	// Only whitelisted keys are accepted.
	if _, err := lookupKey("chain-id"); err == nil {
		t.Error("unknown key must error")
	}
	if _, err := lookupKey("base-url"); err != nil {
		t.Errorf("known key rejected: %v", err)
	}
	// base-url must be a valid http(s) URL.
	if err := validateBaseURL("not-a-url"); err == nil {
		t.Error("invalid base-url must error")
	}
	if err := validateBaseURL("http://localhost:9292"); err != nil {
		t.Errorf("valid base-url rejected: %v", err)
	}
}

func TestConfigCommandTree(t *testing.T) {
	// Bare `config` lists (has its own RunE) and takes no args.
	if configCmd.RunE == nil {
		t.Error("config should list when run bare (nil RunE)")
	}
	for _, verb := range []string{"set", "get", "unset", "path"} {
		if sub, _, err := configCmd.Find([]string{verb}); err != nil || sub == nil || sub.Name() != verb {
			t.Errorf("config %s: not found (%v)", verb, err)
		}
	}
}
