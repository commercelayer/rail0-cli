package cmd

import (
	"strings"
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
	if _, err := lookupKey("private-key"); err == nil {
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

// The whole point of #106: an environment switch has to be able to carry the
// per-payment constants, which means they have to BE settable keys. Asserted as a
// set so a key cannot be dropped from the whitelist without a failure here.
func TestConfigKeysCoverThePerPaymentConstants(t *testing.T) {
	for _, name := range []string{"base-url", "chain-id", "token", "payer", "payee", "key", "output", "http-timeout"} {
		if _, err := lookupKey(name); err != nil {
			t.Errorf("%q must be a settable key: %v", name, err)
		}
	}
}

// The config file is plaintext on disk and gets pasted into issues, so a private
// key in it is a permanent compromise rather than a misconfiguration. `key` holds a
// keychain reference and nothing else — and the rejection must not echo the value
// it just refused to store.
func TestConfigKeyRefusesARawPrivateKey(t *testing.T) {
	raw := "0x58cfb681b2a04e9dc0083d8258da8c19ac7cc1750f94bb1ab7651f606df5a4d6"
	err := validateKeyReference(raw)
	if err == nil {
		t.Fatal("a raw private key must be refused as a stored 'key'")
	}
	if strings.Contains(err.Error(), raw) {
		t.Error("the error echoed the rejected key back — it must be redacted")
	}
	if !strings.Contains(err.Error(), "keys add") {
		t.Error("the error should name the command that stores a key properly")
	}
}

func TestConfigValidators(t *testing.T) {
	for _, tc := range []struct {
		name  string
		fn    func(string) error
		ok    []string
		notOk []string
	}{
		{"chain-id", validatePositiveInt("chain-id"), []string{"1", "5042002"}, []string{"", "0", "-1", "arc-testnet", "1.5"}},
		{"token", validateTokenSymbol, []string{"USDC", "EURC"}, []string{"", "usdc", "U", "TOO-LONG-SYMBOL"}},
		{"output", validateOutputMode, []string{"auto", "json", "pretty", "quiet"}, []string{"", "yaml", "JSON"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, v := range tc.ok {
				if err := tc.fn(v); err != nil {
					t.Errorf("%q rejected: %v", v, err)
				}
			}
			for _, v := range tc.notOk {
				if err := tc.fn(v); err == nil {
					t.Errorf("%q accepted, want an error", v)
				}
			}
		})
	}
}

// configuredDefault answers "did the user configure this?", so a built-in default
// must read as unconfigured: `output` defaults to "auto" and `http-timeout` to
// "30", and returning those would make every caller think a choice had been made.
func TestConfiguredDefaultIsEmptyWhenNothingWasSet(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("RAIL0_OUTPUT", "")
	t.Setenv("RAIL0_HTTP_TIMEOUT", "")

	for _, name := range []string{"output", "http-timeout", "chain-id", "payee"} {
		if got := configuredDefault(name); got != "" {
			t.Errorf("configuredDefault(%q) = %q, want empty when unset", name, got)
		}
	}

	cfg, _ := config.Load()
	cfg.Settings = map[string]string{"chain-id": "5042002"}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := configuredDefault("chain-id"); got != "5042002" {
		t.Errorf("configuredDefault(chain-id) = %q after config set", got)
	}
}
