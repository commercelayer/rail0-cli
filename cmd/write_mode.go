package cmd

import (
	"context"
	"fmt"
	"strings"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/token"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// Write mode.
//
// The gateway lets an operator session READ. A request that writes is refused 403
// `write_mode_required` unless it carries a write-mode token: a second JWT for the same
// session, valid one hour by default and never longer than the session, issued by
// POST /admin/write_mode with no further signature. Both switches are audited.
//
// The CLI keeps that token beside the session's (token.SaveWriteMode). While a valid one
// is stored, EVERY request this CLI makes to that gateway carries it instead of the
// session token (bearerToken): write mode is a state of the operator's session, not of
// one command, so the operator's writes outside `rail0 admin` (`account update
// --account …`, a redrive, a wallet switched off) work in it too, as they do in the
// console. It ends by itself when the token expires, with 'admin write-mode off', and
// with the session ('auth logout', a new 'auth login').
//
// Agent mode never carries it: see bearerToken.

// writeModeToken returns this gateway's stored write-mode token when it is usable: not
// expired, and issued to the address of the stored session. Anything else — expired,
// unreadable, another address's, or no session at all — is removed and reads as "".
func writeModeToken(baseURL string) string {
	wt := token.LoadWriteMode(baseURL)
	if wt == "" {
		return ""
	}
	if writeModeBelongs(wt, token.Load(baseURL)) {
		return wt
	}
	_ = token.DeleteWriteMode(baseURL)
	return ""
}

// writeModeBelongs reports whether writeToken is a live token of the same address as
// the session token. parseJWTClaims rejects an expired token, so both must be live.
func writeModeBelongs(writeToken, sessionToken string) bool {
	if sessionToken == "" {
		return false
	}
	wc, err := parseJWTClaims(writeToken)
	if err != nil {
		return false
	}
	sc, err := parseJWTClaims(sessionToken)
	if err != nil {
		return false
	}
	wsub, _ := wc["sub"].(string)
	ssub, _ := sc["sub"].(string)
	return wsub != "" && strings.EqualFold(wsub, ssub)
}

// bearerToken is the token a client for this gateway carries: the write-mode token
// while write mode is on, the session token otherwise.
//
// Never the write-mode token in agent mode. An operator's write is a person's decision,
// and a write-mode window opened in a terminal must not become an agent's authority to
// write for the rest of the hour: the agent's requests carry the session token, and the
// gateway refuses the operator's writes with it.
//
// And never an operator's session in agent mode: see operator_session.go.
func bearerToken(baseURL string) string {
	if operatorSessionWithheld(baseURL) {
		return ""
	}
	if !agentMode() {
		if wt := writeModeToken(baseURL); wt != "" {
			return wt
		}
	}
	return token.Load(baseURL)
}

// endWriteMode switches write mode off for this gateway: it revokes the stored
// write-mode token on the gateway, best-effort, and removes it locally whatever the
// gateway answered. had says a token was stored; revoked says the gateway recorded the
// revocation (false, with why, when it could not be reached or refused: the token then
// stays valid until it expires, at most an hour after it was issued).
//
// Called before anything that ends or replaces the session, so a write-mode token never
// outlives the session it was issued to on this machine.
func endWriteMode(baseURL string) (had, revoked bool, why string) {
	wt := token.LoadWriteMode(baseURL)
	if wt == "" {
		return false, false, ""
	}
	if _, err := parseJWTClaims(wt); err != nil {
		why = "the stored write-mode token had already expired"
	} else {
		ok, err := clientWithToken(baseURL, wt).Admin.DisableWriteMode(context.Background())
		switch {
		case err != nil:
			why = fmt.Sprintf("could not revoke it on %s (%s)", baseURL, humanizeError(err))
		case !ok:
			why = "the gateway could not record the revocation, so that token is valid until it expires"
		default:
			revoked = true
		}
	}
	if err := token.DeleteWriteMode(baseURL); err != nil && why == "" {
		why = fmt.Sprintf("could not remove the local write-mode token: %v", err)
	}
	return true, revoked, why
}

// clientWithToken builds a client that carries exactly tok, whatever is stored: for the
// calls that are ABOUT a token (revoking the write-mode one, signing a session out).
func clientWithToken(baseURL, tok string) *rail0.Client {
	opts := rail0.ClientOptions{BaseURL: baseURL, Token: tok, Timeout: httpTimeout()}
	if debug {
		opts.Logger = makeDebugLogger()
	}
	return rail0.NewClient(opts)
}

// writeModeRequired is the error of an operator write attempted with write mode off:
// the gateway's own code and exit class (403 → exit 4), raised before the request so the
// message can name the command that fixes it.
func writeModeRequired() error {
	return &sessionError{
		Code: "write_mode_required",
		Msg:  "write mode is off: an operator session reads until 'rail0 admin write-mode on' (it lasts an hour and is audited)",
		Exit: 4,
	}
}

// requireWriteMode refuses an operator's write here, before anything is read or asked:
// no session, or write mode off. The gateway would refuse the write the same way; said
// first, the operator is not shown figures and a question for a command that cannot run.
func requireWriteMode() error {
	if _, err := sessionAddress(); err != nil {
		return err
	}
	if writeModeToken(resolveBaseURL()) == "" {
		return writeModeRequired()
	}
	return nil
}

// writeModeInfo is what the write-mode commands print: never the token.
func writeModeInfo(writeToken string) map[string]any {
	claims, err := parseJWTClaims(writeToken)
	if writeToken == "" || err != nil {
		return map[string]any{"active": false}
	}
	expiresAt, expiresIn := tokenExpiry(claims)
	return map[string]any{"active": true, "expires_at": expiresAt, "expires_in": expiresIn}
}

var adminWriteModeCmd = &cobra.Command{
	Use:   "write-mode",
	Short: "Operator write mode: see it, switch it on or off (shows it when run bare)",
	Long: `An operator session reads. A write is refused 403 'write_mode_required' until write
mode is on: a second token for the same session, valid one hour by default and never
longer than the session. Switching it on asks for no signature; both switches are
audited on the gateway.

While it is on, every command this CLI runs against this gateway carries it, so the
operator's writes outside 'rail0 admin' work too ('account update --account …',
'payments redrive', a wallet switched off). It ends when the hour is up, with
'rail0 admin write-mode off', with 'rail0 auth logout' and with a new 'rail0 auth login'.

The token is stored beside the session's and is never printed.`,
	Example: `  rail0 admin write-mode        # is it on, and until when
  rail0 admin write-mode on
  rail0 admin write-mode off`,
	Args: cobra.NoArgs,
	RunE: operatorOnly(runWriteModeStatus),
}

var adminWriteModeStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Whether write mode is on, and until when",
	Args:  cobra.NoArgs,
	RunE:  operatorOnly(runWriteModeStatus),
}

// runWriteModeStatus answers from the stored token and then asks the gateway, which is
// the one that knows whether the token was revoked from elsewhere.
func runWriteModeStatus(_ *cobra.Command, _ []string) error {
	baseURL := resolveBaseURL()
	if _, err := sessionAddress(); err != nil {
		return err
	}
	return ui.RunAPI(func() (any, error) {
		wt := writeModeToken(baseURL)
		if wt == "" {
			return writeModeInfo(""), nil
		}
		st, err := clientWithToken(baseURL, wt).Admin.WriteMode(context.Background())
		if err != nil {
			return nil, err
		}
		if !st.Active {
			_ = token.DeleteWriteMode(baseURL) // revoked elsewhere: stop carrying it
			return writeModeInfo(""), nil
		}
		return writeModeInfo(wt), nil
	})
}

var adminWriteModeOnCmd = &cobra.Command{
	Use:   "on",
	Short: "Switch write mode on for an hour (audited)",
	Args:  cobra.NoArgs,
	RunE: operatorOnly(func(_ *cobra.Command, _ []string) error {
		baseURL := resolveBaseURL()
		if _, err := sessionAddress(); err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			// Asked with the SESSION token, also when write mode is already on: the
			// new token replaces the stored one, and the old one is revoked first so
			// two are never live for this machine.
			endWriteMode(baseURL)
			wm, err := clientWithToken(baseURL, token.Load(baseURL)).Admin.EnableWriteMode(context.Background())
			if err != nil {
				return nil, err
			}
			if err := token.SaveWriteMode(baseURL, wm.Token); err != nil {
				return nil, fmt.Errorf("write mode was switched on but its token could not be stored (%w): "+
					"it expires by itself at %s", err, wm.ExpiresAt)
			}
			info := writeModeInfo(wm.Token)
			ui.Notice("write_mode_on", map[string]any{"expires_at": info["expires_at"]},
				fmt.Sprintf("Write mode is on until %s (%s). Operator writes from this machine are accepted and audited; "+
					"'rail0 admin write-mode off' ends it.", info["expires_at"], info["expires_in"]))
			return info, nil
		})
	}),
}

var adminWriteModeOffCmd = &cobra.Command{
	Use:   "off",
	Short: "Switch write mode off: revoke its token and remove it",
	Args:  cobra.NoArgs,
	RunE: operatorOnly(func(_ *cobra.Command, _ []string) error {
		baseURL := resolveBaseURL()
		return ui.RunAPI(func() (any, error) {
			had, revoked, why := endWriteMode(baseURL)
			out := map[string]any{"active": false, "was_active": had}
			if had {
				out["revoked"] = revoked
			}
			if had && !revoked {
				out["detail"] = why
				ui.Notice("warning", map[string]any{"what": "write_mode_not_revoked", "why": why},
					fmt.Sprintf("warning: the write-mode token was removed here but NOT revoked — %s", why))
			}
			return out, nil
		})
	}),
}
