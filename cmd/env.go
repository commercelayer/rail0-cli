package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rail0/cli/internal/config"
	"github.com/rail0/cli/internal/token"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// environment is a named gateway plus the built-in defaults of its per-environment
// settings (the configKeys marked perEnv). The user's own values for an environment
// live in the config file (environments.<name>.<key>, written by `config set` while
// that environment is active) and apply over these defaults.
type environment struct {
	name     string
	defaults map[string]string
}

// environments lists the selectable environments in display order
// (development -> staging -> production). Adding a new per-environment parameter
// means registering its key in configKeys with perEnv and, where one makes sense,
// giving each entry here a default for it.
//
// Every environment defaults chain to the slug arc and token to USDC, so a
// `payments create` needs neither -c nor -t anywhere. The slug is resolved by the
// environment's gateway, which serves one network class: Arc Testnet on development
// and staging, Arc (mainnet, the only production chain) on production. production
// has no admin-url default: there is no admin address to declare yet.
var environments = []environment{
	{name: "development", defaults: map[string]string{
		"base-url":  "http://localhost:9292",
		"admin-url": "http://localhost:3000",
		"chain":     "arc",
		"token":     "USDC",
	}},
	{name: "staging", defaults: map[string]string{
		"base-url":  "https://rail0-gateway-staging.fly.dev",
		"admin-url": "https://rail0-admin-staging.fly.dev",
		"chain":     "arc",
		"token":     "USDC",
	}},
	{name: "production", defaults: map[string]string{
		"base-url": config.DefaultBaseURL,
		"chain":    "arc",
		"token":    "USDC",
	}},
}

// defaultEnvName is the environment in effect when the config file names none: the
// published CLI talks to production, and only someone who knows the hidden `env`
// command ever moves it elsewhere.
const defaultEnvName = "production"

// envIdentityKey is the setting that DEFINES an environment: its gateway. It is the
// one a switch is checked against (refuseOverriddenSwitch), since a switch the
// commands would not follow is no switch.
const envIdentityKey = "base-url"

// lookupEnv resolves an environment by name, erroring with the valid set when
// unknown.
func lookupEnv(name string) (environment, error) {
	for _, e := range environments {
		if e.name == name {
			return e, nil
		}
	}
	return environment{}, fmt.Errorf("unknown environment %q (valid: %s)", name, strings.Join(envNames(), ", "))
}

// envNames lists the selectable environment names in display order.
func envNames() []string {
	names := make([]string, len(environments))
	for i, e := range environments {
		names[i] = e.name
	}
	return names
}

// envDefault returns environment env's built-in default for key, if it has one.
func envDefault(env, key string) (string, bool) {
	e, err := lookupEnv(env)
	if err != nil {
		return "", false
	}
	v, ok := e.defaults[key]
	return v, ok && v != ""
}

// activeEnv names the environment every command resolves its per-environment
// settings against: the one the config file records, production when it records
// none (or cannot be read — the warning is configuredSetting's).
func activeEnv() string {
	cfg, err := config.Load()
	if err != nil {
		return defaultEnvName
	}
	return activeEnvOf(cfg)
}

// activeEnvOf is activeEnv for an already-loaded config. A name the CLI does not
// know (a hand-edited file) falls back to production, with a warning once per
// process: silently honouring it would resolve every per-environment key to
// nothing, and silently ignoring it would hide the typo.
func activeEnvOf(cfg *config.Config) string {
	if cfg.Environment == "" {
		return defaultEnvName
	}
	if _, err := lookupEnv(cfg.Environment); err != nil {
		warnUnknownEnv(cfg.Environment)
		return defaultEnvName
	}
	return cfg.Environment
}

var unknownEnvWarned bool

func warnUnknownEnv(name string) {
	if unknownEnvWarned {
		return
	}
	unknownEnvWarned = true
	ui.Notice("warning", map[string]any{"what": "unknown_environment", "environment": name},
		fmt.Sprintf("warning: the config file names an unknown environment %q — using %s", name, defaultEnvName))
}

// perEnvKeys lists the per-environment setting names, sorted, so every listing and
// every message orders them the same way.
func perEnvKeys() []string {
	var keys []string
	for name, k := range configKeys {
		if k.perEnv {
			keys = append(keys, name)
		}
	}
	sort.Strings(keys)
	return keys
}

// envEffective returns environment e's per-environment settings as they would
// resolve from the config file alone — the user's stored value where there is one,
// else the built-in default — for every key that has either. Flags and env vars are
// NOT applied: they override the active environment for one command, and this
// describes what each environment IS. That is what `env -d` lists.
func envEffective(cfg *config.Config, e environment) map[string]string {
	out := map[string]string{}
	for _, key := range perEnvKeys() {
		if v, ok := cfg.EnvSetting(e.name, key); ok {
			out[key] = v
		} else if v, ok := e.defaults[key]; ok && v != "" {
			out[key] = v
		}
	}
	return out
}

// envParamRows returns one row per per-environment key that is in effect for the
// ACTIVE environment, with its value and origin — the detail view shared by `env
// <name> -d` and `env current -d`. Flags and env vars count here, as in `rail0
// config`: this is what the commands will use. Keys come sorted for stable output.
func envParamRows() []map[string]any {
	keys := perEnvKeys()
	rows := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		value, origin, ok := configuredSetting(configKeys[key])
		if !ok {
			continue
		}
		rows = append(rows, map[string]any{"key": key, "value": value, "origin": origin})
	}
	return rows
}

var envDetails bool

// envCmd switches the active environment when given a name, and lists all
// environments (marking the current one) when run bare. `current` is a
// subcommand; any other single argument is treated as an environment name to
// switch to.
//
// HIDDEN: it is left out of `rail0 --help`, of completion and of the generated
// reference, and `rail0 env --help` still answers. The published CLI is for
// production; development and staging are for the people who run them, who know
// the command. A switch is remembered in the config file, so every later command
// uses it until the next switch.
var envCmd = &cobra.Command{
	Use:    "env [name]",
	Hidden: true,
	Short:  "Switch the active environment (development/staging/production); remembered until the next switch",
	Long: `Select the environment every command works against: its gateway, admin app,
default chain and default token.

Run bare to list the selectable environments, with the active one marked; add
--details to also print each one's parameters. Pass a name to switch: the choice
is written to the config file and every later command uses that environment until
you switch again. The default, with nothing written, is production.

Each environment has built-in defaults (base-url, admin-url, chain, token) and
keeps its own stored values: 'rail0 config set chain …' while an environment is
active changes that environment only, and 'rail0 config' / 'rail0 config get'
read the active one. 'env <name> --details' prints what the switch ended up with.

Sessions are per gateway, so a switch does not log you out of anything: the
session of the environment you leave stays cached and is there again when you
come back, and the result says whether the environment you arrive on has one.

A stored value is still the lowest-priority source, so --base-url or
RAIL0_BASE_URL override it for a single command — and while either one points at
a different gateway, a switch is refused: it could not take effect.`,
	Example: `  rail0 env                 # list the environments, marking the active one
  rail0 env -d              # ...with each one's parameters
  rail0 env staging         # switch to staging
  rail0 env staging -d      # ...and also print the resulting parameters
  rail0 env current         # show the active environment
  rail0 env current -d      # ...with every parameter's effective value`,
	Args: cobra.MaximumNArgs(1),
	// The environment names are a fixed, short set, so `rail0 env st<TAB>` completes
	// rather than sending the reader to --help. (#110)
	ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return append(envNames(), "current"), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(_ *cobra.Command, args []string) error {
		if len(args) == 0 {
			return listEnvironments(envDetails)
		}
		return switchEnv(args[0], envDetails)
	},
}

// listEnvironments names the selectable environments and marks the active one,
// adding each one's parameters only under --details.
func listEnvironments(details bool) error {
	rows, err := environmentRows(details)
	if err != nil {
		return err
	}
	return ui.RunAPI(func() (any, error) { return rows, nil })
}

// environmentRows builds the listing. Pure and separate from the rendering for
// the same reason switchResult and envParamRows are: the -d gating is what the
// test pins, and it should not have to capture stdout to do it.
//
// Bare answers "which environments can I pick?"; -d adds what each one is made of
// (stored values over built-in defaults). A persistent flag must mean the same
// thing on every form of `env`, so the bare listing never dumps the parameters.
func environmentRows(details bool) ([]map[string]any, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	current := activeEnvOf(cfg)
	rows := make([]map[string]any, 0, len(environments))
	for _, e := range environments {
		row := map[string]any{
			"env":     e.name,
			"current": e.name == current,
		}
		if details {
			row["settings"] = envEffective(cfg, e)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// switchEnv records name as the active environment. Nothing else moves: each
// environment keeps its own stored settings and its own cached session, so a
// switch is a one-line write and switching back restores exactly what was there.
// The confirmation is the status and env name; the resulting parameters are listed
// only when details is set (the -d flag), matching `env current`.
func switchEnv(name string, details bool) error {
	e, err := lookupEnv(name)
	if err != nil {
		return err
	}
	cfg, err := loadConfigForWrite()
	if err != nil {
		return err
	}
	if err := refuseOverriddenSwitch(cfg, e); err != nil {
		return err
	}
	cfg.Environment = name
	if err := cfg.Save(); err != nil {
		return err
	}
	// AFTER the save, so resolveBaseURL() is the environment we just moved to.
	// Sessions are per gateway, so arriving somewhere unauthenticated is a normal
	// outcome of a switch — and it used to be completely silent: the next command
	// failed with a 401 that looked like the switch had broken something.
	loggedIn := hasLiveSession()
	if !loggedIn {
		ui.Notice("not_logged_in", nil, "No session for this gateway. "+loginHint)
	}
	return ui.RunAPI(func() (any, error) { return switchResult(name, details, loggedIn), nil })
}

// refuseOverriddenSwitch stops a switch that could not take effect. `env` writes the
// config FILE, but --base-url and RAIL0_BASE_URL outrank the file, so with either set
// the switch would change nothing the commands use. Refused before any side effect,
// naming the override to remove. An override that already points at the target's
// gateway (its stored base-url, else its default) is harmless.
func refuseOverriddenSwitch(cfg *config.Config, e environment) error {
	k := configKeys[envIdentityKey]
	value, origin, ok := configuredSetting(k)
	if !ok || originRank(origin) < originRank("env") {
		return nil
	}
	target := envEffective(cfg, e)[envIdentityKey]
	if token.NormalizeBaseURL(value) == token.NormalizeBaseURL(target) {
		return nil
	}
	return fmt.Errorf("cannot switch to %q: %s is set by %s, which overrides the config file — "+
		"the switch would change nothing the commands use; remove the override and retry",
		e.name, envIdentityKey, origin)
}

// hasLiveSession reports whether the CURRENT gateway has a usable session: a
// stored token that still parses (parseJWTClaims rejects an expired one), which is
// the same test every authenticated command applies.
func hasLiveSession() bool {
	stored := token.Load(resolveBaseURL())
	if stored == "" {
		return false
	}
	_, err := parseJWTClaims(stored)
	return err == nil
}

// switchResult builds the confirmation payload for a switch: status, env name and
// whether the new gateway has a live session, plus each parameter's effective value
// and origin only when details is set (the -d flag). Split out from switchEnv so the
// gating is unit-testable without persisting or capturing stdout.
func switchResult(name string, details bool, loggedIn bool) map[string]any {
	out := map[string]any{"status": "switched", "env": name, "logged_in": loggedIn}
	if details {
		out["settings"] = envParamRows()
	}
	return out
}

var envCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the active environment (--details for every parameter)",
	Example: `  rail0 env current
  rail0 env current --details`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		current := activeEnv()
		return ui.RunAPI(func() (any, error) {
			out := map[string]any{"env": current}
			if envDetails {
				out["settings"] = envParamRows()
			}
			return out, nil
		})
	},
}

func init() {
	// --details/-d is persistent on `env`, so both `env <name>` (switch) and
	// `env current` accept it: without it they print just status/name; with it
	// they also list each parameter's effective value and origin.
	envCmd.PersistentFlags().BoolVarP(&envDetails, "details", "d", false, "Also print each parameter's effective value and origin")
	envCmd.AddCommand(envCurrentCmd)
}
