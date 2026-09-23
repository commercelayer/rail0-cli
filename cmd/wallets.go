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

// walletsCmd manages the logged-in account's wallets. The account id is taken
// from the session (JWT), never passed as an argument — run bare to list; the
// verbs below act on a single wallet, named by its id or its 0x address (the
// wallet's own identity; unique per account) — no @alias there, a wallet
// already has its own address. (create --address does accept an @alias: it
// names the address to register.)

var walletsAccount string
var walletsActive string
var walletsChainID int
var walletsSymbol string
var walletsSort string
var walletsDefault string
var walletsTokenActive string
var walletsPage int
var walletsPerPage int

var walletsCmd = &cobra.Command{
	Use:   "wallets",
	Short: "Manage the logged-in account's wallets (lists when run bare)",
	Long: `List and manage the wallets of an account.

All wallet endpoints require a login ('rail0 auth login') — they are scoped to
your own account. When listing, --account defaults to the session account; if
given it must be your own account (the gateway rejects another account's id with
403). To discover ANOTHER merchant's accepted wallets/tokens without a session,
use 'rail0 payment-methods'. The verbs below act on one wallet (id or 0x
address) of the session account.`,
	Example: `  rail0 wallets                                   # session account
  rail0 wallets --account 019e748b-… --chain-id 5042002 --symbol USDC
  rail0 wallets get 0xABCD…
  rail0 wallets create --address 0xABCD…`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		account, err := resolveAccountID(walletsAccount)
		if err != nil {
			return err
		}
		p := rail0.ListWalletsParams{
			ChainID: walletsChainID, TokenSymbol: walletsSymbol, Sort: walletsSort,
			Page: walletsPage, PerPage: walletsPerPage,
		}
		if p.TokenActive, err = parseBoolFlag(walletsTokenActive); err != nil {
			return fmt.Errorf("--token-active: %w", err)
		}
		if walletsActive != "" {
			switch walletsActive {
			case "true":
				t := true
				p.Active = &t
			case "false":
				f := false
				p.Active = &f
			default:
				return errors.New("--active must be 'true' or 'false'")
			}
		}
		if walletsDefault != "" {
			switch walletsDefault {
			case "true":
				t := true
				p.Default = &t
			case "false":
				f := false
				p.Default = &f
			default:
				return errors.New("--default must be 'true' or 'false'")
			}
		}
		return ui.RunAPI(func() (any, error) {
			resp, err := newReadClient().Wallets.List(context.Background(), account, p)
			if err != nil {
				return nil, err
			}
			// Return the full gateway response (each wallet with its nested tokens
			// and account_id); the output layer shows every field.
			return resp.Data, nil
		})
	},
}

// ── wallets get ───────────────────────────────────────────────────────────────

var walletsGetCmd = &cobra.Command{
	Use:     "get <wallet>",
	Short:   "Fetch a single wallet (by id or 0x address)",
	Example: `  rail0 wallets get 0xABCD…`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		account, err := currentAccountID()
		if err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return newReadClient().Wallets.Get(context.Background(), account, args[0])
		})
	},
}

// ── wallets create ──────────────────────────────────────────────────────────

var createWalletAddress string
var createWalletLabel string
var createWalletKey string

var walletsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Register a new EVM wallet address on the account (SIWE proof-of-ownership)",
	Long: `Register a wallet on the logged-in account.

The gateway requires a SIWE proof-of-ownership of the address being added: this
command fetches a single-use nonce, builds an EIP-4361 message for the added
address, signs it, and submits the message + signature. So you must hold the
private key OF THE ADDRESS you are adding, resolved exactly like the signing
commands — pass -p as a raw 0x hex key, as @name for an OS-keychain key
('rail0 keys add'), or omit it to fall back to RAIL0_PRIVATE_KEY (you are
prompted if none is set).

The message is NOT the one 'auth login' signs. The gateway pins each endpoint to
one statement — "Sign in to RAIL0" there, "Add this wallet to your RAIL0 account"
here — and refuses the other with 422 siwe_purpose_mismatch. That separation is
what stops a login signature, which you hand out on every sign-in, from being
replayed to bind an address to someone else's account.

The added address need NOT be your session address — a merchant may register
several payee wallets — but the key must match --address (the command derives
the signer from the key and rejects a mismatch). Addresses are globally unique:
one already registered to any account is refused (409).`,
	Example: `  rail0 wallets create --address 0xABCD… -p @merchant-key --label "Main wallet"
  rail0 wallets create --address @merchant -p 0xYourKey…`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		account, err := currentAccountID()
		if err != nil {
			return err
		}
		// Resolve @alias here: create names the address to register, so pointing
		// at a saved address is meaningful (unlike addressing an existing wallet).
		addr, err := resolveAlias(createWalletAddress)
		if err != nil {
			return err
		}

		// Resolve the key of the address being ADDED (not the session key) the
		// same way every signing command does — flag / @keychain / env / prompt.
		keyHex, err := signingKeyOrPrompt(createWalletKey)
		if err != nil {
			return err
		}
		// The proof only counts if it's signed by the added address's own key, so
		// derive the signer and reject a mismatch locally with a clear message —
		// before the gateway rejects it with a 422 signer_mismatch.
		signer, err := addressFromKey(keyHex)
		if err != nil {
			return err
		}
		if !strings.EqualFold(signer, addr) {
			return fmt.Errorf("the signing key controls %s, but --address is %s — adding a wallet needs the key OF the address being added", signer, addr)
		}
		privKey, err := rail0.HexToPrivateKey(keyHex)
		if err != nil {
			return fmt.Errorf("invalid private key: %w", err)
		}

		client := newClient()
		return ui.RunAPI(func() (any, error) {
			// Same SIWE handshake as login (host-only domain, default chain), but
			// ProveAddress does NOT log in — the request is authorized by the
			// existing merchant session (its bearer token is on `client`), while
			// the proof attests control of the added address.
			proof, err := client.Auth.ProveAddress(context.Background(), privKey, siweHost(resolveBaseURL()))
			if err != nil {
				return nil, err
			}
			req := rail0.CreateWalletRequest{
				Address:   addr,
				Message:   proof.Message,
				Signature: proof.Signature,
				Label:     createWalletLabel,
			}
			return client.Wallets.Create(context.Background(), account, req)
		})
	},
}

// ── wallets update ──────────────────────────────────────────────────────────

var updateWalletLabel string
var updateWalletActive string

var walletsUpdateCmd = &cobra.Command{
	Use:   "update <wallet>",
	Short: "Update label or active status on a wallet (by id or 0x address)",
	Long: `Change a wallet's label, or turn it on and off with --active.

--active false is the merchant's REVOCATION switch, and it does more than hide a
row: a deactivated wallet stops being offered by 'payment-methods', so no new
payment can name it, and webhooks stop being dispatched for payments to it. That
is deliberate — deactivation says "stop trusting this address", and an outbound
push to a revoked wallet's payments would contradict it.

Existing payments are unaffected: they carry their own payee address and settle
whatever the flag says now. Reversible with --active true.

'wallets delete' is the same operation under the API's DELETE verb — a soft
delete, not a removal — so the two are interchangeable.`,
	Example: `  rail0 wallets update 0xABCD… --label "New label" --active true`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		account, err := currentAccountID()
		if err != nil {
			return err
		}
		wallet := args[0]
		req := rail0.UpdateWalletRequest{Label: updateWalletLabel}
		if updateWalletActive != "" {
			switch updateWalletActive {
			case "true":
				t := true
				req.Active = &t
			case "false":
				f := false
				req.Active = &f
			default:
				return errors.New("--active must be 'true' or 'false'")
			}
		}
		// --active false IS the revocation, so it is gated like `wallets delete`, which
		// performs the identical operation. Before this only the delete spelling asked,
		// while the flag spelling — the one the help points at, and the one a script is
		// likelier to reach for — went straight through. (gateway#237)
		if req.Active != nil && !*req.Active {
			if err := confirmDestructive(deactivationPrompt(wallet)); err != nil {
				return err
			}
		}
		return ui.RunAPI(func() (any, error) {
			return newClient().Wallets.Update(context.Background(), account, wallet, req)
		})
	},
}

// ── wallets delete ──────────────────────────────────────────────────────────

var walletsDeleteCmd = &cobra.Command{
	Use:   "delete <wallet>",
	Short: "Deactivate a wallet — soft delete, reversible (by id or 0x address)",
	Long: `Deactivate a wallet. Despite the verb, this is a SOFT delete and it is
reversible: the gateway sets active=false and keeps the row, so 'wallets update
<wallet> --active true' brings it back. Nothing is removed, nothing on-chain is
touched, and no funds move.

It is the same operation as 'wallets update <wallet> --active false'. Both
spellings exist because the API exposes a DELETE verb as well as the flag —
use whichever reads better where you are writing it.

What deactivation actually does: the address stops being offered by
'payment-methods', so buyers can no longer discover it and no new payment can
name it, and webhooks stop being dispatched for payments to it.

Existing payments still SETTLE — they carry their own payee address and the chain
does not care — but you stop being TOLD about the ones still in flight: webhook
dispatch resolves the payee wallet's account, so a deactivated wallet delivers
nothing further and no delivery is recorded as skipped. Capture them, or expect
to poll. (rail0-gateway#237)`,
	Example: `  rail0 wallets delete 0xABCD…`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		account, err := currentAccountID()
		if err != nil {
			return err
		}
		if err := confirmDestructive(deactivationPrompt(args[0])); err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return nil, newClient().Wallets.Delete(context.Background(), account, args[0])
		})
	},
}

// deactivationPrompt is shared by `wallets delete` and `wallets update --active false`,
// which are the same operation on the gateway — so they must ask the same question.
//
// It names the WEBHOOK consequence because that is the one nothing else surfaces. The rest
// of deactivation announces itself: the wallet stops appearing in payment-methods, buyers
// stop being able to name it. But webhook dispatch resolves the payee wallet's account, so
// a deactivated wallet silently stops delivering events for payments that are STILL IN
// FLIGHT — the merchant is not told, no delivery is recorded as skipped, and the payments
// settle on-chain regardless. That asymmetry is gateway#237.
func deactivationPrompt(wallet string) string {
	return fmt.Sprintf(
		"Deactivate wallet %s?\n"+
			"  - buyers can no longer discover it and no new payment can name it\n"+
			"  - webhooks STOP for its payments that are still in flight (they settle, you are not told)\n"+
			"  - reversible: rail0 wallets update %s --active true",
		wallet, wallet)
}

// ── wallets balances ──────────────────────────────────────────────────────────

var balancesChainID int
var balancesSymbol string

var walletsBalancesCmd = &cobra.Command{
	Use:   "balances <wallet>",
	Short: "Read a wallet's live on-chain balances (by id or 0x address)",
	Long: `Read a wallet's live on-chain balances across the configured chains.

Each chain reports the native gas-token balance and the active token balances,
or an error if that chain's RPC was unreachable (reported per chain, so one dead
RPC never hides the others). Use --chain-id / --symbol to narrow the result.`,
	Example: `  rail0 wallets balances 0xABCD… --chain-id 84532 --symbol USDC`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		account, err := currentAccountID()
		if err != nil {
			return err
		}
		p := rail0.WalletBalancesParams{ChainID: balancesChainID, TokenSymbol: balancesSymbol}
		return ui.RunAPI(func() (any, error) {
			return newReadClient().Wallets.Balances(context.Background(), account, args[0], p)
		})
	},
}

// walletsAliasCmd is a convenience alias of `aliases add` with the arguments in
// wallet-first order (<address> <name>), reading naturally after copying an
// address out of `wallets`. It shares addAlias, so the same name-uniqueness
// rule applies (an existing name is rejected — rm it first).
var walletsAliasCmd = &cobra.Command{
	Use:     "alias <address> <name>",
	Short:   "Save a unique address-book alias for an address (like 'aliases add', args reversed)",
	Example: `  rail0 wallets alias 0xAbC123… payee`,
	Args:    cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		addr, name := args[0], args[1]
		if err := addAlias(name, addr); err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return map[string]any{"status": "saved", "alias": name, "address": addr}, nil
		})
	},
}

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	// wallets (list). --account targets a specific account; without it the
	// logged-in session's account is used (one of the two must be present, else
	// error). --active filters the wallets. --chain-id/--symbol restrict the
	// server-side nested tokens shown for each wallet.
	walletsCmd.Flags().StringVar(&walletsAccount, "account", "", "Your account id to list (default: the logged-in session account; must be your own)")
	walletsCmd.Flags().StringVar(&walletsActive, "active", "", "Filter wallets by active flag ('true' or 'false')")
	walletsCmd.Flags().IntVar(&walletsChainID, "chain-id", 0, "Restrict nested tokens to this chain id")
	walletsCmd.Flags().StringVarP(&walletsSymbol, "symbol", "s", "", "Restrict nested tokens to this symbol, e.g. USDC (alias: --token)")
	walletsCmd.Flags().StringVar(&walletsSymbol, "token", "", "Alias for --symbol")
	_ = walletsCmd.Flags().MarkHidden("token")
	walletsCmd.Flags().StringVar(&walletsSort, "sort", "", "Sort wallets (e.g. -created_at)")
	walletsCmd.Flags().StringVar(&walletsDefault, "default", "", "Restrict nested token holdings to the default one ('true' or 'false')")
	walletsCmd.Flags().StringVar(&walletsTokenActive, "token-active", "", "Restrict nested token holdings by active flag ('true' or 'false')")
	// Without these the listing stopped, silently, at the gateway's default page.
	walletsCmd.Flags().IntVar(&walletsPage, "page", 0, "Page number (1-based)")
	walletsCmd.Flags().IntVar(&walletsPerPage, "per-page", 0, "Wallets per page")

	// wallets create
	walletsCreateCmd.Flags().StringVarP(&createWalletAddress, "address", "A", "", "EVM wallet address (0x-prefixed hex, or @alias)")
	_ = walletsCreateCmd.MarkFlagRequired("address")
	walletsCreateCmd.Flags().StringVarP(&createWalletLabel, "label", "l", "", "Human-readable wallet label")
	walletsCreateCmd.Flags().StringVarP(&createWalletKey, "private-key", "p", "",
		"Signing key OF the address being added: 0x hex, @name for an OS-keychain key ('rail0 keys add'), or RAIL0_PRIVATE_KEY")
	walletsCreateCmd.Flags().StringVar(&createWalletKey, "pk", "", "Alias for --private-key")
	_ = walletsCreateCmd.Flags().MarkHidden("pk")

	// wallets update
	walletsUpdateCmd.Flags().StringVarP(&updateWalletLabel, "label", "l", "", "New wallet label")
	walletsUpdateCmd.Flags().StringVar(&updateWalletActive, "active", "", "Set active flag ('true' or 'false')")

	// wallets balances
	walletsBalancesCmd.Flags().IntVar(&balancesChainID, "chain-id", 0, "Restrict to one chain id")
	walletsBalancesCmd.Flags().StringVarP(&balancesSymbol, "symbol", "s", "", "Restrict tokens to this symbol, e.g. USDC (alias: --token)")
	walletsBalancesCmd.Flags().StringVar(&balancesSymbol, "token", "", "Alias for --symbol")
	_ = walletsBalancesCmd.Flags().MarkHidden("token")

	addYesFlag(walletsDeleteCmd)
	// --active false is the same revocation, so it needs the same escape hatch: without
	// --yes the new confirmation would refuse in a non-interactive shell, which would
	// break any script that turns a wallet off.
	addYesFlag(walletsUpdateCmd)
	walletsCmd.AddCommand(
		walletsGetCmd,
		walletsCreateCmd,
		walletsUpdateCmd,
		walletsDeleteCmd,
		walletsBalancesCmd,
		walletsAliasCmd,
	)
}
