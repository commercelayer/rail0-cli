package cmd

import (
	"fmt"
	"os"
	"time"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/token"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// loginHint is the one-liner shown whenever a command needs a session but none
// is usable. Kept in one place so `token`, `auth status`, and any future caller
// stay consistent — and it spells out that no account is required (a buyer can
// log in with any wallet).
const loginHint = "Run: rail0 auth login -p 0xYourKey… (any wallet — no gateway account required)"

// ── token ───────────────────────────────────────────────────────────────────
// `rail0 token` reveals the stored session JWT (the singular counterpart to the
// `rail0 tokens` catalog command, which is unrelated). If there's no usable
// session it prints the login hint to stderr and exits 0 rather than erroring —
// "you're not logged in" is a normal state, not a failure.

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Print the stored session token, or suggest logging in",
	Long: `Print the JWT session token saved by the last 'rail0 auth login' for the
current gateway (~/.cache/rail0/tokens/<digest of the base URL>), together with
its expiry. If no valid token is stored, it suggests logging in instead.

This is the singular session token — not to be confused with 'rail0 tokens',
which lists the gateway's accepted payment tokens.`,
	Example: `  rail0 token`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		info, hint := tokenSession(token.Load(resolveBaseURL()))
		if hint != "" {
			fmt.Fprintln(os.Stderr, hint)
			return nil
		}
		return ui.RunAPI(func() (any, error) { return info, nil })
	},
}

func init() {
	rootCmd.AddCommand(tokenCmd)
}

// ── shared token helpers ──────────────────────────────────────────────────────

// tokenSession turns a stored/just-issued JWT into the display info, or returns
// a non-empty hint describing why there is no usable session (no token, or a
// token that is malformed/expired — parseJWTClaims rejects both). Exactly one of
// (info, hint) is non-empty. Pure and side-effect-free, so it is unit-tested
// directly without driving the cobra command.
func tokenSession(jwt string) (info map[string]any, hint string) {
	if jwt == "" {
		return nil, "Not logged in. " + loginHint
	}
	claims, err := parseJWTClaims(jwt)
	if err != nil {
		return nil, "Stored token is invalid or expired. " + loginHint
	}
	expiresAt, expiresIn := tokenExpiry(claims)
	return map[string]any{
		"token":      jwt,
		"expires_at": expiresAt,
		"expires_in": expiresIn,
	}, ""
}

// loginSummary is what `auth login` prints on success. The raw JWT is sensitive,
// so it is included ONLY when the caller passed --token; the default summary
// carries just the non-secret session facts (address, account when present, and
// expiry). account_id is omitted for an account-less (e.g. buyer) session.
func loginSummary(a *rail0.AuthResponse, showToken bool) map[string]any {
	info := map[string]any{"address": a.Address, "expires_at": a.ExpiresAt}
	if a.AccountID != "" {
		info["account_id"] = a.AccountID
	}
	if claims, err := parseJWTClaims(a.Token); err == nil {
		_, expiresIn := tokenExpiry(claims)
		info["expires_in"] = expiresIn
	}
	if showToken {
		info["token"] = a.Token
	}
	return info
}

// tokenExpiry derives the absolute expiry (RFC3339, UTC) and a human "expires
// in" string from a JWT's exp claim. parseJWTClaims has already validated exp is
// present and in the future, so the type assertion is safe for its callers.
func tokenExpiry(claims map[string]any) (expiresAt, expiresIn string) {
	exp := time.Unix(int64(claims["exp"].(float64)), 0).UTC()
	return exp.Format(time.RFC3339), humanUntil(time.Until(exp))
}

// humanUntil formats a remaining duration as "Xh Ym" (or "Ym" under an hour),
// rounded to the minute; a non-positive duration reads "expired".
func humanUntil(d time.Duration) string {
	d = d.Round(time.Minute)
	if d <= 0 {
		return "expired"
	}
	if h := int(d.Hours()); h > 0 {
		return fmt.Sprintf("%dh %dm", h, int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}
