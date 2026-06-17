// Package cmd defines the rail0 CLI command tree.
package cmd

import (
	"os"

	charmlog "github.com/charmbracelet/log"
	"github.com/rail0/cli/internal/config"
	"github.com/rail0/cli/internal/token"
	"github.com/rail0/cli/internal/ui"
	rail0 "github.com/rail0/go-sdk"
	"github.com/spf13/cobra"
)

var (
	baseURL string
	debug   bool
	jsonOut bool
)

var rootCmd = &cobra.Command{
	Use:           "rail0",
	Short:         "CLI for the RAIL0 stablecoin payment protocol",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `rail0 is a command-line interface for the RAIL0 stablecoin payment API.

Defaults to https://api.rail0.xyz. Override with --base-url or RAIL0_BASE_URL.

Use 'rail0 <command> --help' for detailed usage of any command.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		ui.Fatal(err)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&baseURL, "base-url", "", "RAIL0 API base URL (env: RAIL0_BASE_URL)")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Print HTTP request/response details to stderr")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Output raw JSON instead of styled text")

	cobra.OnInitialize(func() { ui.SetJSON(jsonOut) })

	rootCmd.AddCommand(paymentsCmd, accountsCmd, webhooksCmd, chainsCmd, tokensCmd, authCmd, healthCmd)
}

// newClient builds a RAIL0 API client.
// Priority: --base-url flag > RAIL0_BASE_URL env var > config.DefaultBaseURL.
// If a JWT is stored (RAIL0_TOKEN env var or ~/.cache/rail0/token), it is sent
// as Authorization: Bearer on every request.
func newClient() *rail0.Client {
	url := baseURL
	if url == "" {
		url = os.Getenv("RAIL0_BASE_URL")
	}
	if url == "" {
		url = config.DefaultBaseURL
	}
	opts := rail0.ClientOptions{BaseURL: url}
	if t := token.Load(); t != "" {
		opts.Headers = map[string]string{"Authorization": "Bearer " + t}
	}
	if debug {
		opts.Logger = makeDebugLogger()
	}
	return rail0.NewClient(opts)
}

// makeDebugLogger returns a rail0.Logger that routes HTTP traces through
// charmbracelet/log, producing structured, coloured output on stderr.
func makeDebugLogger() rail0.Logger {
	logger := charmlog.New(os.Stderr)
	logger.SetLevel(charmlog.DebugLevel)
	return func(e rail0.LogEntry) {
		fields := []any{
			"method", e.Method,
			"url", e.URL,
			"status", e.Status,
			"duration_ms", e.DurationMs,
		}
		if e.Attempt > 1 {
			fields = append(fields, "attempt", e.Attempt)
		}
		if e.WillRetry {
			fields = append(fields, "will_retry", true)
		}
		if e.RequestBody != nil {
			fields = append(fields, "request", e.RequestBody)
		}
		if e.ResponseBody != nil {
			fields = append(fields, "response", e.ResponseBody)
		}
		if e.Err != nil {
			logger.Error("HTTP", append(fields, "error", e.Err)...)
		} else {
			logger.Debug("HTTP", fields...)
		}
	}
}
