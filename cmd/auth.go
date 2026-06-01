package cmd

import (
	"context"
	"fmt"
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
	Short: "Authenticate with the RAIL0 API using SIWE",
}

// ── auth login ────────────────────────────────────────────────────────────────

var loginPrivateKey string

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in with your Ethereum wallet (SIWE)",
	Long: `Authenticates with the RAIL0 API using Sign-In with Ethereum (EIP-4361).

  1. Fetches a single-use nonce from the API
  2. Builds a SIWE message and signs it with your private key
  3. Verifies the signature and stores the JWT token

The token is saved to ~/.cache/rail0/token and used automatically by all
commands that require authentication (e.g. 'payments list').
Override with the RAIL0_TOKEN environment variable.`,
	Example: `  rail0 auth login -p 0xYourPrivateKey…`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		client := newClient()

		privKey, err := rail0.HexToPrivateKey(loginPrivateKey)
		if err != nil {
			return fmt.Errorf("invalid private key: %w", err)
		}

		apiURL := baseURL
		if apiURL == "" {
			apiURL = "https://api.rail0.xyz"
		}
		host := strings.TrimPrefix(strings.TrimPrefix(apiURL, "https://"), "http://")

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
		"Ethereum private key (0x-prefixed hex) to sign the SIWE message")
	_ = authLoginCmd.MarkFlagRequired("private-key")
	authLoginCmd.Flags().StringVar(&loginPrivateKey, "pk", "", "Alias for --private-key")
	_ = authLoginCmd.Flags().MarkHidden("pk")

	authCmd.AddCommand(authLoginCmd, authStatusCmd, authLogoutCmd)
}
