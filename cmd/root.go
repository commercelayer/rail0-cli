// Package cmd defines the rail0 CLI command tree.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	charmlog "github.com/charmbracelet/log"
	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/selfupdate"
	"github.com/rail0/cli/internal/token"
	"github.com/rail0/cli/internal/ui"
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
	// Runs before every subcommand's RunE (no subcommand defines its own
	// PersistentPreRunE, which would shadow this one). See checkBaseURL (#62) and
	// maybeOfferUpdate, the once-a-day release check for interactive sessions.
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		maybeOfferUpdate(cmd)
		return checkBaseURL(cmd)
	},
	Long: `rail0 is a command-line interface for the RAIL0 stablecoin payment gateway.

QUICKSTART
  rail0 init                       guided setup: gateway, key, sign-in
  rail0 payment-methods -A 0x…     what a merchant accepts (public, no login)
  rail0 payments create …          create + sign a payment as the payer
  rail0 payments capture <id> -a … capture it as the payee

Defaults to https://api.rail0.xyz. Override with --base-url or RAIL0_BASE_URL.

Note the singular/plural pair: 'rail0 token' prints your SESSION JWT, while
'rail0 tokens' lists the gateway's accepted PAYMENT tokens.

Use 'rail0 <command> --help' for detailed usage of any command.`,
}

func Execute() {
	selfupdate.CleanupOld()
	rejectUnknownSubcommands(rootCmd)
	if err := rootCmd.Execute(); err != nil {
		ui.FatalCode(humanizeError(err), exitCode(err))
	}
}

// rejectUnknownSubcommands makes every pure group command (`auth`, `webhooks`, `keys`,
// …: subcommands, no action of its own) fail on an argument it does not recognise.
//
// cobra reports an unknown command only at the ROOT. Below it, a group with no Run
// just prints its help and exits 0, so `rail0 webhooks lsit` or `rail0 auth logn`
// "succeeded" — and a script with a typo carried on as if the step had run. The bare
// group still prints its help and exits 0; only a stray argument is an error, with
// cobra's own did-you-mean suggestions.
//
// Applied at Execute rather than in each file's init so a group added later is
// covered without remembering to, and so the generated reference (bin/docs, which
// walks the tree without executing it) keeps describing the groups as groups.
func rejectUnknownSubcommands(cmd *cobra.Command) {
	for _, sub := range cmd.Commands() {
		rejectUnknownSubcommands(sub)
	}
	if cmd == cmd.Root() || !cmd.HasSubCommands() || cmd.Run != nil || cmd.RunE != nil {
		return
	}
	// cobra defaults the distance only for the root's own lookup; a group left at 0
	// would suggest nothing but exact matches.
	if cmd.SuggestionsMinimumDistance <= 0 {
		cmd.SuggestionsMinimumDistance = 2
	}
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if len(args) == 0 {
			return c.Help()
		}
		msg := fmt.Sprintf("unknown command %q for %q", args[0], c.CommandPath())
		if suggestions := c.SuggestionsFor(args[0]); len(suggestions) > 0 {
			msg += "\n\nDid you mean this?\n\t" + strings.Join(suggestions, "\n\t")
		}
		return fmt.Errorf("%s\n\nRun '%s --help' for usage", msg, c.CommandPath())
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

	cobra.OnInitialize(applyOutputMode)

	// Grouped help. Sixteen top-level commands listed alphabetically gave
	// `completion` the same weight as `payments`, and put the confusable `token`
	// (session JWT) next to `tokens` (payment-token catalog) with nothing to
	// distinguish them — the difference was explained only inside cmd/token.go. (#105)
	rootCmd.AddGroup(
		&cobra.Group{ID: groupPayments, Title: "Payments:"},
		&cobra.Group{ID: groupAccount, Title: "Account & merchant setup:"},
		&cobra.Group{ID: groupCatalog, Title: "Network catalog:"},
		&cobra.Group{ID: groupLocal, Title: "Local configuration:"},
	)
	assignGroup(groupPayments, paymentsCmd)
	assignGroup(groupAccount, authCmd, tokenCmd, walletsCmd, paymentMethodsCmd, webhooksCmd, analyticsCmd, disputesCmd)
	assignGroup(groupCatalog, chainsCmd, tokensCmd, healthCmd)
	assignGroup(groupLocal, initCmd, configCmd, envCmd, aliasesCmd, keysCmd)

	rootCmd.AddCommand(paymentsCmd, walletsCmd, paymentMethodsCmd, webhooksCmd, chainsCmd, tokensCmd, authCmd, healthCmd, aliasesCmd, keysCmd, configCmd, envCmd, disputesCmd, analyticsCmd)
}

// Help groups. `completion`, `help`, `update` and `version` deliberately have none:
// cobra lists an ungrouped command under "Additional Commands", which is exactly the
// weight the meta commands should carry next to `payments`.
const (
	groupPayments = "payments"
	groupAccount  = "account"
	groupCatalog  = "catalog"
	groupLocal    = "local"
)

// assignGroup tags commands with a help group. A helper because the alternative is
// setting GroupID at each command's own definition, which spreads one decision — the
// shape of `rail0 --help` — across a dozen files.
func assignGroup(id string, cmds ...*cobra.Command) {
	for _, c := range cmds {
		c.GroupID = id
	}
}

// newClient builds a RAIL0 gateway client.
// If a JWT is stored for this gateway (~/.cache/rail0/tokens/<digest>), it is
// sent as Authorization: Bearer on every request.
//
// The token goes in ClientOptions.Token, not a hand-built Authorization header.
// Both end up as the same header, but Token is the SDK's mutable slot: Auth.Login
// replaces it via SetToken, so a client built with no cached token starts
// unauthenticated and becomes authenticated the moment ensureSession signs in on it.
// A static header cannot be replaced that way, and seeding one from a STALE cached
// token was worse than seeding nothing — it kept being sent. (#59)
func newClient() *rail0.Client {
	return newClientFor(resolveBaseURL())
}

// newClientFor is newClient for a named gateway rather than the current one — for the
// one call that must reach a gateway the configuration has already moved away from:
// revoking the session `env` just left.
func newClientFor(baseURL string) *rail0.Client {
	opts := rail0.ClientOptions{
		BaseURL: baseURL,
		Token:   token.Load(baseURL),
		Timeout: httpTimeout(),
	}
	if debug {
		opts.Logger = makeDebugLogger()
	}
	return rail0.NewClient(opts)
}

// newReadClient is newClient plus transient-failure resilience: 2 retries on a
// network error and opt-in retry of 429s (sleeping the server's Retry-After —
// the budget the SDK already implements and nothing enabled, while the CLI's
// own 429 branch parsed the header just to fail immediately).
//
// READ COMMANDS ONLY, on purpose. The SDK re-sends on a network failure
// whatever the method: for a submit whose first attempt actually LANDED, a
// retry turns an operation that succeeded into a misleading follow-up error
// (the gateway answers the replay with a 404/422), which is worse than the
// plain network error. Fund-moving commands keep newClient. (#102)
func newReadClient() *rail0.Client {
	baseURL := resolveBaseURL()
	opts := rail0.ClientOptions{
		BaseURL:    baseURL,
		Token:      token.Load(baseURL),
		Timeout:    httpTimeout(),
		MaxRetries: 2,
		RetryOn429: true,
	}
	if debug {
		opts.Logger = makeDebugLogger()
	}
	return rail0.NewClient(opts)
}

// httpTimeout is the per-request HTTP timeout, resolved through the config layer
// (RAIL0_HTTP_TIMEOUT > `config set http-timeout` > SDK default). Distinct from
// `--timeout` on the wait commands, which is the POLLING deadline — one bounds a
// single request, the other bounds how long the CLI keeps asking. 0/garbage falls
// back to the SDK default (30s).
//
// The warning still names RAIL0_HTTP_TIMEOUT only when that is where the bad value
// came from: `config set` validates, so a stored value cannot be garbage, and
// blaming the env var for a file value would send the user to the wrong place.
func httpTimeout() time.Duration {
	raw := os.Getenv("RAIL0_HTTP_TIMEOUT")
	fromEnv := raw != ""
	if !fromEnv {
		raw = configuredDefault("http-timeout")
	}
	if raw == "" {
		return 0 // SDK default
	}
	secs, err := strconv.Atoi(raw)
	if err != nil || secs <= 0 {
		if fromEnv {
			fmt.Fprintf(os.Stderr, "warning: ignoring invalid RAIL0_HTTP_TIMEOUT=%q (want seconds)\n", raw)
		} else {
			fmt.Fprintf(os.Stderr, "warning: ignoring invalid stored http-timeout=%q (want seconds)\n", raw)
		}
		return 0
	}
	return time.Duration(secs) * time.Second
}

// outputFlagMode reports which output mode was selected BY FLAG, or "" when none
// was. Kept separate from the resolution below so `config` can name the flag as
// the origin, instead of reporting a stored default that is being overridden.
func outputFlagMode() string {
	switch {
	case quietOut:
		return "quiet"
	case prettyOut:
		return "pretty"
	case jsonOut:
		return "json"
	}
	return ""
}

// applyOutputMode sets the renderer from the flags, falling back to the stored
// `output` default when none was passed.
//
// The flags stay authoritative and are never re-derived from config: a stored
// `output: quiet` must not survive an explicit --pretty on the command line. Only
// "auto" is a no-op, since that IS the built-in behaviour (colorized JSON on a
// terminal, plain JSON when piped).
func applyOutputMode() {
	mode := outputFlagMode()
	if mode == "" {
		mode = configuredDefault("output")
	}
	ui.SetJSON(mode == "json")
	ui.SetPretty(mode == "pretty")
	ui.SetQuiet(mode == "quiet")
}

// sessionReuseMargin is how long a cached session must still have to live before a
// signing command reuses it rather than signing in afresh.
//
// Merely "not expired yet" was the test, so a token with seconds left was reused and
// the 401 could arrive partway through prepare → sign → submit → wait — after the
// broadcast, turning an operation that went out into an error. A fresh sign-in costs
// one signature with a key the command already holds, so the margin is generous: a
// minute for the round-trips, plus the whole --wait budget when the command will
// poll, since those reads use the same session.
func sessionReuseMargin() time.Duration {
	margin := time.Minute
	if waitFlag {
		margin += waitTimeout
	}
	return margin
}

// sessionOutlives reports whether the claims' exp is more than margin away. Callers
// pass claims parseJWTClaims accepted, so exp is known to be numeric.
func sessionOutlives(claims map[string]any, margin time.Duration) bool {
	exp, _ := claims["exp"].(float64)
	return time.Unix(int64(exp), 0).After(time.Now().Add(margin))
}

// signedInAs records, per client, the address ensureSession signed in as during this
// process (see ensureSession). A CLI process runs one command, so it never grows past
// a client or two.
var signedInAs = map[*rail0.Client]string{}

// ensureSession guarantees a valid SIWE session whose address matches `addr`
// before a write to /payments. The gateway now gates every /payments endpoint on
// a session, and `create` additionally requires the payer to BE the authenticated
// address — so this signs in with the caller's key (the same key the command
// signs the payload with, hence the session address == addr) when the stored
// token is missing, expired, too close to expiry (sessionReuseMargin), or for a
// different address. A no-op when already logged in as `addr`. The token is saved
// so follow-up reads (e.g. payments get) reuse the session — replacing an expiring
// one for the same address, but never another address's (see below).
//
// It signs in through the CALLER'S client, which is the whole point of taking one.
// It used to log in on a throwaway `rail0.NewClient(...)`, so Auth.Login stashed the
// JWT on that client and then discarded it: on a fresh machine with no cached token,
// the command printed "Signed in as 0x…" and the very next request on the real client
// — still built with no token — came back 401 "run 'rail0 auth login' first". Logging
// in here mutates the client the command actually uses. (#59)
func ensureSession(ctx context.Context, client *rail0.Client, keyHex, addr string) error {
	// Already signed in as addr on THIS client earlier in the command: its token is
	// minutes old, so it outlives any margin. Without this, a command that signs in
	// twice (the interactive picker, then the operation's signer check) repeated the
	// nonce + verify round-trip whenever the stored session was another address's —
	// the one case where the fresh token is deliberately not saved (#95).
	if signedInAs[client] != "" && strings.EqualFold(signedInAs[client], addr) {
		return nil
	}
	baseURL := resolveBaseURL()
	// Whether a DIFFERENT live session is already stored decides if we may
	// persist this one — see the token.Save call below (#95).
	otherSessionStored := false
	if t := token.Load(baseURL); t != "" {
		// parseJWTClaims already rejects an expired token (#71); on top of that the
		// session must OUTLIVE the command about to use it — see sessionReuseMargin.
		if claims, err := parseJWTClaims(t); err == nil {
			sub, _ := claims["sub"].(string)
			sameAddress := strings.EqualFold(sub, addr)
			if sameAddress && sessionOutlives(claims, sessionReuseMargin()) {
				return nil
			}
			// Only a session for ANOTHER address is protected from eviction. The
			// same address's session, merely too close to expiry, is this machine's
			// session being renewed: replacing it is the point. Treating it as
			// "other" signed in afresh on every -w command near expiry, printed "for
			// this command only", and never saved the new token — so the next
			// command did it all again, until the stored one finally lapsed.
			otherSessionStored = !sameAddress
		}
	}
	privKey, err := rail0.HexToPrivateKey(keyHex)
	if err != nil {
		return fmt.Errorf("invalid signing key: %w", err)
	}
	// Auth.Login stashes the JWT on the client it is called on (SetToken), so this
	// leaves `client` authenticated for every subsequent request the command makes.
	authResp, err := client.Auth.Login(ctx, privKey, siweHost(resolveBaseURL()))
	if err != nil {
		return fmt.Errorf("sign-in failed: %w", err)
	}
	signedInAs[client] = addr
	// Persist ONLY when this is the machine's session for this gateway. When a
	// different live session is already stored, the sign-in we just did serves
	// this one command (the key came from its -p flag) and must not evict it:
	// a single payer-side `payments release -p @payer` used to overwrite the
	// merchant's JWT with the buyer's account-less token, after which `wallets`,
	// `webhooks` and analytics answered 403 account_required and `payments list`
	// silently showed the buyer's payments. The client is authenticated either
	// way — Auth.Login stashed the JWT on it — so the command still works. (#95)
	if otherSessionStored {
		fmt.Fprintf(os.Stderr,
			"Signed in as %s for this command only (the stored session was left in place)\n", addr)
		return nil
	}
	if err := token.Save(baseURL, authResp.Token); err != nil {
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
	t := token.Load(resolveBaseURL())
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
//
// Deliberately does NOT validate: `rail0 config` has to be able to DISPLAY a bad
// value so the user can see what is wrong with it. Validation happens in
// checkBaseURL, wired to every gateway-touching command below. (#62)
func resolveBaseURL() string {
	value, _ := effectiveSetting(configKeys["base-url"])
	return value
}

// Command trees that never build a gateway client, and are therefore exempt from
// base-url validation. This exemption is the point, not an oversight: these are the
// tools used to INSPECT and REPAIR a bad base URL (`rail0 config`, `rail0 env
// development`), so failing them on the very value they exist to fix would lock the
// CLI out of its own recovery path. Verified against the newClient() call sites —
// none of these trees has one.
//
// `version` and `token` read nothing but local state. `init` is the guided repair
// path itself: the only gateway it talks to is the one its env step just validated.
// cobra's hidden `__complete` / `__completeNoDesc` serve TAB completion, which a bad
// base URL used to break outright — every completion returned the validation error.
var localOnlyCommands = map[string]bool{
	"config": true, "env": true, "keys": true, "aliases": true,
	"help": true, "completion": true, "version": true, "token": true, "init": true, "update": true,
	cobra.ShellCompRequestCmd: true, cobra.ShellCompNoDescRequestCmd: true,
}

// topLevelName returns the first-level command `cmd` sits under — "payments" for
// `rail0 payments capture`, "config" for `rail0 config set` — so the exemption above
// applies to a whole tree rather than being repeated on each leaf.
func topLevelName(cmd *cobra.Command) string {
	for c := cmd; c != nil && c.Parent() != nil; c = c.Parent() {
		if c.Parent().Parent() == nil {
			return c.Name()
		}
	}
	return cmd.Name()
}

// checkBaseURL validates the base URL a command is about to talk to, before it can
// send anything. Attached as the root PersistentPreRunE, so it covers ALL four
// resolution paths at once — flag, env var, config file, built-in default — which
// is what validateBaseURL being wired only to `config set` / `env switch` used to
// miss: --base-url and RAIL0_BASE_URL reached the client entirely unchecked.
func checkBaseURL(cmd *cobra.Command) error {
	if localOnlyCommands[topLevelName(cmd)] {
		return nil
	}
	return validateBaseURL(resolveBaseURL())
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
		// Bodies are redacted, never logged verbatim: a submit request carries
		// `signed_transaction`, a login carries `signature`, and the responses carry
		// the session JWT and webhook shared secrets. See redactBody. (#51)
		if e.RequestBody != nil {
			fields = append(fields, "request", redactBody(e.RequestBody))
		}
		if e.ResponseBody != nil {
			fields = append(fields, "response", redactBody(e.ResponseBody))
		}
		if e.Err != nil {
			logger.Error("HTTP", append(fields, "error", e.Err)...)
		} else {
			logger.Debug("HTTP", fields...)
		}
	}
}
