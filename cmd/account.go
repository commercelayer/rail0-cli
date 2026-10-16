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

// accountCmd reads and edits the account PROFILE — the account row itself (name,
// email, priority-tip fee setting), as opposed to `wallets`, which manages the
// addresses under it. Like
// `wallets`, the account id comes from the session (the JWT's account_id claim, the
// one `auth status` prints); --account names another one, which the gateway answers
// only for an admin (anyone else gets 403 not_your_account).
//
// The account's `active` flag is deliberately NOT exposed: it is the operator's
// field (the gateway refuses it from an owner with 403), and reactivating or parking
// an account belongs on the operator surface, not in a merchant's CLI.

var accountID string
var accountUpdateName string
var accountUpdateEmail string
var accountUpdatePriorityTip bool

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Show or update the signed-in account's profile (shows when run bare)",
	Long: `Read and update the profile of the signed-in account: its id, name, email,
priority-tip setting and timestamps.

Requires a login ('rail0 auth login') by a wallet that belongs to an account — a
wallet-only session has no account to show. The account is the session's own
('rail0 auth status' prints its id); --account reads or updates another one, which
the gateway allows only for an admin (anyone else gets 403 not_your_account).

Run bare, or as 'account get', to show it; 'account update' changes the name,
email and/or priority tip. The account's active flag is operator-only and is not
settable here.`,
	Example: `  rail0 account                          # the signed-in account
  rail0 account get --account 019e748b-… # another account (admin only)
  rail0 account update --email ops@example.com
  rail0 account update --priority-tip=false`,
	Args: cobra.NoArgs,
	RunE: runAccountGet,
}

// ── account get ───────────────────────────────────────────────────────────────

var accountGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show the account's profile (the same as 'rail0 account')",
	Example: `  rail0 account get
  rail0 account get --account 019e748b-…`,
	Args: cobra.NoArgs,
	RunE: runAccountGet,
}

// runAccountGet is shared by the bare `account` and `account get`, which are the
// same read under two spellings.
func runAccountGet(_ *cobra.Command, _ []string) error {
	account, err := resolveAccountID(accountID)
	if err != nil {
		return err
	}
	return ui.RunAPI(func() (any, error) {
		return newReadClient().Accounts.Get(context.Background(), account)
	})
}

// ── account update ────────────────────────────────────────────────────────────

var accountUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Change the account's name, email and/or priority tip",
	Long: `Change the account's display name, contact email and/or priority-tip
setting. At least one of --name / --email / --priority-tip is required.

Name and email are unique across accounts: a value another account already holds
is refused with 409. A deactivated account, or a session signed in with a
deactivated wallet, cannot change them (403) — a revoked key must not be able to
redirect the contact email, which is where the merchant's operational
notifications go. The email is the owner's to change: an admin updating another
account with --account may change its name and priority tip, but --email answers
403 forbidden (before the account exists, it is corrected on its onboarding
request in the console).

--priority-tip is the fee setting of the account's OWN transactions (authorize,
capture, void, release, refund). On, the default, they tip the network's suggested
priority fee; '--priority-tip=false' uses the chain's minimum tip instead (0 on
most chains, 25 gwei on Polygon PoS/Amoy) — cheaper, slower to confirm under load.
The buyer's dispute and close-dispute never read it.

No confirmation is asked: the change is not destructive and 'account update'
reverses it.`,
	Example: `  rail0 account update --name "Acme Store"
  rail0 account update --name "Acme Store" --email ops@acme.example
  rail0 account update --priority-tip=false   # minimum tip on the account's transactions
  rail0 account update --priority-tip         # back to the suggested priority fee`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		req, err := accountUpdateRequest(cmd)
		if err != nil {
			return err
		}
		account, err := resolveAccountID(accountID)
		if err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			return newClient().Accounts.Update(context.Background(), account, req)
		})
	},
}

// accountUpdateRequest builds the PATCH body from the flags, refusing locally what
// the gateway would refuse with a 400: nothing to change, or a flag passed with a
// blank value. A blank value matters more than it looks — the SDK omits empty
// fields, so `--name ""` alone would otherwise go out as an empty PATCH, and
// `--name "" --email x` would silently drop the name the user thought they set.
func accountUpdateRequest(cmd *cobra.Command) (rail0.UpdateAccountRequest, error) {
	var req rail0.UpdateAccountRequest
	for _, f := range []struct {
		name string
		val  string
		dst  *string
	}{
		{"name", accountUpdateName, &req.Name},
		{"email", accountUpdateEmail, &req.Email},
	} {
		if !cmd.Flags().Changed(f.name) {
			continue
		}
		v := strings.TrimSpace(f.val)
		if v == "" {
			return req, fmt.Errorf("--%s cannot be empty", f.name)
		}
		*f.dst = v
	}
	// A bool flag has no blank to refuse, but it has the same ambiguity the SDK's
	// omitempty would create: false must reach the wire. Changed, not the value, is
	// what says the user asked for it.
	if cmd.Flags().Changed("priority-tip") {
		v := accountUpdatePriorityTip
		req.PriorityTip = &v
	}
	if req.Name == "" && req.Email == "" && req.PriorityTip == nil {
		return req, errors.New("nothing to update: pass --name, --email and/or --priority-tip")
	}
	return req, nil
}

// accountConflictHint is what a 409 on `account update` means. The gateway's
// conflict body is generic ("A resource with those identifying values already
// exists") because it comes from the unique index, not the route — so it cannot say
// which field collided, but the command can say which fields could have.
const accountConflictHint = "that name or email is already used by another account — choose a different one"

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	// Persistent, so it reads the same on the bare command and on both verbs.
	accountCmd.PersistentFlags().StringVar(&accountID, "account", "", "Account id (default: the logged-in session account; another account's needs an admin session)")

	accountUpdateCmd.Flags().StringVar(&accountUpdateName, "name", "", "New display name (unique across accounts)")
	accountUpdateCmd.Flags().StringVar(&accountUpdateEmail, "email", "", "New contact email (unique across accounts)")
	accountUpdateCmd.Flags().BoolVar(&accountUpdatePriorityTip, "priority-tip", true,
		"Tip the network's suggested priority fee on the account's own transactions; --priority-tip=false uses the chain's minimum tip")

	accountCmd.AddCommand(accountGetCmd, accountUpdateCmd)
}
