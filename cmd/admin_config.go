package cmd

import (
	"context"
	"sort"
	"strings"

	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// The configuration the gateway is running with. The gateway answers a table of
// settings by group; this prints it one setting per row, which is how a person reads it
// and how a script filters it.

var adminConfigOverridden bool

var adminConfigCmd = &cobra.Command{
	Use:   "config [group]",
	Short: "The configuration the gateway is running with: every window, limit and feature flag",
	Long: `Print the effective configuration of the gateway process that answers: every window,
threshold, limit and feature flag, one per row.

  group        build, payments, recovery, retention, webhooks, payouts, settlement,
               rate_limit, siwe, session, fees, rpc, onboarding, email
  setting      its name within the group
  value        what the gateway is using now
  env          the environment variable that sets it (empty for a constant of the code)
  overridden   whether that variable is set in this process: a default and an override
               that happen to coincide read differently
  description  what it controls and its unit, in the gateway's own words

Pass a group to print only that one. --overridden keeps only the settings whose variable
is set: what this environment changed from the defaults.

Read-only by nature: the values come from the environment when the process starts, and a
deploy is the only way to change one. No secret is among them.`,
	Example: `  rail0 admin config --pretty
  rail0 admin config payouts               # is the payouts feature on here
  rail0 admin config rate_limit --pretty
  rail0 admin config --overridden          # what this environment changed from the defaults`,
	Args: cobra.MaximumNArgs(1),
	RunE: operatorOnly(func(_ *cobra.Command, args []string) error {
		group := ""
		if len(args) == 1 {
			group = strings.TrimSpace(args[0])
		}
		return ui.RunAPI(func() (any, error) {
			report, err := newReadClient().Admin.Config(context.Background())
			if err != nil {
				return nil, err
			}
			rows, groups := configRows(report)
			if group != "" && !groups[group] {
				names := make([]string, 0, len(groups))
				for g := range groups {
					names = append(names, g)
				}
				sort.Strings(names)
				return nil, usagef("unknown configuration group %q; this gateway has: %s", group, strings.Join(names, ", "))
			}
			kept := rows[:0:0]
			for _, row := range rows {
				if group != "" && row["group"] != group {
					continue
				}
				if adminConfigOverridden && row["overridden"] != true {
					continue
				}
				kept = append(kept, row)
			}
			return table(kept, configColumns), nil
		})
	}),
}

// configRows turns the gateway's report (group → setting → {value, env, overridden,
// description}) into one row per setting, sorted by group and setting, and returns the
// groups it found. Anything in the report that is not a group of settings (generated_at)
// is left out.
func configRows(report map[string]any) ([]map[string]any, map[string]bool) {
	var rows []map[string]any
	groups := map[string]bool{}
	for group, raw := range report {
		settings, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		groups[group] = true
		for name, rawSetting := range settings {
			setting, _ := rawSetting.(map[string]any)
			row := map[string]any{"group": group, "setting": name}
			for _, field := range []string{"value", "env", "overridden", "description"} {
				row[field] = setting[field]
			}
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		gi, gj := rows[i]["group"].(string), rows[j]["group"].(string)
		if gi != gj {
			return gi < gj
		}
		return rows[i]["setting"].(string) < rows[j]["setting"].(string)
	})
	return rows, groups
}

func init() {
	adminConfigCmd.Flags().BoolVar(&adminConfigOverridden, "overridden", false,
		"Only the settings whose environment variable is set in this process")
}
