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
	Use:           "rail0",
	Short:         "CLI for the RAIL0 stablecoin payment protocol",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `rail0 is a command-line interface for the RAIL0 API.

Configure the API endpoint with --base-url or RAIL0_BASE_URL.
Set an API key with --api-key or RAIL0_API_KEY.`,
}

// Execute runs the root command and handles errors with styled output.
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

	cobra.OnInitialize(func() { output.SetJSON(jsonOut) })

	rootCmd.AddCommand(paymentsCmd, tokensCmd, utilsCmd)
}

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
