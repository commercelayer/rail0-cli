package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rail0/cli/internal/config"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// environment is a named bundle of CLI settings applied together by
// `env <name>`. Every settings key must be a member of the configKeys
// whitelist, so an environment switch is validated and resolved through exactly
// the same machinery as `config set` — it is just a one-shot set of several
// keys.
type environment struct {
	name     string
	settings map[string]string
}

// environments lists the selectable environments in display order
// (development -> staging -> production). Adding a new per-environment
// parameter means adding its key/value to each entry here (and registering the
// key in configKeys).
var environments = []environment{
	{name: "development", settings: map[string]string{"base-url": "http://localhost:9292"}},
	{name: "staging", settings: map[string]string{"base-url": "https://rail0-gateway-staging.fly.dev"}},
	{name: "production", settings: map[string]string{"base-url": config.DefaultBaseURL}},
}

// lookupEnv resolves an environment by name, erroring with the valid set when
// unknown.
func lookupEnv(name string) (environment, error) {
	for _, e := range environments {
		if e.name == name {
			return e, nil
		}
	}
	names := make([]string, len(environments))
	for i, e := range environments {
		names[i] = e.name
	}
	return environment{}, fmt.Errorf("unknown environment %q (valid: %s)", name, strings.Join(names, ", "))
}

// detectEnv reports which environment the *effective* configuration currently
// matches, or "custom" if it matches none. It compares each environment's
// declared settings against the values the commands would actually use
// (flag > env > config file > built-in default), so it stays truthful even when
// a key was set directly with `config set` rather than via `env`. The
// environments have distinct base-urls, so at most one matches; the first match
// wins.
func detectEnv() string {
	for _, e := range environments {
		match := true
		for key, want := range e.settings {
			k, err := lookupKey(key)
			if err != nil {
				match = false
				break
			}
			if got, _ := effectiveSetting(k); got != want {
				match = false
				break
			}
		}
		if match {
			return e.name
		}
	}
	return "custom"
}

// envParamRows returns one row per environment-managed key with its effective
// value and origin — the detail view shared by the bare `env` listing and
// `env current --details`. Keys are emitted in sorted order for stable output.
func envParamRows() []map[string]any {
	keySet := map[string]struct{}{}
	for _, e := range environments {
		for key := range e.settings {
			keySet[key] = struct{}{}
		}
	}
	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		k, err := lookupKey(key)
		if err != nil {
			continue
		}
		value, origin := effectiveSetting(k)
		rows = append(rows, map[string]any{"key": key, "value": value, "origin": origin})
	}
	return rows
}

var envDetails bool

// envCmd switches the active environment when given a name, and lists all
// environments (marking the current one) when run bare. `current` is a
// subcommand; any other single argument is treated as an environment name to
// switch to.
var envCmd = &cobra.Command{
	Use:   "env [name]",
	Short: "Switch the active environment (development/staging/production) in one shot",
	Long: `Switch a bundle of CLI settings at once by selecting an environment.

Run bare to list the environments and their parameters, with the current one
marked. Pass a name to switch: it writes that environment's settings (today just
base-url) to the config file, so every command uses them.

A stored value is still the lowest-priority source, so --base-url or
RAIL0_BASE_URL override it for a single command.`,
	Example: `  rail0 env                 # list environments, marking the current one
  rail0 env staging         # switch to staging
  rail0 env current         # show the active environment
  rail0 env current -d      # ...with every parameter's effective value`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		if len(args) == 0 {
			return listEnvironments()
		}
		return switchEnv(args[0])
	},
}

// listEnvironments renders every environment with its declared parameters and a
// `current` flag marking the active one.
func listEnvironments() error {
	current := detectEnv()
	rows := make([]map[string]any, 0, len(environments))
	for _, e := range environments {
		rows = append(rows, map[string]any{
			"env":      e.name,
			"settings": e.settings,
			"current":  e.name == current,
		})
	}
	return ui.RunAPI(func() (any, error) { return rows, nil })
}

// switchEnv applies an environment's settings to the config file. Each value is
// validated through the same validator as `config set` before being persisted,
// so a switch can never write a value the commands would reject.
func switchEnv(name string) error {
	e, err := lookupEnv(name)
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Settings == nil {
		cfg.Settings = map[string]string{}
	}
	for key, value := range e.settings {
		k, err := lookupKey(key)
		if err != nil {
			return err
		}
		if k.validate != nil {
			if err := k.validate(value); err != nil {
				return fmt.Errorf("environment %q: %w", name, err)
			}
		}
		cfg.Settings[key] = value
	}
	if err := cfg.Save(); err != nil {
		return err
	}
	return ui.RunAPI(func() (any, error) {
		return map[string]any{"status": "switched", "env": name, "settings": e.settings}, nil
	})
}

var envCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the active environment (--details for every parameter)",
	Example: `  rail0 env current
  rail0 env current --details`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		current := detectEnv()
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
	envCurrentCmd.Flags().BoolVarP(&envDetails, "details", "d", false, "Show every parameter's effective value and origin")
	envCmd.AddCommand(envCurrentCmd)
}
