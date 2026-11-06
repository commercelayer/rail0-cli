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
// that overrides the stored value, a description, what the CLI does when the key
// is not set, and an optional validator for `config set`.
//
// `whenUnset` is NOT a value of the key. A key is either set (somewhere: config
// file, env var, flag) and then has a value, or it is not set at all. What the CLI
// falls back to in that case is the CLI's behaviour, documented in the help and
// used by the resolution, never listed as if the key held it — `rail0 config` used
// to show every key with a `default` column, including an empty one for chain-id
// and token, which read as keys that exist without a value.
type configKey struct {
	name      string
	env       string
	desc      string
	whenUnset func() string
	validate  func(string) error
}

// configKeys is the whitelist of settable keys. Only non-sensitive endpoint /
// default values belong here — never private keys or tokens (CLAUDE.md rule 8).
//
// base-url used to be the only entry, which made `env <name>` a switch of one
// setting: every `payments create` still needed -c and -t spelled out again, even
// though the chain and the token are per-environment constants in practice. (#106)
//
// Payer, payee and the signing key are deliberately NOT here. Each already has its
// own mechanism for not retyping a value — the alias book for addresses ('rail0
// aliases add'), the OS keychain for keys ('rail0 keys add', used as @name) — and a
// second home for them in the config file would both duplicate that and make the
// two parties and the signer of a fund-moving command implicit, resolved from a file
// instead of from the command line.
var configKeys = map[string]configKey{
	"base-url": {
		name:      "base-url",
		env:       "RAIL0_BASE_URL",
		desc:      "RAIL0 gateway base URL",
		whenUnset: func() string { return config.DefaultBaseURL },
		validate:  validateBaseURL,
	},
	"chain": {
		name:      "chain",
		env:       "RAIL0_CHAIN",
		desc:      "Default chain for --chain/-c: a slug (arc) or a chain id",
		whenUnset: func() string { return "" },
		validate:  validateChainRef,
	},
	// The numeric predecessor of `chain`, still read (configuredChain) so configs and
	// RAIL0_CHAIN_ID exports written before slugs keep working. `chain` wins when both
	// are set in the same place.
	"chain-id": {
		name:      "chain-id",
		env:       "RAIL0_CHAIN_ID",
		desc:      "Legacy numeric default chain; superseded by chain",
		whenUnset: func() string { return "" },
		validate:  validatePositiveInt("chain-id"),
	},
	"token": {
		name: "token",
		// NOT RAIL0_TOKEN: that name was a JWT source until #60, and reusing it
		// would make a stale exported session token arrive here as a payment token
		// symbol — a confusing failure in the one place a leftover RAIL0_TOKEN is
		// still likely to exist in someone's shell.
		env:       "RAIL0_TOKEN_SYMBOL",
		desc:      "Default token symbol for --token/-t (e.g. USDC)",
		whenUnset: func() string { return "" },
		validate:  validateTokenSymbol,
	},
	"admin-url": {
		name:      "admin-url",
		env:       "RAIL0_ADMIN_URL",
		desc:      "Admin app URL that 'rail0 open' opens",
		whenUnset: func() string { return "" },
		validate:  validateAdminURL,
	},
	"output": {
		name:      "output",
		env:       "RAIL0_OUTPUT",
		desc:      "Default output mode: auto, json, pretty or quiet",
		whenUnset: func() string { return "auto" },
		validate:  validateOutputMode,
	},
	"http-timeout": {
		name:      "http-timeout",
		env:       "RAIL0_HTTP_TIMEOUT",
		desc:      "Per-request HTTP timeout in seconds",
		whenUnset: func() string { return "30" },
		validate:  validatePositiveInt("http-timeout"),
	},
}

// configuredDefault returns a key's value when it is set, or "" when it is not.
// Consumers use it to fill a flag the user did not pass, so "" has to mean "nothing
// configured" — returning the unset fallback would make `output`'s "auto" or
// `http-timeout`'s "30" look like a user choice and override the caller's own.
func configuredDefault(name string) string {
	k, err := lookupKey(name)
	if err != nil {
		return ""
	}
	value, _, ok := configuredSetting(k)
	if !ok {
		return ""
	}
	return value
}

// unsetBehaviour says, in words, what the CLI does when the key is not set: the
// fallback value it uses, or that the matching flag must be passed.
func unsetBehaviour(k configKey) string {
	if v := k.whenUnset(); v != "" {
		return "the CLI uses " + v
	}
	return "the matching flag must be passed"
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

// validateChainRef checks the shape of a default chain: a positive chain id or a
// slug. Syntactic only — whether the slug exists depends on the gateway, which may not
// be configured yet (and differs per environment); the command using it resolves it.
func validateChainRef(v string) error {
	if n, err := strconv.Atoi(v); err == nil {
		if n <= 0 {
			return fmt.Errorf("chain must be a slug (e.g. arc) or a positive chain id (got %q)", v)
		}
		return nil
	}
	if !chainSlugPattern(v) {
		return fmt.Errorf("chain must be a slug (e.g. arc) or a positive chain id (got %q)", v)
	}
	return nil
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

// originUnset is the origin effectiveSetting reports when the key is set nowhere and
// the value is the CLI's own fallback.
const originUnset = "not set"

// effectiveSetting resolves a key exactly as the commands do —
// flag > env > config file > the CLI's fallback when unset — and reports the origin.
// It is what the commands USE; what is SET is configuredSetting.
func effectiveSetting(k configKey) (value, origin string) {
	if v, o, ok := configuredSetting(k); ok {
		return v, o
	}
	return k.whenUnset(), originUnset
}

// configuredSetting resolves a key from the places it can be SET — flag > env >
// config file — and reports whether it is set at all. No fallback: an unset key has
// no value, which is what `rail0 config` lists and `config get` answers.
func configuredSetting(k configKey) (value, origin string, ok bool) {
	// Only the two keys with a GLOBAL flag can be overridden per command this way.
	// chain and token are overridden by per-command flags, which are not bound
	// while `config`/`env` render — so their origin here describes the stored
	// default, which is what those listings are about.
	if k.name == "base-url" && baseURL != "" {
		return baseURL, "flag (--base-url)", true
	}
	if k.name == "output" {
		if mode := outputFlagMode(); mode != "" {
			return mode, "flag (--" + mode + ")", true
		}
	}
	if v := os.Getenv(k.env); v != "" {
		return v, "env (" + k.env + ")", true
	}
	// A LOAD FAILURE IS REPORTED, NOT SWALLOWED. `err == nil` alone meant a
	// malformed config.yaml silently vanished from the resolution: `rail0 config`
	// showed the key as unset while the file on disk held a different base-url,
	// and every command quietly talked to the wrong gateway. The warning goes to
	// stderr (this returns a value, and callers render it as data) and is printed
	// once per process, since several keys resolve through here. (#108)
	cfg, err := config.Load()
	if err != nil {
		warnConfigUnreadable(err)
	} else if v := cfg.Settings[k.name]; v != "" {
		return v, "config file", true
	}
	return "", "", false
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
	ui.Notice("warning", map[string]any{"what": "config_unreadable"}, fmt.Sprintf("warning: ignoring the config file — %v", err))
}

// loadConfigForWrite is config.LoadForWrite plus the one thing the package
// cannot do: tell the user, on stderr, that an unreadable config is being
// dropped. Silence here would turn a corrupt file into a silently empty one,
// which is the failure mode that makes "why did my aliases vanish?" unanswerable.
func loadConfigForWrite() (*config.Config, error) {
	cfg, replaced, err := config.LoadForWrite()
	if err != nil {
		return nil, err
	}
	if replaced {
		ui.Notice("warning", map[string]any{"what": "config_replaced"},
			"warning: the config file could not be parsed, so it is being replaced —\n"+
				"         any settings, aliases or key names it held are gone")
	}
	return cfg, nil
}

// configCmd lists the effective settings when run bare, and holds the
// set/get/unset/path verbs.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage persisted CLI defaults (base-url, chain, token, output, http-timeout)",
	Long: `Manage persisted CLI defaults stored in the config file.

Run bare to list the settings that are SET, with each value and where it comes from
(flag, env var or config file). A key that is set nowhere is not listed: it has no
value, and the CLI does what the key reference below says for an unset key.
Resolution precedence is: flag  >  env var  >  config file.

A stored default fills in the matching flag when you omit it, so the chain and the
token can be set once instead of on every command. 'rail0 env <name>' sets both in
one shot along with the gateway.

Addresses and signing keys are NOT settable here: use the alias book ('rail0 aliases
add') and the OS keychain ('rail0 keys add', passed as @name), which already spare
you the retyping without making the parties or the signer implicit.`,
	Example: `  rail0 config
  rail0 config set base-url http://localhost:9292
  rail0 config set chain arc
  rail0 config set token USDC
  rail0 config get chain
  rail0 config unset chain`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		names := sortedConfigKeyNames()
		rows := make([]map[string]any, 0, len(names))
		for _, n := range names {
			// Only keys that are set: a row is a key AND its value. What the CLI does
			// for an unset key is in the help (`rail0 config --help`), not a column.
			value, origin, ok := configuredSetting(configKeys[n])
			if !ok {
				continue
			}
			rows = append(rows, map[string]any{"key": n, "value": value, "origin": origin})
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
		cfg, err := loadConfigForWrite()
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
	Short:   "Show a setting's value and origin; an unset key is an error",
	Example: `  rail0 config get base-url`,
	Args:    cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		k, err := lookupKey(args[0])
		if err != nil {
			return err
		}
		value, origin, ok := configuredSetting(k)
		if !ok {
			return fmt.Errorf("%s is not set — when it is unset, %s", k.name, unsetBehaviour(k))
		}
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
		cfg, err := loadConfigForWrite()
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
		return printLine(path, map[string]any{"path": path})
	},
}

// sortedConfigKeyNames returns the settable keys in a stable order.
func sortedConfigKeyNames() []string {
	names := make([]string, 0, len(configKeys))
	for n := range configKeys {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// configKeyReference is the help's table of settable keys: what each one sets, its
// env var, and what the CLI does when it is not set. Built from configKeys so the
// help cannot drift from the code.
func configKeyReference() string {
	var b strings.Builder
	b.WriteString("\n\nSettable keys (env var) — and what the CLI does when the key is not set:\n")
	for _, n := range sortedConfigKeyNames() {
		k := configKeys[n]
		fmt.Fprintf(&b, "  %-13s %s (%s) — not set: %s\n", n, k.desc, k.env, unsetBehaviour(k))
	}
	return strings.TrimRight(b.String(), "\n")
}

func init() {
	configCmd.Long += configKeyReference()
	configCmd.AddCommand(configSetCmd, configGetCmd, configUnsetCmd, configPathCmd)
}
