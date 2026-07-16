package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// version is the CLI version. It defaults to "dev" for a plain `go build` and is
// overridden at build time via -ldflags "-X github.com/rail0/cli/cmd.version=…"
// (see bin/build, which fills it from `git describe`).
var version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the rail0 CLI version",
	Args:  cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("rail0 %s (%s %s/%s)\n", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	// Enables the built-in `--version` flag, and a matching `version` subcommand.
	rootCmd.Version = version
	rootCmd.SetVersionTemplate("rail0 {{.Version}}\n")
	rootCmd.AddCommand(versionCmd)
}
