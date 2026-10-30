package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// walletTokensCmd manages WHAT A WALLET ACCEPTS: the (chain, token) holdings that
// drive the public payment-methods discovery and gate payment creation. A wallet
// with no active holding is invisible to buyers and unusable as a payee, so without
// these a merchant could register a wallet from the CLI and then had to leave it to
// make the wallet usable.
var walletTokensCmd = &cobra.Command{
	Use:   "tokens",
	Short: "Choose which tokens (and chains) a wallet accepts",
	Long: `Manage a wallet's token holdings — the (chain, token) pairs it accepts.

They are what 'payment-methods' offers buyers and what payment creation checks:
a wallet with no active holding cannot be paid. Every holding survives being
disabled, so re-enabling one keeps its history.

A wallet with active holdings always has exactly ONE default, the method a buyer
is offered first. The first holding you add takes it; to move it, run
'wallets tokens add' on the new one with --default. While other holdings are
active the gateway refuses to disable the default (422 default_payment_method) —
move it first. The LAST active holding can be disabled even as the default: the
wallet then accepts nothing, and the next holding enabled becomes the default.

These commands act only on the wallet you signed in with: another wallet of the
account is refused (403 not_your_wallet). 'rail0 auth login' with that wallet's
key to manage its tokens.`,
	Example: `  rail0 wallets tokens add 0xABCD… --chain base --token USDC --default
  rail0 wallets tokens disable 0xABCD… USDC --chain base
  rail0 wallets tokens enable 0xABCD… USDC --chain base`,
}

var (
	walletTokenChainID int
	walletTokenToken   string
	walletTokenDefault bool
)

var walletTokensAddCmd = &cobra.Command{
	Use:   "add <wallet>",
	Short: "Accept a token on a wallet (re-enables a disabled holding)",
	Long: `Accept a token, on one chain, on a wallet (by id, 0x address or @alias).

An upsert: a holding that was disabled is re-enabled rather than duplicated.
--token is a symbol (USDC) resolved on --chain, or the token's 0x address.
--default makes it the wallet's default method, demoting the previous one;
--chain falls back to the configured default ('rail0 config set chain').`,
	Example: `  rail0 wallets tokens add 0xABCD… --chain base --token USDC
  rail0 wallets tokens add 0xABCD… --chain base --token 0x036C… --default`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		account, err := currentAccountID()
		if err != nil {
			return err
		}
		chainID, err := walletTokenChain()
		if err != nil {
			return err
		}
		if walletTokenToken == "" {
			walletTokenToken = configuredDefault("token")
		}
		if walletTokenToken == "" {
			return errors.New("--token is required (a symbol such as USDC, or the token's 0x address)")
		}
		ctx := context.Background()
		address := walletTokenToken
		if !strings.HasPrefix(address, "0x") {
			if address, _, _, err = lookupTokenAddress(ctx, chainID, walletTokenToken); err != nil {
				return err
			}
		}
		req := rail0.AddWalletTokenRequest{ChainID: chainID, Token: address}
		// Tri-state: an omitted --default keeps the holding's current flag, which is
		// a different request from --default=false.
		if cmd.Flags().Changed("default") {
			req.Default = &walletTokenDefault
		}
		wallet, err := resolveAlias(args[0])
		if err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return newClient().Wallets.AddToken(ctx, account, wallet, req)
		})
	},
}

var walletTokensEnableCmd = &cobra.Command{
	Use:   "enable <wallet> <token>",
	Short: "Re-enable a holding the wallet already has",
	Long: `Re-enable an existing, disabled holding. <token> is the token's symbol or 0x
address (add --chain when the wallet holds that symbol on several chains), or
the token id shown as token_id by 'rail0 wallets'. A token the wallet has never
held is not enabled here — use 'wallets tokens add'.`,
	Example: `  rail0 wallets tokens enable 0xABCD… USDC --chain base`,
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWalletTokenAction(args[0], args[1], func(c *rail0.Client, account, wallet, tokenID string) (any, error) {
			return c.Wallets.EnableToken(context.Background(), account, wallet, tokenID)
		})
	},
}

var walletTokensDisableCmd = &cobra.Command{
	Use:     "disable <wallet> <token>",
	Aliases: []string{"remove"},
	Short:   "Stop accepting a token on a wallet (soft, reversible)",
	Long: `Stop accepting a token on a wallet. The holding is kept, disabled, so
'wallets tokens enable' brings it back. <token> is resolved as for 'enable'.

Buyers stop being offered this method at once and no new payment can use it;
payments already created with it are unaffected. The wallet's DEFAULT method
cannot be disabled — make another one the default first ('wallets tokens add …
--default').`,
	Example: `  rail0 wallets tokens disable 0xABCD… USDC --chain base`,
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := confirmDestructive(fmt.Sprintf(
			"Stop accepting %s on wallet %s? Buyers will no longer be offered it.", args[1], args[0])); err != nil {
			return err
		}
		return runWalletTokenAction(args[0], args[1], func(c *rail0.Client, account, wallet, tokenID string) (any, error) {
			return c.Wallets.DisableToken(context.Background(), account, wallet, tokenID)
		})
	},
}

// walletTokenChain is --chain, falling back to the configured default. 0 when
// neither is set — an error for `add`, "any chain" for the holding lookup.
func walletTokenChain() (int, error) {
	if walletTokenChainID != 0 {
		return walletTokenChainID, nil
	}
	id, _, ok, err := configuredChainID(context.Background())
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, errChainRequired()
	}
	return id, nil
}

// runWalletTokenAction resolves <token> to the holding's token id and runs the
// enable/disable call against it.
func runWalletTokenAction(wallet, tokenArg string, call func(c *rail0.Client, account, wallet, tokenID string) (any, error)) error {
	account, err := currentAccountID()
	if err != nil {
		return err
	}
	if wallet, err = resolveAlias(wallet); err != nil {
		return err
	}
	chainID := walletTokenChainID // explicit only: the lookup narrows by it, no default needed
	client := newClient()
	return ui.RunAPI(func() (any, error) {
		tokenID, err := resolveHoldingTokenID(context.Background(), client, account, wallet, tokenArg, chainID)
		if err != nil {
			return nil, err
		}
		return call(client, account, wallet, tokenID)
	})
}

// resolveHoldingTokenID finds the token id of one of a wallet's holdings.
//
// The gateway addresses a holding by the TOKEN's UUID, which nobody types: a
// merchant knows "USDC on Base Sepolia". So <token> may be that UUID, the token's
// 0x address or its symbol, matched against the holdings the wallet ACTUALLY has —
// read from the wallets listing, the one endpoint that returns holdings with their
// token_id. chainID narrows the match (0 = any chain); a symbol held on several
// chains without it is an error naming them, never a guess.
//
// The wallet itself is matched by id or address, case-insensitively, walking every
// page of the listing so an account with many wallets is not cut off at the first.
func resolveHoldingTokenID(ctx context.Context, client *rail0.Client, account, wallet, tokenArg string, chainID int) (string, error) {
	p := rail0.ListWalletsParams{ChainID: chainID, PerPage: 100}
	for page := 1; ; page++ {
		p.Page = page
		resp, err := client.Wallets.List(ctx, account, p)
		if err != nil {
			return "", err
		}
		for _, w := range resp.Data {
			if w.ID != wallet && !strings.EqualFold(w.Address, wallet) {
				continue
			}
			return matchHolding(w, tokenArg)
		}
		if resp.Meta.Links.Next == "" {
			return "", fmt.Errorf("wallet %s not found on this account — see 'rail0 wallets'", wallet)
		}
	}
}

// matchHolding picks the holding <token> names on one wallet. See
// resolveHoldingTokenID.
func matchHolding(w rail0.WalletWithTokens, tokenArg string) (string, error) {
	var matches []rail0.WalletTokenHolding
	for _, h := range w.Tokens {
		if h.TokenID == tokenArg ||
			strings.EqualFold(string(h.Token.Address), tokenArg) ||
			strings.EqualFold(h.Token.Symbol, tokenArg) {
			matches = append(matches, h)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("wallet %s holds no token %q — 'wallets tokens add' accepts a new one", w.Address, tokenArg)
	case 1:
		return matches[0].TokenID, nil
	}
	chains := make([]string, 0, len(matches))
	for _, h := range matches {
		chains = append(chains, chainLabel(h.Token.ChainID))
	}
	return "", fmt.Errorf("wallet %s holds %s on several chains (%s) — pass --chain",
		w.Address, tokenArg, strings.Join(chains, ", "))
}

func init() {
	addChainFlag(walletTokensAddCmd, &walletTokenChainID, "", "Chain of the token (default: the configured chain)")
	walletTokensAddCmd.Flags().StringVarP(&walletTokenToken, "token", "t", "", "Token symbol (e.g. USDC) or 0x address (default: the configured token)")
	walletTokensAddCmd.Flags().BoolVar(&walletTokenDefault, "default", false, "Make it the wallet's default payment method")
	for _, c := range []*cobra.Command{walletTokensEnableCmd, walletTokensDisableCmd} {
		addChainFlag(c, &walletTokenChainID, "", "Narrow <token> to one chain when the wallet holds it on several")
	}
	addYesFlag(walletTokensDisableCmd)
	walletTokensCmd.AddCommand(walletTokensAddCmd, walletTokensEnableCmd, walletTokensDisableCmd)
	walletsCmd.AddCommand(walletTokensCmd)
}
