package cmd

import (
	"context"

	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:     "health",
	Short:   "Report gateway health (liveness/readiness)",
	Example: `  rail0 health`,
	Args:    cobra.NoArgs,
	// An unhealthy gateway (503, database down) still answers with its report, and the
	// report is the useful part — it says WHAT is down. So it is printed, and the command
	// still fails with the 503 afterwards (exit 8), so a script checking the exit code
	// reads "unhealthy" exactly as before. It used to print only "upstream error (HTTP
	// 503): HTTP 503" and drop the report.
	RunE: func(cmd *cobra.Command, args []string) error {
		var unhealthy error
		if err := ui.RunAPI(func() (any, error) {
			report, err := newReadClient().Health.Get(context.Background())
			if report != nil && err != nil {
				unhealthy = err
				return report, nil
			}
			return report, err
		}); err != nil {
			return err
		}
		return unhealthy
	},
}

func init() {}
