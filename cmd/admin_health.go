package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

var adminHealthRPC bool

var adminHealthCmd = &cobra.Command{
	Use:   "health",
	Short: "Operator diagnostics: one verdict, then a check per thing that can be wrong",
	Long: `The gateway's operator diagnostics: a verdict ("ok", "degraded" or "error") and a named
check per standing failure condition — the worker fleet, the indexer's intake, stuck
transactions, open webhook circuits, balance divergences — each with its figures and a
sentence saying what they mean.

--rpc asks the other question: can the gateway reach the chains. Every active chain's RPC
endpoints are probed from the gateway's own process, with each one's status, head block
and latency.

The exit code follows the verdict: 0 when it is "ok", 1 otherwise, after the report is
printed. So 'rail0 admin health -q || alert' works as a check.`,
	Example: `  rail0 admin health
  rail0 admin health --rpc
  rail0 admin health --pretty`,
	Args: cobra.NoArgs,
	RunE: operatorOnly(func(_ *cobra.Command, _ []string) error {
		// The report is printed whatever it says, and a verdict that is not "ok" fails
		// the command AFTERWARDS: the figures are the useful part of a bad answer.
		var verdict error
		err := ui.RunAPI(func() (any, error) {
			ctx := context.Background()
			client := newReadClient()
			if adminHealthRPC {
				report, err := client.Admin.RPCHealth(ctx)
				if err != nil {
					return nil, err
				}
				var failing []string
				for _, c := range report.Chains {
					if c.Status != "ok" {
						failing = append(failing, fmt.Sprintf("%s (%s)", firstNonEmptyStr(c.Name, intOrEmpty(c.ChainID)), c.Status))
					}
				}
				verdict = healthVerdict("RPC health", report.Status, failing)
				return report, nil
			}
			report, err := client.Admin.Health(ctx)
			if err != nil {
				return nil, err
			}
			verdict = healthVerdict("gateway health", report.Status, failingChecks(report.Checks))
			return report, nil
		})
		if err != nil {
			return err
		}
		return verdict
	}),
}

// healthVerdict turns a report's status into the command's outcome: nil for "ok", and
// otherwise an error naming what is not ok (exit 1: nothing about the call is wrong).
func healthVerdict(what, status string, failing []string) error {
	if status == "ok" {
		return nil
	}
	if len(failing) == 0 {
		return fmt.Errorf("%s is %q", what, status)
	}
	return fmt.Errorf("%s is %q: %s", what, status, strings.Join(failing, ", "))
}

// failingChecks names the checks of GET /admin/health whose own status is not "ok", as
// "name (status)", sorted. The checks are read generically: the gateway adds new ones
// over time, and the verdict must count them without a release of the CLI.
func failingChecks(checks any) []string {
	m, _ := checks.(map[string]any)
	var out []string
	for name, v := range m {
		check, _ := v.(map[string]any)
		if status, _ := check["status"].(string); status != "" && status != "ok" {
			out = append(out, fmt.Sprintf("%s (%s)", name, status))
		}
	}
	sort.Strings(out)
	return out
}

func init() {
	adminHealthCmd.Flags().BoolVar(&adminHealthRPC, "rpc", false,
		"Probe every active chain's RPC endpoints from the gateway instead of reading the diagnostics")
}
