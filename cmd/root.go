// Package cmd defines the rail0 CLI command tree.
//
// The root command wires global flags (--base-url, --api-key, --debug, --json)
// and delegates to the payments, tokens, and utils sub-trees.
package cmd

import (
	"os"

	"github.com/rail0/cli/internal/output"
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
	// routes every error through output.Fatal for consistent styled output.
	SilenceErrors: true,
	Long: `rail0 is a command-line interface for the RAIL0 API.

Configure the API endpoint with --base-url or RAIL0_BASE_URL.
Set an API key with --api-key or RAIL0_API_KEY.`,
}

// Execute is the entry point called from main. Errors are printed by
// output.Fatal so they receive the same lipgloss styling as other errors.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		output.Fatal(err)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&baseURL, "base-url", "", "RAIL0 API base URL (env: RAIL0_BASE_URL)")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key sent as Bearer token (env: RAIL0_API_KEY)")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Print HTTP request/response details to stderr")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Output raw JSON instead of styled text")

	// OnInitialize runs after flag parsing, so jsonOut already holds the
	// correct value when output.SetJSON is called.
	cobra.OnInitialize(func() { output.SetJSON(jsonOut) })

	rootCmd.AddCommand(paymentsCmd, tokensCmd, utilsCmd)
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
		opts.Logger = rail0.DebugLogger
	}
	return rail0.NewClient(opts)
}
