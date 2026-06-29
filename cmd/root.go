// Package cmd defines the rail0 CLI command tree.
package cmd

import (
	"errors"
	"os"

	charmlog "github.com/charmbracelet/log"
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

	rootCmd.AddCommand(paymentsCmd, walletsCmd, webhooksCmd, chainsCmd, tokensCmd, authCmd, healthCmd, aliasesCmd, configCmd)
}

// newClient builds a RAIL0 API client.
// If a JWT is stored (RAIL0_TOKEN env var or ~/.cache/rail0/token), it is sent
// as Authorization: Bearer on every request.
func newClient() *rail0.Client {
	opts := rail0.ClientOptions{BaseURL: resolveBaseURL()}
	if t := token.Load(); t != "" {
		opts.Headers = map[string]string{"Authorization": "Bearer " + t}
	}
	if debug {
		opts.Logger = makeDebugLogger()
	}
	return rail0.NewClient(opts)
}

// currentAccountID returns the account id of the logged-in session (the JWT
// `account_id` claim). Wallet commands are scoped to the logged-in account, so
// they derive it from the session instead of taking it as an argument; with no
// (or an expired) session this errors, pointing the user at `auth login`.
func currentAccountID() (string, error) {
	t := token.Load()
	if t == "" {
		return "", errors.New("not logged in: run 'rail0 auth login -p 0xYourKey…'")
	}
	claims, err := parseJWTClaims(t)
	if err != nil {
		return "", errors.New("session invalid or expired: run 'rail0 auth login'")
	}
	id, _ := claims["account_id"].(string)
	if id == "" {
		return "", errors.New("session carries no account_id: re-run 'rail0 auth login'")
	}
	return id, nil
}

// resolveBaseURL picks the API base URL. Precedence:
// --base-url flag > RAIL0_BASE_URL env > config file (`config set base-url`) >
// config.DefaultBaseURL. Single source of truth shared with the `config`
// command via effectiveSetting.
func resolveBaseURL() string {
	value, _ := effectiveSetting(configKeys["base-url"])
	return value
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
