package cmd

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/rail0/cli/internal/config"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// configKey describes a settable CLI default: its canonical name, the env var
// that overrides the stored value, a description, the built-in default, and an
// optional validator for `config set`.
type configKey struct {
	name     string
	env      string
	desc     string
	def      func() string
	validate func(string) error
}

// configKeys is the whitelist of settable keys. Only non-sensitive endpoint /
// default values belong here — never private keys or tokens (CLAUDE.md rule 8).
// The `key` entry is the near-miss: it stores a keychain REFERENCE (@name), and
// its validator exists to guarantee a raw secret can never be written here.
//
// base-url used to be the only entry, which made `env <name>` a switch of one
// setting: every `payments create` still needed -c, -t, -F, -T and -p spelled out
// again, even though those are per-environment constants in practice. (#106)
var configKeys = map[string]configKey{
	"base-url": {
		name:     "base-url",
		env:      "RAIL0_BASE_URL",
		desc:     "RAIL0 gateway base URL",
		def:      func() string { return config.DefaultBaseURL },
		validate: validateBaseURL,
	},
	"chain-id": {
		name:     "chain-id",
		env:      "RAIL0_CHAIN_ID",
		desc:     "Default EVM chain ID for --chain-id/-c",
		def:      func() string { return "" },
		validate: validatePositiveInt("chain-id"),
	},
	"token": {
		name: "token",
		// NOT RAIL0_TOKEN: that name was a JWT source until #60, and reusing it
		// would make a stale exported session token arrive here as a payment token
		// symbol — a confusing failure in the one place a leftover RAIL0_TOKEN is
		// still likely to exist in someone's shell.
		env:      "RAIL0_TOKEN_SYMBOL",
		desc:     "Default token symbol for --token/-t (e.g. USDC)",
		def:      func() string { return "" },
		validate: validateTokenSymbol,
	},
	"payer": {
		name:     "payer",
		env:      "RAIL0_PAYER",
		desc:     "Default payer for --from/-F (0x… or @alias)",
		def:      func() string { return "" },
		validate: validateAddress,
	},
	"payee": {
		name:     "payee",
		env:      "RAIL0_PAYEE",
		desc:     "Default payee for --to/-T (0x… or @alias)",
		def:      func() string { return "" },
		validate: validateAddress,
	},
	"key": {
		name: "key",
		// Deliberately no env var. RAIL0_PRIVATE_KEY already carries a raw key for
		// one process; a second, near-identical name for a keychain reference would
		// invite pasting the secret into the wrong one.
		env:      "",
		desc:     "Default signing key for --private-key/-p, as an @keychain-name",
		def:      func() string { return "" },
		validate: validateKeyReference,
	},
	"output": {
		name:     "output",
		env:      "RAIL0_OUTPUT",
		desc:     "Default output mode: auto, json, pretty or quiet",
		def:      func() string { return "auto" },
		validate: validateOutputMode,
	},
	"http-timeout": {
		name:     "http-timeout",
		env:      "RAIL0_HTTP_TIMEOUT",
		desc:     "Per-request HTTP timeout in seconds",
		def:      func() string { return "30" },
		validate: validatePositiveInt("http-timeout"),
	},
}

// configuredDefault returns a key's effective value, or "" when it resolves to
// the built-in default. Consumers use it to fill a flag the user did not pass, so
// "" has to mean "nothing configured" — returning the built-in default would make
// `output`'s "auto" or `http-timeout`'s "30" look like a user choice and override
// the caller's own fallback.
func configuredDefault(name string) string {
	k, err := lookupKey(name)
	if err != nil {
		return ""
	}
	value, origin := effectiveSetting(k)
	if origin == "default" {
		return ""
	}
	return value
}

// validatePositiveInt builds a validator for a key whose value counts something
// (a chain id, a number of seconds): a non-number or a non-positive number is
// always a mistake, and catching it at `config set` keeps it from surfacing later
// as an unrelated-looking gateway error.
func validatePositiveInt(key string) func(string) error {
	return func(v string) error {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return fmt.Errorf("%s must be a positive integer (got %q)", key, v)
		}
		return nil
	}
}

// validateTokenSymbol checks the shape of a token symbol as the gateway lists it
// in GET /tokens (USDC, EURC): upper-case letters and digits. Purely syntactic —
// whether the symbol exists on a given chain is the gateway's call, and depends on
// a chain that may not be configured yet.
func validateTokenSymbol(v string) error {
	if !tokenSymbolPattern.MatchString(v) {
		return fmt.Errorf("token must be a symbol as listed in GET /tokens, e.g. USDC (got %q)", v)
	}
	return nil
}

var tokenSymbolPattern = regexp.MustCompile(`^[A-Z0-9]{2,12}$`)

// validateKeyReference enforces that the stored signing key is a KEYCHAIN
// REFERENCE, never the key itself.
//
// The config file is plaintext on disk, world-readable in plenty of setups, and
// gets pasted into issues; a private key there is a permanent compromise, not a
// misconfiguration. `keys add` exists precisely so the secret lives in the OS
// keychain, so this rejects anything that isn't @name — and says which command to
// run instead, because someone typing their key here has the right intent.
func validateKeyReference(v string) error {
	if !strings.HasPrefix(v, "@") {
		return fmt.Errorf(
			"key must be an @keychain-name, not a key itself — the config file is plaintext. "+
				"Save the key with 'rail0 keys add <name>' and set it as '@<name>' (got %q)",
			redactKeyish(v))
	}
	if _, err := loadKey(strings.TrimPrefix(v, "@")); err != nil {
		return err
	}
	return nil
}

// redactKeyish keeps a rejected value out of the error message when it looks like
// a secret: the whole point of the rejection is that this value must not be
// written down, and an error string ends up in scrollback and CI logs too.
func redactKeyish(v string) string {
	if len(v) > 8 {
		return v[:4] + "…(redacted)"
	}
	return v
}

func validateOutputMode(v string) error {
	switch v {
	case "auto", "json", "pretty", "quiet":
		return nil
	}
	return fmt.Errorf("output must be one of auto, json, pretty, quiet (got %q)", v)
}

// validateBaseURL rejects a base URL the CLI must not talk to. Two rules:
//
//  1. it has to be an absolute http(s) URL with a host;
//  2. plaintext http:// is allowed ONLY against a loopback host.
//
// The second rule is the substantive one. Every credential the CLI handles crosses
// this URL — the SIWE signature, the JWT it is exchanged for, and the signed raw
// transactions — so a plaintext endpoint puts all three on the wire in the clear,
// and a typo'd or injected RAIL0_BASE_URL used to do that with no warning at all.
// A local gateway is the one legitimate plaintext case, which is why loopback is
// carved out rather than http:// being banned outright (`rail0 env development`
// sets http://localhost:9292). (#62)
func validateBaseURL(v string) error {
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("base-url must be an http(s) URL, e.g. https://api.rail0.xyz (got %q)", v)
	}
	if u.Scheme == "http" && !isLoopbackHost(u.Hostname()) {
		return fmt.Errorf(
			"base-url %q uses plaintext http:// against a non-loopback host — the SIWE signature, "+
				"the session token and signed transactions would cross the network in the clear. "+
				"Use https://, or a loopback host for local development", v)
	}
	return nil
}

// isLoopbackHost reports whether host names the local machine. Covers "localhost"
// and any *.localhost name (RFC 6761 reserves the whole TLD for loopback), plus
// literal loopback addresses in either family — url.Hostname() has already stripped
// the port and the brackets from an IPv6 literal, so net.ParseIP sees a bare "::1".
func isLoopbackHost(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// lookupKey resolves a key name against the whitelist, erroring with the valid
// set when unknown.
func lookupKey(name string) (configKey, error) {
	if k, ok := configKeys[name]; ok {
		return k, nil
	}
	valid := make([]string, 0, len(configKeys))
	for n := range configKeys {
		valid = append(valid, n)
	}
	sort.Strings(valid)
	return configKey{}, fmt.Errorf("unknown config key %q (valid: %s)", name, strings.Join(valid, ", "))
}

// effectiveSetting resolves a key exactly as the commands do —
// flag > env > config file > built-in default — and reports the origin.
func effectiveSetting(k configKey) (value, origin string) {
	// Only the two keys with a GLOBAL flag can be overridden per command this way.
	// The others (chain-id, token, payer, payee, key) are overridden by per-command
	// flags, which are not bound while `config`/`env` render — so their origin here
	// describes the stored default, which is what those listings are about.
	if k.name == "base-url" && baseURL != "" {
		return baseURL, "flag (--base-url)"
	}
	if k.name == "output" {
		if mode := outputFlagMode(); mode != "" {
			return mode, "flag (--" + mode + ")"
		}
	}
	if v := os.Getenv(k.env); v != "" {
		return v, "env (" + k.env + ")"
	}
	// A LOAD FAILURE IS REPORTED, NOT SWALLOWED. `err == nil` alone meant a
	// malformed config.yaml silently vanished from the resolution: `rail0 config`
	// showed `origin: default` while the file on disk held a different base-url,
	// and every command quietly talked to the wrong gateway. The warning goes to
	// stderr (this returns a value, and callers render it as data) and is printed
	// once per process, since several keys resolve through here. (#108)
	cfg, err := config.Load()
	if err != nil {
		warnConfigUnreadable(err)
	} else if v := cfg.Settings[k.name]; v != "" {
		return v, "config file"
	}
	return k.def(), "default"
}

// warnConfigUnreadable prints the load failure once per process. Deduplicated
// because effectiveSetting runs per key, and one broken file must not produce
// one warning per setting.
var configWarned bool

func warnConfigUnreadable(err error) {
	if configWarned {
		return
	}
	configWarned = true
	fmt.Fprintf(os.Stderr, "warning: ignoring the config file — %v\n", err)
}

// configCmd lists the effective settings when run bare, and holds the
// set/get/unset/path verbs.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage persisted CLI defaults (base-url, chain-id, token, payer, payee, key…)",
	Long: `Manage persisted CLI defaults stored in the config file.

Run bare to list every setting with its effective value and where it comes from.
Resolution precedence is: flag  >  env var  >  config file  >  built-in default.

A stored default fills in the matching flag when you omit it, so the per-payment
constants (chain-id, token, payer, payee, key) can be set once instead of on every
command. 'rail0 env <name>' sets several of them in one shot.

Only non-sensitive values are stored — never private keys or tokens. 'key' holds an
@keychain-name reference ('rail0 keys add'), and a raw key is refused.`,
	Example: `  rail0 config
  rail0 config set base-url http://localhost:9292
  rail0 config set chain-id 5042002
  rail0 config set payee @merchant
  rail0 config set key @payer
  rail0 config get chain-id
  rail0 config unset chain-id`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		names := make([]string, 0, len(configKeys))
		for n := range configKeys {
			names = append(names, n)
		}
		sort.Strings(names)
		rows := make([]map[string]any, 0, len(names))
		for _, n := range names {
			k := configKeys[n]
			value, origin := effectiveSetting(k)
			// `modified` flags settings the user changed away from the built-in
			// default (via flag/env/config file); `default` shows what that
			// built-in value is, so an override is easy to spot at a glance.
			rows = append(rows, map[string]any{
				"key":      n,
				"value":    value,
				"origin":   origin,
				"default":  k.def(),
				"modified": origin != "default",
			})
		}
		return ui.RunAPI(func() (any, error) { return rows, nil })
	},
}

var configSetCmd = &cobra.Command{
	Use:     "set <key> <value>",
	Short:   "Persist a setting to the config file",
	Example: `  rail0 config set base-url http://localhost:9292`,
	Args:    cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		key, value := args[0], args[1]
		k, err := lookupKey(key)
		if err != nil {
			return err
		}
		if k.validate != nil {
			if err := k.validate(value); err != nil {
				return err
			}
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if cfg.Settings == nil {
			cfg.Settings = map[string]string{}
		}
		cfg.Settings[key] = value
		if err := cfg.Save(); err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return map[string]any{"status": "saved", "key": key, "value": value}, nil
		})
	},
}

var configGetCmd = &cobra.Command{
	Use:     "get <key>",
	Short:   "Show a setting's effective value and origin",
	Example: `  rail0 config get base-url`,
	Args:    cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		k, err := lookupKey(args[0])
		if err != nil {
			return err
		}
		value, origin := effectiveSetting(k)
		return ui.RunAPI(func() (any, error) {
			return map[string]any{"key": k.name, "value": value, "origin": origin}, nil
		})
	},
}

var configUnsetCmd = &cobra.Command{
	Use:     "unset <key>",
	Short:   "Remove a setting from the config file",
	Example: `  rail0 config unset base-url`,
	Args:    cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		key := args[0]
		if _, err := lookupKey(key); err != nil {
			return err
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if _, ok := cfg.Settings[key]; !ok {
			return fmt.Errorf("%q is not set in the config file", key)
		}
		delete(cfg.Settings, key)
		if err := cfg.Save(); err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return map[string]any{"status": "removed", "key": key}, nil
		})
	},
}

var configPathCmd = &cobra.Command{
	Use:     "path",
	Short:   "Print the config file path",
	Example: `  rail0 config path`,
	Args:    cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		path, err := config.Path()
		if err != nil {
			return err
		}
		fmt.Println(path)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configSetCmd, configGetCmd, configUnsetCmd, configPathCmd)
}
