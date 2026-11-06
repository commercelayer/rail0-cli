package cmd

import (
	"encoding/json"
	"io"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/config"
)

func TestConfigPrecedenceAndOrigin(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("RAIL0_BASE_URL", "") // ensure the env doesn't shadow the file

	k := configKeys["base-url"]

	// Nothing set → the active environment's (production's) default.
	if v, origin := effectiveSetting(k); v != config.DefaultBaseURL || origin != "production default" {
		t.Fatalf("unset: got %q (%s)", v, origin)
	}
	// A global key set nowhere has no value; the commands use the CLI's fallback.
	if _, _, ok := configuredSetting(configKeys["http-timeout"]); ok {
		t.Fatal("unset global key reported as set")
	}
	if v, origin := effectiveSetting(configKeys["http-timeout"]); v != "30" || origin != originUnset {
		t.Fatalf("unset http-timeout: got %q (%s)", v, origin)
	}

	// Persisted in the config file, in the active environment's block → origin is the file.
	cfg, _ := config.Load()
	cfg.SetEnvSetting("production", "base-url", "http://localhost:9292")
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if v, origin := effectiveSetting(k); v != "http://localhost:9292" || origin != "config file" {
		t.Errorf("file: got %q (%s)", v, origin)
	}
	// Another environment's block does not apply.
	cfg.Environments = nil
	cfg.SetEnvSetting("staging", "base-url", "http://localhost:9292")
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if v, origin := effectiveSetting(k); v != config.DefaultBaseURL || origin != "production default" {
		t.Errorf("other environment's block applied: got %q (%s)", v, origin)
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

// The whole point of #106: an environment switch has to be able to carry the chain
// and the token, which means they have to BE settable keys. Asserted as an exact set
// in BOTH directions — a key cannot be dropped, and none can be added silently.
//
// The negative half is the substantive one. Addresses and signing keys are
// deliberately absent: the alias book and the OS keychain already spare the retyping,
// and a stored payer/payee/key would make the two parties and the signer of a
// fund-moving command resolve from a file instead of from the command line.
func TestConfigKeysAreExactlyTheNonSensitiveDefaults(t *testing.T) {
	want := []string{"admin-url", "base-url", "chain", "chain-id", "http-timeout", "output", "token"}
	got := make([]string, 0, len(configKeys))
	for name := range configKeys {
		got = append(got, name)
	}
	sort.Strings(got)
	if !slices.Equal(got, want) {
		t.Errorf("configKeys = %v, want %v", got, want)
	}
	for _, name := range []string{"payer", "payee", "key", "private-key"} {
		if _, err := lookupKey(name); err == nil {
			t.Errorf("%q must NOT be settable in the config file", name)
		}
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

	for _, name := range []string{"output", "http-timeout", "chain-id", "admin-url"} {
		if got := configuredDefault(name); got != "" {
			t.Errorf("configuredDefault(%q) = %q, want empty when unset", name, got)
		}
	}
	// A per-environment key with a built-in default IS configured: the commands
	// fill the flag with it.
	if got := configuredDefault("token"); got != "USDC" {
		t.Errorf("configuredDefault(token) = %q, want production's default", got)
	}

	cfg, _ := config.Load()
	cfg.SetEnvSetting("production", "chain-id", "5042002")
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if got := configuredDefault("chain-id"); got != "5042002" {
		t.Errorf("configuredDefault(chain-id) = %q after config set", got)
	}
}

// clearConfigEnv points the config at an empty temp dir and unsets every key's env
// var, so a test sees only what it writes.
func clearConfigEnv(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	for _, k := range configKeys {
		t.Setenv(k.env, "")
	}
}

// A config key either is in effect, and then has a value and an origin, or it is not
// set at all. The listing used to show every key with `default` and `modified`
// columns, including an empty default for chain-id — a key that looked present
// without a value. The environment's defaults ARE values, listed with their origin.
func TestConfigListShowsOnlyTheKeysThatAreSet(t *testing.T) {
	clearConfigEnv(t)
	cfg, _ := config.Load()
	cfg.SetEnvSetting("production", "chain-id", "5042002")
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RAIL0_TOKEN_SYMBOL", "EURC")

	out := captureStdout(t, func() {
		if err := configCmd.RunE(configCmd, nil); err != nil {
			t.Fatal(err)
		}
	})
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("listing is not JSON: %v\n%s", err, out)
	}
	got := map[string]map[string]any{}
	for _, r := range rows {
		got[r["key"].(string)] = r
	}
	if len(got) != 4 || got["chain-id"] == nil || got["token"] == nil || got["chain"] == nil || got["base-url"] == nil {
		t.Fatalf("want exactly base-url, chain, chain-id and token, got %v", rows)
	}
	if got["chain-id"]["origin"] != "config file" || got["token"]["origin"] != "env (RAIL0_TOKEN_SYMBOL)" ||
		got["chain"]["origin"] != "production default" || got["token"]["value"] != "EURC" {
		t.Errorf("origins: %v", rows)
	}
	for _, r := range rows {
		for _, field := range []string{"default", "modified"} {
			if _, has := r[field]; has {
				t.Errorf("row %v still carries %q", r["key"], field)
			}
		}
	}
}

func TestConfigGetOnAnUnsetKeyIsAnError(t *testing.T) {
	clearConfigEnv(t)

	err := configGetCmd.RunE(configGetCmd, []string{"admin-url"})
	if err == nil || !strings.Contains(err.Error(), "admin-url is not set") ||
		!strings.Contains(err.Error(), "the matching flag must be passed") {
		t.Errorf("admin-url: got %v", err)
	}
	err = configGetCmd.RunE(configGetCmd, []string{"http-timeout"})
	if err == nil || !strings.Contains(err.Error(), "the CLI uses 30") {
		t.Errorf("http-timeout: got %v", err)
	}
	// A key with an environment default is never "not set".
	if err := configGetCmd.RunE(configGetCmd, []string{"token"}); err != nil {
		t.Errorf("token has a default: %v", err)
	}
}

// `config set` / `config unset` on a per-environment key touch the ACTIVE
// environment's block only; a global key goes in settings.
func TestConfigSetAndUnsetFollowTheActiveEnvironment(t *testing.T) {
	clearConfigEnv(t)
	if err := (&config.Config{Environment: "staging"}).Save(); err != nil {
		t.Fatal(err)
	}
	if err := configSetCmd.RunE(configSetCmd, []string{"token", "EURC"}); err != nil {
		t.Fatal(err)
	}
	if err := configSetCmd.RunE(configSetCmd, []string{"output", "quiet"}); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.Load()
	if cfg.Environments["staging"]["token"] != "EURC" || cfg.Settings["token"] != "" {
		t.Errorf("token must be stored under staging, got %+v", cfg)
	}
	if cfg.Settings["output"] != "quiet" || cfg.Environments["staging"]["output"] != "" {
		t.Errorf("output must be stored globally, got %+v", cfg)
	}
	if v, origin := effectiveSetting(configKeys["token"]); v != "EURC" || origin != originConfigFile {
		t.Errorf("token = %q (%s)", v, origin)
	}

	// unset removes the stored value; the default applies again. A key only in
	// effect through its default is not "set in the config file".
	if err := configUnsetCmd.RunE(configUnsetCmd, []string{"token"}); err != nil {
		t.Fatal(err)
	}
	if v, origin := effectiveSetting(configKeys["token"]); v != "USDC" || origin != "staging default" {
		t.Errorf("after unset: token = %q (%s)", v, origin)
	}
	if err := configUnsetCmd.RunE(configUnsetCmd, []string{"token"}); err == nil {
		t.Error("unsetting a key that is only defaulted must error")
	}
	cfg, _ = config.Load()
	if len(cfg.Environments) != 0 {
		t.Errorf("an emptied environment block must be dropped, got %v", cfg.Environments)
	}
}

// What the CLI does for an unset key lives in the help, generated from configKeys.
func TestConfigHelpSaysWhatHappensWhenAKeyIsNotSet(t *testing.T) {
	for _, want := range []string{
		"base-url",
		"chain-id",
		"not set: the matching flag must be passed",
		"http-timeout",
		"not set: the CLI uses 30",
	} {
		if !strings.Contains(configCmd.Long, want) {
			t.Errorf("config help lacks %q", want)
		}
	}
}

// A chain or token taken from the stored settings is announced, with where it came
// from: a configured chain-id used to be applied silently, and a refusal then named
// a chain the user had never typed.
func TestCreateAnnouncesTheChainAndTokenItTookFromTheSettings(t *testing.T) {
	clearConfigEnv(t)
	seedChainCatalog(t, rail0.Blockchain{ChainID: 5042002, Name: "Arc Testnet"})
	cfg, _ := config.Load()
	cfg.SetEnvSetting("production", "chain-id", "5042002")
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RAIL0_TOKEN_SYMBOL", "USDC")

	f := createPaymentFlags{}
	f.applyConfiguredDefaults()
	if f.chainID != 5042002 || f.token != "USDC" {
		t.Fatalf("defaults not applied: chain %d token %q", f.chainID, f.token)
	}
	note := captureStderr(t, f.noteConfiguredDefaults)
	for _, want := range []string{"token USDC (env (RAIL0_TOKEN_SYMBOL))", "Arc Testnet (5042002) (config file)"} {
		if !strings.Contains(note, want) {
			t.Errorf("note %q lacks %q", note, want)
		}
	}

	// Flags the user passed are not "from config", so nothing is announced.
	g := createPaymentFlags{chainID: 84532, token: "USDC"}
	g.applyConfiguredDefaults()
	if out := captureStderr(t, g.noteConfiguredDefaults); out != "" {
		t.Errorf("announced flags the user typed: %q", out)
	}
}

// captureStderr returns what fn writes to stderr.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	defer func() { os.Stderr = old }()
	fn()
	_ = w.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
