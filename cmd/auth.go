package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

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

var loginPrivateKey string

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
commands that require authentication (e.g. 'payments list').
Override with the RAIL0_TOKEN environment variable.

The signing key is resolved like every other signing command: pass -p as a raw
0x-prefixed hex key, as @name for a key saved in the OS keychain ('rail0 keys
add'), or omit it to fall back to the RAIL0_PRIVATE_KEY environment variable.`,
	Example: `  rail0 auth login -p @payee     # or -p 0xYourPrivateKey…`,
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

		tokenPath, saveErr := token.SaveAndReport(authResp.Token)
		if saveErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save token to disk: %v\n", saveErr)
			fmt.Fprintf(os.Stderr, "Set RAIL0_TOKEN=%s to authenticate manually.\n", authResp.Token)
		} else {
			fmt.Fprintf(os.Stderr, "Token saved to %s\n", tokenPath)
		}

		return ui.RunAPI(func() (any, error) { return authResp, nil })
	},
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
		t := token.Load()
		if t == "" {
			fmt.Println("Not logged in. Run: rail0 auth login -p 0xYourKey…")
			return nil
		}
		claims, err := parseJWTClaims(t)
		if err != nil {
			fmt.Println("Token invalid or expired. Run: rail0 auth login -p 0xYourKey…")
			return nil
		}
		exp := time.Unix(int64(claims["exp"].(float64)), 0).UTC()
		expiresIn := time.Until(exp).Round(time.Minute)
		var expiresInStr string
		if expiresIn <= 0 {
			expiresInStr = "expired"
		} else {
			h := int(expiresIn.Hours())
			m := int(expiresIn.Minutes()) % 60
			if h > 0 {
				expiresInStr = fmt.Sprintf("%dh %dm", h, m)
			} else {
				expiresInStr = fmt.Sprintf("%dm", m)
			}
		}
		return ui.RunAPI(func() (any, error) {
			info := map[string]any{
				"address":    claims["sub"],
				"account_id": claims["account_id"],
				"expires_at": exp.Format(time.RFC3339),
				"expires_in": expiresInStr,
			}
			return info, nil
		})
	},
}

// ── auth logout ───────────────────────────────────────────────────────────────

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove the stored authentication token",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := token.Delete(); err != nil {
			return err
		}
		fmt.Println("Logged out.")
		return nil
	},
}

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	authLoginCmd.Flags().StringVarP(&loginPrivateKey, "private-key", "p", "",
		"Signing key: 0x-prefixed hex, @name for an OS-keychain key ('rail0 keys add'), or RAIL0_PRIVATE_KEY")
	authLoginCmd.Flags().StringVar(&loginPrivateKey, "pk", "", "Alias for --private-key")
	_ = authLoginCmd.Flags().MarkHidden("pk")

	authCmd.AddCommand(authLoginCmd, authStatusCmd, authLogoutCmd)
}
