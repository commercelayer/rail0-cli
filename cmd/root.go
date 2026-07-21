// Package cmd defines the rail0 CLI command tree.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	charmlog "github.com/charmbracelet/log"
	"github.com/rail0/cli/internal/token"
	"github.com/rail0/cli/internal/ui"
	rail0 "github.com/rail0/go-sdk"
	"github.com/spf13/cobra"
)

var (
	baseURL   string
	debug     bool
	jsonOut   bool
	prettyOut bool
	quietOut  bool
)

var rootCmd = &cobra.Command{
	Use:           "rail0",
	Short:         "CLI for the RAIL0 stablecoin payment protocol",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `rail0 is a command-line interface for the RAIL0 stablecoin payment gateway.

Defaults to https://api.rail0.xyz. Override with --base-url or RAIL0_BASE_URL.

Use 'rail0 <command> --help' for detailed usage of any command.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		ui.FatalCode(humanizeError(err), exitCode(err))
	}
}

// Root exposes the assembled command tree so out-of-band tools (bin/docs, which
// generates the CLI reference) can walk it — keeping the doc-generation
// dependencies out of the shipped binary, which never imports them.
func Root() *cobra.Command { return rootCmd }

func init() {
	rootCmd.PersistentFlags().StringVar(&baseURL, "base-url", "", "RAIL0 gateway base URL (env: RAIL0_BASE_URL)")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Print HTTP request/response details to stderr")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Force plain (uncolored) JSON output (useful in scripts)")
	rootCmd.PersistentFlags().BoolVar(&prettyOut, "pretty", false, "Use the key-value/table view instead of JSON (terminal only)")
	rootCmd.PersistentFlags().BoolVarP(&quietOut, "quiet", "q", false, "Print only the result's id/hash, one per line (useful in scripts)")

	cobra.OnInitialize(func() {
		ui.SetJSON(jsonOut)
		ui.SetPretty(prettyOut)
		ui.SetQuiet(quietOut)
	})

	rootCmd.AddCommand(paymentsCmd, walletsCmd, paymentMethodsCmd, webhooksCmd, chainsCmd, tokensCmd, authCmd, healthCmd, aliasesCmd, keysCmd, configCmd, envCmd)
}

// newClient builds a RAIL0 gateway client.
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

// ensureSession guarantees a valid SIWE session whose address matches `addr`
// before a write to /payments. The gateway now gates every /payments endpoint on
// a session, and `create` additionally requires the payer to BE the authenticated
// address — so this signs in with the caller's key (the same key the command
// signs the payload with, hence the session address == addr) when the stored
// token is missing, expired, or for a different address. A no-op when already
// logged in as `addr`. The token is saved so follow-up reads (e.g. payments get)
// reuse the session.
func ensureSession(ctx context.Context, keyHex, addr string) error {
	if t := token.Load(); t != "" {
		if claims, err := parseJWTClaims(t); err == nil {
			sub, _ := claims["sub"].(string)
			exp, _ := claims["exp"].(float64)
			if strings.EqualFold(sub, addr) && int64(exp) > time.Now().Unix() {
				return nil
			}
		}
	}
	privKey, err := rail0.HexToPrivateKey(keyHex)
	if err != nil {
		return fmt.Errorf("invalid signing key: %w", err)
	}
	authResp, err := rail0.NewClient(rail0.ClientOptions{BaseURL: resolveBaseURL()}).
		Auth.Login(ctx, privKey, siweHost(resolveBaseURL()))
	if err != nil {
		return fmt.Errorf("sign-in failed (required to create a payment): %w", err)
	}
	if err := token.Save(authResp.Token); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Signed in as %s\n", addr)
	return nil
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

// resolveAccountID returns the explicit --account value when given, otherwise
// the logged-in session's account id. One of the two MUST be present: with no
// flag and no session it errors, so a wallets query always has an account to
// scope to.
func resolveAccountID(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	id, err := currentAccountID()
	if err != nil {
		return "", fmt.Errorf("%w (or pass --account <id>)", err)
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
