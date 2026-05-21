// Package cmd defines the rail0 CLI command tree.
//
// The root command wires global flags (--base-url, --api-key, --debug, --json)
// and delegates to the payments and merchants sub-trees.
package cmd

import (
	"os"

	charmlog "github.com/charmbracelet/log"
	"github.com/rail0/cli/internal/ui"
	rail0 "github.com/rail0/go-sdk"
	"github.com/spf13/cobra"
)

var (
	baseURL string
	apiKey  string
	debug   bool
	jsonOut bool
)

var rootCmd = &cobra.Command{
	Use:          "rail0",
	Short:        "CLI for the RAIL0 stablecoin payment protocol",
	SilenceUsage: true,
	// SilenceErrors prevents cobra from printing the error itself; Execute()
	// routes every error through ui.Fatal for consistent styled output.
	SilenceErrors: true,
	Long: `rail0 is a command-line interface for the RAIL0 stablecoin payment API.

Configure the API endpoint and credentials via flags or environment variables:

  --base-url / RAIL0_BASE_URL   RAIL0 API base URL
  --api-key  / RAIL0_API_KEY    Bearer token for authenticated requests

Use 'rail0 <command> --help' for detailed usage of any command.`,
}

// Execute is the entry point called from main. Errors are printed by
// ui.Fatal so they receive the same lipgloss styling as other errors.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		ui.Fatal(err)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&baseURL, "base-url", "", "RAIL0 API base URL (env: RAIL0_BASE_URL)")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key sent as Bearer token (env: RAIL0_API_KEY)")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Print HTTP request/response details to stderr")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Output raw JSON instead of styled text")

	// OnInitialize runs after flag parsing, so jsonOut already holds the
	// correct value when ui.SetJSON is called.
	cobra.OnInitialize(func() { ui.SetJSON(jsonOut) })

	rootCmd.AddCommand(paymentsCmd, merchantsCmd)
}

// newClient builds a RAIL0 API client from the resolved configuration.
// Flag values take priority over environment variables.
func newClient() *rail0.Client {
	url := baseURL
	if url == "" {
		url = os.Getenv("RAIL0_BASE_URL")
	}
	key := apiKey
	if key == "" {
		key = os.Getenv("RAIL0_API_KEY")
	}
	opts := rail0.ClientOptions{BaseURL: url}
	if key != "" {
		opts.Headers = map[string]string{"Authorization": "Bearer " + key}
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
