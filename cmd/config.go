package cmd

import (
	"fmt"
	"net/url"
	"os"
	"sort"
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
var configKeys = map[string]configKey{
	"base-url": {
		name:     "base-url",
		env:      "RAIL0_BASE_URL",
		desc:     "RAIL0 gateway base URL",
		def:      func() string { return config.DefaultBaseURL },
		validate: validateBaseURL,
	},
}

func validateBaseURL(v string) error {
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("base-url must be an http(s) URL, e.g. https://api.rail0.xyz")
	}
	return nil
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
	// Only base-url has a global flag today; extend here if more keys gain one.
	if k.name == "base-url" && baseURL != "" {
		return baseURL, "flag (--base-url)"
	}
	if v := os.Getenv(k.env); v != "" {
		return v, "env (" + k.env + ")"
	}
	if cfg, err := config.Load(); err == nil {
		if v := cfg.Settings[k.name]; v != "" {
			return v, "config file"
		}
	}
	return k.def(), "default"
}

// configCmd lists the effective settings when run bare, and holds the
// set/get/unset/path verbs.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage persisted CLI defaults (e.g. base-url)",
	Long: `Manage persisted CLI defaults stored in the config file.

Run bare to list every setting with its effective value and where it comes from.
Resolution precedence is: flag  >  env var  >  config file  >  built-in default.

Only non-sensitive values are stored — never private keys or tokens.`,
	Example: `  rail0 config
  rail0 config set base-url http://localhost:9292
  rail0 config get base-url
  rail0 config unset base-url`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		names := make([]string, 0, len(configKeys))
		for n := range configKeys {
			names = append(names, n)
		}
		sort.Strings(names)
		rows := make([]map[string]any, 0, len(names))
		for _, n := range names {
			value, origin := effectiveSetting(configKeys[n])
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
