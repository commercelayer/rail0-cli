package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/rail0/cli/internal/config"
	"github.com/rail0/cli/internal/token"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// environment is a named bundle of CLI settings applied together by
// `env <name>`. Every settings key must be a member of the configKeys
// whitelist, so an environment switch is validated and resolved through exactly
// the same machinery as `config set` — it is just a one-shot set of several
// keys.
type environment struct {
	name     string
	settings map[string]string
}

// environments lists the selectable environments in display order
// (development -> staging -> production). Adding a new per-environment
// parameter means adding its key/value to each entry here (and registering the
// key in configKeys).
//
// development and staging carry a chain and a token; production does not. That is
// not an omission: both non-production environments serve testnets, so Arc Testnet
// (4 confirmations, the quickest active one) is a safe default there, while the
// production chain set is a deployment decision and inventing one here would send
// a real payment to a chain nobody chose.
var environments = []environment{
	{name: "development", settings: map[string]string{
		"base-url": "http://localhost:9292",
		"chain-id": "5042002",
		"token":    "USDC",
	}},
	{name: "staging", settings: map[string]string{
		"base-url": "https://rail0-gateway-staging.fly.dev",
		"chain-id": "5042002",
		"token":    "USDC",
	}},
	{name: "production", settings: map[string]string{"base-url": config.DefaultBaseURL}},
}

// envIdentityKey is the setting that DEFINES which environment you are in: the
// gateway. Everything else in `settings` is a convenience the switch also writes.
//
// detectEnv compares only this one. Comparing them all would report "custom" for
// someone who switched chain after `env staging` — a true statement about the chain
// dressed up as a statement about the environment, and the answer to "which gateway
// am I talking to?" would change for a reason that has nothing to do with gateways.
const envIdentityKey = "base-url"

// lookupEnv resolves an environment by name, erroring with the valid set when
// unknown.
func lookupEnv(name string) (environment, error) {
	for _, e := range environments {
		if e.name == name {
			return e, nil
		}
	}
	names := make([]string, len(environments))
	for i, e := range environments {
		names[i] = e.name
	}
	return environment{}, fmt.Errorf("unknown environment %q (valid: %s)", name, strings.Join(names, ", "))
}

// detectEnv reports which environment the *effective* configuration currently
// matches, or "custom" if it matches none. It compares the identity key (the
// gateway — see envIdentityKey) against the value the commands would actually use
// (flag > env > config file > built-in default), so it stays truthful even when the
// base URL was set directly with `config set` rather than via `env`. The
// environments have distinct base-urls, so at most one matches; the first match
// wins.
func detectEnv() string {
	k, err := lookupKey(envIdentityKey)
	if err != nil {
		return "custom"
	}
	got, _ := effectiveSetting(k)
	for _, e := range environments {
		if e.settings[envIdentityKey] == got {
			return e.name
		}
	}
	return "custom"
}

// envParamRows returns one row per environment-managed key with its effective
// value and origin — the detail view shared by the bare `env` listing and
// `env current --details`. Keys are emitted in sorted order for stable output.
func envParamRows() []map[string]any {
	keySet := map[string]struct{}{}
	for _, e := range environments {
		for key := range e.settings {
			keySet[key] = struct{}{}
		}
	}
	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		k, err := lookupKey(key)
		if err != nil {
			continue
		}
		value, origin := effectiveSetting(k)
		rows = append(rows, map[string]any{"key": key, "value": value, "origin": origin})
	}
	return rows
}

var envDetails bool

// envCmd switches the active environment when given a name, and lists all
// environments (marking the current one) when run bare. `current` is a
// subcommand; any other single argument is treated as an environment name to
// switch to.
var envCmd = &cobra.Command{
	Use:   "env [name]",
	Short: "Switch the active environment (development/staging/production) in one shot",
	Long: `Switch a bundle of CLI settings at once by selecting an environment.

Run bare to list the selectable environments, with the current one marked; add
--details to also print the parameters each one declares. Pass a name to switch:
it writes that environment's settings to the config file, so every command uses
them.

The bundle is base-url plus, on development and staging, chain-id and token — so
a switch moves the default chain and token as well as the gateway. 'env <name>
--details' prints what it ended up with.

A stored value is still the lowest-priority source, so --base-url or
RAIL0_BASE_URL override it for a single command.`,
	Example: `  rail0 env                 # list the environments, marking the current one
  rail0 env -d              # ...with the parameters each one declares
  rail0 env staging         # switch to staging
  rail0 env staging -d      # ...and also print the resulting parameters
  rail0 env current         # show the active environment
  rail0 env current -d      # ...with every parameter's effective value`,
	Args: cobra.MaximumNArgs(1),
	// The environment names are a fixed, short set, so `rail0 env st<TAB>` completes
	// rather than sending the reader to --help. (#110)
	ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		names := make([]string, 0, len(environments)+1)
		for _, e := range environments {
			names = append(names, e.name)
		}
		return append(names, "current"), cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(_ *cobra.Command, args []string) error {
		if len(args) == 0 {
			return listEnvironments(envDetails)
		}
		return switchEnv(args[0], envDetails)
	},
}

// listEnvironments names the selectable environments and marks the active one,
// adding each one's declared parameters only under --details.
//
// The bare listing used to dump every environment's full settings map, which
// made -d mean nothing here while it meant "add the parameters" on both `env
// <name>` and `env current`. A persistent flag that changes two of the three
// commands it is attached to is worse than no flag: the reader cannot tell
// whether the detail they are looking at is the flag's doing or the command's.
// Answering "which environments can I pick?" is what the bare form is for.
func listEnvironments(details bool) error {
	rows := environmentRows(details)
	return ui.RunAPI(func() (any, error) { return rows, nil })
}

// environmentRows builds the listing. Pure and separate from the rendering for
// the same reason switchResult and envParamRows are: the -d gating is what the
// test pins, and it should not have to capture stdout to do it.
func environmentRows(details bool) []map[string]any {
	current := detectEnv()
	rows := make([]map[string]any, 0, len(environments))
	for _, e := range environments {
		row := map[string]any{
			"env":     e.name,
			"current": e.name == current,
		}
		if details {
			row["settings"] = e.settings
		}
		rows = append(rows, row)
	}
	return rows
}

// switchEnv applies an environment's settings to the config file. Each value is
// validated through the same validator as `config set` before being persisted,
// so a switch can never write a value the commands would reject. The
// confirmation is just the status and env name; the resulting parameters are
// listed only when details is set (the -d flag), matching `env current`.
func switchEnv(name string, details bool) error {
	e, err := lookupEnv(name)
	if err != nil {
		return err
	}
	// BEFORE the base URL moves: end the session on the gateway we are LEAVING.
	// Tokens are stored per gateway, so a switch used to simply orphan the old
	// one — a bearer credential left on disk, valid on that gateway until its
	// `exp` (24h), for a session nobody is watching any more. Revoking puts its
	// jti in the gateway's denylist, so the file left behind is worthless.
	//
	// Deliberately before cfg.Save(): revokeCurrentSession resolves the CURRENT
	// base URL, and after the save that would be the new environment's.
	//
	// Only when there IS somewhere to leave. This ran unconditionally, so switching
	// to the environment you were already on logged you out of it: nothing was
	// orphaned (the base URL does not move, so the token still belongs to the
	// gateway in use), but its jti went on the denylist and the file was deleted.
	// `rail0 init` made that the default path rather than an edge case — it
	// pre-selects the current environment via detectEnv() and always calls through
	// here, so accepting the default and declining to save a key ended the "guided
	// setup" logged out, from a command whose help says nothing here is
	// irreversible.
	// The zero value reads as "there was no session to end", which is exactly the
	// state a same-environment switch is in: the session it would have revoked is
	// the one still in use.
	var revoked sessionRevocation
	if detectEnv() != name {
		revoked = revokeCurrentSession()
	}

	cfg, err := loadConfigForWrite()
	if err != nil {
		return err
	}
	if cfg.Settings == nil {
		cfg.Settings = map[string]string{}
	}
	for key, value := range e.settings {
		k, err := lookupKey(key)
		if err != nil {
			return err
		}
		if k.validate != nil {
			if err := k.validate(value); err != nil {
				return fmt.Errorf("environment %q: %w", name, err)
			}
		}
		cfg.Settings[key] = value
	}
	if err := cfg.Save(); err != nil {
		return err
	}
	// AFTER the save, so resolveBaseURL() is the environment we just moved to.
	// Sessions are per gateway, so arriving somewhere unauthenticated is the norm
	// after a switch — and it used to be completely silent: the next command failed
	// with a 401 that looked like the switch had broken something.
	loggedIn := hasLiveSession()
	if !loggedIn {
		fmt.Fprintln(os.Stderr, "No session for this gateway. "+loginHint)
	}
	return ui.RunAPI(func() (any, error) { return switchResult(name, details, revoked, loggedIn), nil })
}

// hasLiveSession reports whether the CURRENT gateway has a usable session: a
// stored token that still parses (parseJWTClaims rejects an expired one), which is
// the same test every authenticated command applies.
func hasLiveSession() bool {
	stored := token.Load(resolveBaseURL())
	if stored == "" {
		return false
	}
	_, err := parseJWTClaims(stored)
	return err == nil
}

// sessionRevocation records what happened to the session of the gateway being
// left, so `env` can report it instead of switching silently.
type sessionRevocation struct {
	// Had is false when there was no stored session — the common case, and the
	// reason the whole block must stay quiet rather than warn.
	Had bool
	// Revoked is true only when the gateway confirmed the denylist entry.
	Revoked bool
	// Why explains a Had-but-not-Revoked outcome (expired, unreachable, refused).
	Why string
}

// revokeCurrentSession ends the session on the CURRENT gateway and removes the
// local token, best-effort.
//
// Best-effort on purpose: switching away from an environment that is DOWN is a
// normal reason to switch, and refusing to move because its gateway will not
// answer would be the wrong trade. So the local token is deleted either way —
// leaving a copy behind after asking to leave is worse than an unrevoked one —
// and the outcome is reported so the operator knows whether the old JWT is dead
// or merely abandoned.
func revokeCurrentSession() sessionRevocation {
	baseURL := resolveBaseURL()
	stored := token.Load(baseURL)
	if stored == "" {
		return sessionRevocation{}
	}
	out := sessionRevocation{Had: true}

	// An already-expired token needs no revocation: parseJWTClaims rejects one,
	// so a nil error means the session is still live and worth killing.
	if _, err := parseJWTClaims(stored); err != nil {
		out.Why = "the stored session had already expired"
	} else {
		res, err := newClient().Auth.Logout(context.Background())
		switch {
		case err != nil:
			out.Why = fmt.Sprintf("could not reach %s to revoke it (%s)", baseURL, humanizeError(err))
		case !res.Revoked:
			// The gateway answered but could not record the revocation, so the
			// JWT is still accepted until exp. Reported, never smoothed over.
			out.Why = "the gateway could not record the revocation, so that token is valid until it expires"
		default:
			out.Revoked = true
		}
	}

	// Local copy goes regardless of the above.
	if err := token.Delete(baseURL); err != nil && out.Why == "" {
		out.Why = fmt.Sprintf("could not remove the local token: %v", err)
	}
	return out
}

// switchResult builds the confirmation payload for a switch: status, env name and
// whether the new gateway has a live session, plus each parameter's effective value
// and origin only when details is set
// (the -d flag). Split out from switchEnv so the gating is unit-testable
// without persisting or capturing stdout.
func switchResult(name string, details bool, revoked sessionRevocation, loggedIn bool) map[string]any {
	out := map[string]any{"status": "switched", "env": name, "logged_in": loggedIn}
	// Reported in the RESULT, not as a log line, so `-q`/`--json` callers see it
	// too: whether the session they were using is now dead is part of what the
	// switch did.
	switch {
	case !revoked.Had:
		out["previous_session"] = "none"
	case revoked.Revoked:
		out["previous_session"] = "revoked"
	default:
		out["previous_session"] = "abandoned"
		out["previous_session_detail"] = revoked.Why
	}
	if details {
		out["settings"] = envParamRows()
	}
	return out
}

var envCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the active environment (--details for every parameter)",
	Example: `  rail0 env current
  rail0 env current --details`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		current := detectEnv()
		return ui.RunAPI(func() (any, error) {
			out := map[string]any{"env": current}
			if envDetails {
				out["settings"] = envParamRows()
			}
			return out, nil
		})
	},
}

func init() {
	// --details/-d is persistent on `env`, so both `env <name>` (switch) and
	// `env current` accept it: without it they print just status/name; with it
	// they also list each parameter's effective value and origin.
	envCmd.PersistentFlags().BoolVarP(&envDetails, "details", "d", false, "Also print each parameter's effective value and origin")
	envCmd.AddCommand(envCurrentCmd)
}
