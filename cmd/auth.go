package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/rail0/cli/internal/token"
	"github.com/rail0/cli/internal/ui"
	rail0 "github.com/rail0/go-sdk"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with the RAIL0 gateway using SIWE",
}

// ── auth login ────────────────────────────────────────────────────────────────

var (
	loginPrivateKey string
	loginShowToken  bool
)

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in with your Ethereum wallet (SIWE)",
	Long: `Authenticates with the RAIL0 gateway using Sign-In with Ethereum (EIP-4361).

  1. Fetches a single-use nonce from the API
  2. Builds a SIWE message and signs it with your private key
  3. Verifies the signature and stores the JWT token

Any wallet can sign in — a gateway account is NOT required. Signing proves
control of the address, which is enough to read your own address-scoped
resources (e.g. 'payments list' shows every payment where you are payer or
payee). A gateway account only unlocks merchant-scoped features.

The token is saved to ~/.cache/rail0/token and used automatically by all
commands that require authentication (e.g. 'payments list') — that file is the
only place the session is read from. On success the command prints the session
(address, expiry) but NOT the raw token — pass --token (-t) to also print the
token itself, or run 'rail0 token' later.

The signing key is resolved like every other signing command: pass -p as a raw
0x-prefixed hex key, as @name for a key saved in the OS keychain ('rail0 keys
add'), or omit it to fall back to the RAIL0_PRIVATE_KEY environment variable.`,
	Example: `  rail0 auth login -p @payee     # or -p 0xYourPrivateKey…
  rail0 auth login -p @payee -t  # also print the token + its expiry`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		client := newClient()

		// Resolve the key the same way the signing commands do, so `auth login`
		// accepts a raw 0x hex key, a keychain `@name` (rail0 keys add), or the
		// RAIL0_PRIVATE_KEY env var — one consistent key-source model everywhere.
		keyHex, err := signingKeyOrPrompt(loginPrivateKey)
		if err != nil {
			return err
		}
		privKey, err := rail0.HexToPrivateKey(keyHex)
		if err != nil {
			return fmt.Errorf("invalid private key: %w", err)
		}

		// Derive the SIWE domain from the SAME resolved base URL the client calls
		// (flag > env > config > default) — not the raw --base-url flag. Otherwise
		// a base URL set via `config set base-url` is ignored here and the SIWE
		// message is signed for the wrong domain, which the gateway rejects.
		host := siweHost(resolveBaseURL())

		authResp, err := client.Auth.Login(ctx, privKey, host)
		if err != nil {
			return err
		}

		tokenPath, saveErr := token.SaveAndReport(resolveBaseURL(), authResp.Token)
		if saveErr != nil {
			// Reports the failure and how to recover, but NEVER the token itself —
			// see tokenSaveFailureNotice for why that matters. (#64)
			fmt.Fprint(os.Stderr, tokenSaveFailureNotice(saveErr))
		} else {
			fmt.Fprintf(os.Stderr, "Token saved to %s\n", tokenPath)
		}

		// Print the session summary; include the raw token only with --token.
		summary := loginSummary(authResp, loginShowToken)
		return ui.RunAPI(func() (any, error) { return summary, nil })
	},
}

// tokenSaveFailureNotice is what `auth login` prints when the session could not be
// persisted. Extracted so the one property that matters here is testable: the raw
// JWT must NOT appear in it.
//
// It used to print `Set RAIL0_TOKEN=<jwt>`, which contradicted the command's own
// contract — the help text and loginSummary both withhold the token unless --token is
// passed — and put a live bearer credential into terminal scrollback, CI logs and any
// `2>` redirect. The trigger is mundane (a read-only filesystem, a sandboxed HOME), so
// it was not an exotic path. --token stays the single place the value is printed,
// because there the user asked for it. (#64)
func tokenSaveFailureNotice(saveErr error) string {
	return fmt.Sprintf(
		"warning: could not save token to disk: %v\n"+
			"The session is valid but was not persisted, so the next command will not be authenticated.\n"+
			"Fix the cache directory and sign in again — the token file is the only place a\n"+
			"session is read from, so there is no variable to fall back to.\n",
		saveErr,
	)
}

// siweHost derives the SIWE domain from a base URL: the bare host WITHOUT the
// scheme or port, so "https://api.rail0.xyz/" -> "api.rail0.xyz" and
// "http://localhost:9292" -> "localhost". The gateway's expected SIWE domain is
// host-only (Policy.siwe_domain defaults to "localhost" in dev, "api.rail0.xyz"
// in production), and rail0-admin signs host-only too — so keeping the port here
// would make the gateway reject the message with "SIWE domain mismatch".
func siweHost(baseURL string) string {
	if u, err := url.Parse(baseURL); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	// Fallback for a scheme-less base URL: drop scheme/trailing slash, then port.
	s := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(baseURL, "https://"), "http://"), "/")
	if h, _, found := strings.Cut(s, ":"); found {
		return h
	}
	return s
}

// ── auth status ───────────────────────────────────────────────────────────────

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		t := token.Load(resolveBaseURL())
		if t == "" {
			fmt.Fprintln(os.Stderr, "Not logged in. "+loginHint)
			return nil
		}
		claims, err := parseJWTClaims(t)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Token invalid or expired. "+loginHint)
			return nil
		}
		expiresAt, expiresIn := tokenExpiry(claims)
		return ui.RunAPI(func() (any, error) {
			return map[string]any{
				"address":    claims["sub"],
				"account_id": claims["account_id"],
				"expires_at": expiresAt,
				"expires_in": expiresIn,
			}, nil
		})
	},
}

// ── auth logout ───────────────────────────────────────────────────────────────

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "End the session: revoke the token on the gateway and remove it locally",
	Long: `End the current session.

The token is revoked on the gateway — its jti goes into the denylist, so the JWT
stops being accepted BEFORE it expires — and then removed from this machine.

Removing the local copy alone is not a logout: the JWT stays valid for the rest
of its lifetime, so anyone holding another copy keeps the session. The local file
is removed even when the revocation cannot be performed (an unreachable gateway
is a normal reason to log out), and the outcome says which of the two happened.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		baseURL := resolveBaseURL()
		if token.Load(baseURL) == "" {
			fmt.Println("No stored session for", baseURL)
			return nil
		}
		revoked := revokeCurrentSession()
		switch {
		case revoked.Revoked:
			fmt.Println("Logged out — the token was revoked on the gateway.")
		default:
			// Exit 0 on purpose: the local credential IS gone, which is what the
			// command was asked to do. The warning is what the operator acts on.
			fmt.Println("Logged out locally.")
			fmt.Fprintf(os.Stderr, "warning: the token was NOT revoked — %s\n", revoked.Why)
		}
		return nil
	},
}

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	authLoginCmd.Flags().StringVarP(&loginPrivateKey, "private-key", "p", "",
		"Signing key: 0x-prefixed hex, @name for an OS-keychain key ('rail0 keys add'), or RAIL0_PRIVATE_KEY")
	authLoginCmd.Flags().StringVar(&loginPrivateKey, "pk", "", "Alias for --private-key")
	_ = authLoginCmd.Flags().MarkHidden("pk")
	authLoginCmd.Flags().BoolVarP(&loginShowToken, "token", "t", false,
		"After signing in, also print the JWT token and its expiry to the terminal")

	authCmd.AddCommand(authLoginCmd, authStatusCmd, authLogoutCmd)
}
