package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/token"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with the RAIL0 gateway using SIWE",
	Long: `Sign in to the gateway with an Ethereum wallet, using SIWE (EIP-4361).

Signing in proves you hold a key — it does not require a gateway account. ANY
wallet can log in; an account is what unlocks the merchant surface (wallets,
analytics, webhooks, disputes).

Who actually needs this:

  buyers      no. 'payments create' signs in on its own with the payer key,
              so paying needs nothing but a key.
  merchants   yes. The action commands (authorize, capture, refund, …) and
              every account view read the session.

The session is a JWT cached per gateway under the OS cache dir, so switching
'rail0 env' switches session with it rather than logging you out. The token is
never written to the config file. 'auth status' prints who you are and when it
expires; 'rail0 token' prints the token itself.`,
	Example: `  rail0 auth login -p @merchant-key
  rail0 auth status
  rail0 auth logout`,
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

The token is saved under ~/.cache/rail0/tokens/, one file per gateway (its name
is a digest of the base URL), and used automatically by all commands that
require authentication (e.g. 'payments list') — that file is the only place the
session is read from. The exact path is printed on success and by 'rail0 auth
status'. On success the command prints the session (address, expiry) but NOT the
raw token — pass --token (-t) to also print the token itself, or run 'rail0
token' later.

The signing key is resolved like every other signing command: pass -p as a raw
0x-prefixed hex key, as @name for a key saved in the OS keychain ('rail0 keys
add'), or omit it to fall back to the RAIL0_PRIVATE_KEY environment variable.`,
	Example: `  rail0 auth login -p @merchant-key   # or -p 0xYourPrivateKey…
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
	Long: `Report whether this gateway has a usable session, and where it is stored.

Sessions are per gateway, in a file named after a digest of the base URL, so
'token_path' is the only way to know which file backs the current session —
useful when several environments are in play, or to remove one by hand.`,
	Args: cobra.NoArgs,
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
				"token_path": token.Path(resolveBaseURL()),
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
		revoked := revokeCurrentSession()
		// Exit 0 even when the revocation failed, on purpose: the local credential IS
		// gone, which is what the command was asked to do. The warning (stderr) is
		// what the operator acts on; the result carries the same verdict for a
		// script, with the vocabulary `env` uses for previous_session.
		if revoked.Had && !revoked.Revoked {
			fmt.Fprintf(os.Stderr, "warning: the token was NOT revoked — %s\n", revoked.Why)
		}
		// A result rather than fmt.Println: plain text on stdout broke --json and -q
		// callers, and was the one command whose outcome a script could not read.
		return ui.RunAPI(func() (any, error) { return logoutResult(baseURL, revoked), nil })
	},
}

// logoutResult renders what `auth logout` did: session "none" (nothing was stored),
// "revoked" (the gateway denylisted it), or "abandoned" (removed locally only, with
// the reason).
func logoutResult(baseURL string, revoked sessionRevocation) map[string]any {
	out := map[string]any{"base_url": baseURL}
	switch {
	case !revoked.Had:
		out["session"] = "none"
	case revoked.Revoked:
		out["session"] = "revoked"
	default:
		out["session"] = "abandoned"
		out["session_detail"] = revoked.Why
	}
	return out
}

// ── auth revoke-all ───────────────────────────────────────────────────────────

var revokeAllPrivateKey string

var authRevokeAllCmd = &cobra.Command{
	Use:   "revoke-all",
	Short: "End EVERY session of an address, on every machine (for a key you no longer trust)",
	Long: `Revoke every session the gateway has issued to an address — the ones on this
machine, on other machines, and any a thief is holding.

'auth logout' cannot do this: it revokes ONE token, the one stored here. This is
per ADDRESS. The gateway records a cutoff instant, and every session of the
address issued before it is refused from then on, including this machine's.

It is authorized by a fresh signature of the address's KEY, not by the session:
whoever reacts to a leaked key holds the wallet, not the stolen token. The signed
message says "Sign out of RAIL0 everywhere", so a login signature cannot be
replayed to trigger it. The key is resolved like every signing command (-p as
0x hex, @name for a keychain key, or RAIL0_PRIVATE_KEY).

Afterwards, sign in again with 'rail0 auth login'. If you suspect the key itself
leaked, this ends the sessions but does not make the key safe: move the funds
and stop using it.`,
	Example: `  rail0 auth revoke-all -p @merchant-key`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		keyHex, err := signingKeyOrPrompt(revokeAllPrivateKey)
		if err != nil {
			return err
		}
		address, err := addressFromKey(keyHex)
		if err != nil {
			return err
		}
		privKey, err := rail0.HexToPrivateKey(keyHex)
		if err != nil {
			return fmt.Errorf("invalid private key: %w", err)
		}
		if err := confirmDestructive(fmt.Sprintf(
			"Revoke EVERY session of %s, on every machine? You will need to sign in again.", address)); err != nil {
			return err
		}
		baseURL := resolveBaseURL()
		client := newClient()
		return ui.RunAPI(func() (any, error) {
			res, err := client.Auth.RevokeAll(context.Background(), privKey, siweHost(baseURL))
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"address":       address,
				"revoked_all":   res.RevokedAll,
				"cutoff_at":     res.CutoffAt,
				"local_session": dropRevokedLocalSession(baseURL, address),
			}, nil
		})
	},
}

// dropRevokedLocalSession deletes this machine's stored session when revoke-all
// just killed it — the cutoff covers every session of the address, this one
// included — so the next command signs in instead of collecting a 401. A session
// of a DIFFERENT address is still valid and is kept. Returns what happened, for
// the result: "removed", "kept" (another address) or "none".
func dropRevokedLocalSession(baseURL, address string) string {
	stored := token.Load(baseURL)
	if stored == "" {
		return "none"
	}
	if claims, err := parseJWTClaims(stored); err == nil {
		if sub, _ := claims["sub"].(string); !strings.EqualFold(sub, address) {
			return "kept"
		}
	}
	// This address's session, or one too broken to read: either way it no longer
	// authenticates anything.
	_ = token.Delete(baseURL)
	return "removed"
}

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	authLoginCmd.Flags().StringVarP(&loginPrivateKey, "private-key", "p", "",
		"Signing key: 0x-prefixed hex, @name for an OS-keychain key ('rail0 keys add'), or RAIL0_PRIVATE_KEY")
	authLoginCmd.Flags().StringVar(&loginPrivateKey, "pk", "", "Alias for --private-key")
	_ = authLoginCmd.Flags().MarkHidden("pk")
	authLoginCmd.Flags().BoolVarP(&loginShowToken, "token", "t", false,
		"After signing in, also print the JWT token and its expiry to the terminal")

	authRevokeAllCmd.Flags().StringVarP(&revokeAllPrivateKey, "private-key", "p", "",
		"Key OF the address whose sessions to end: 0x hex, @name for an OS-keychain key, or RAIL0_PRIVATE_KEY")
	authRevokeAllCmd.Flags().StringVar(&revokeAllPrivateKey, "pk", "", "Alias for --private-key")
	_ = authRevokeAllCmd.Flags().MarkHidden("pk")
	addYesFlag(authRevokeAllCmd)

	authCmd.AddCommand(authLoginCmd, authStatusCmd, authLogoutCmd, authRevokeAllCmd)
}
