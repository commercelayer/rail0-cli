package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// The operator role: who holds it, granting it, revoking it. The role is what opens every
// command of this group, so its two writes are the most consequential here: both need
// write mode, both ask first naming the account, and the gateway audits both. Like the
// rest of the group they never run in agent mode.

var (
	operatorGrantNote    string
	operatorGrantExpires string
)

var adminOperatorsCmd = &cobra.Command{
	Use:   "operators",
	Short: "Who holds the operator role; grant it, revoke it (lists when run bare)",
	Long: `List the operator grants, newest first: the account that holds each, who granted it
(empty for a grant made out of band, by the seeds or the gateway's own maintenance task),
the note it was given with, and when it expires.

Expired grants are listed too, as history. A grant opens the operator surface while its
expiry is empty or still ahead and its account is active.`,
	Example: `  rail0 admin operators
  rail0 admin operators --pretty
  rail0 admin operators grant 019e748b-… --note "on call this week" --expires 7d
  rail0 admin operators revoke 019e748b-…`,
	Args: cobra.NoArgs,
	RunE: operatorOnly(func(_ *cobra.Command, _ []string) error {
		return ui.RunAPI(func() (any, error) {
			grants, err := newReadClient().Admin.Operators(context.Background())
			if err != nil {
				return nil, err
			}
			return table(grants, operatorGrantColumns), nil
		})
	}),
}

var adminOperatorsGrantCmd = &cobra.Command{
	Use:   "grant <account>",
	Short: "Give an account the operator role (write mode, audited)",
	Long: `Give an account the operator role. From its next request that account reads every
account, payment and webhook on this gateway, and with write mode it switches accounts and
wallets off, invites merchants and grants this same role. Give it to a person, for a
reason.

--note says why; it is kept with the grant and in the audit log. --expires time-boxes
the grant: a duration from now ('7d', '12h') or an ISO-8601 time; past it the grant
opens nothing, and stays listed as history. Without --expires the grant lasts until it is
revoked.

The command reads the account and asks, naming it. Your own account is recorded as the
one who granted it. An account that already holds a grant, expired or not, is refused
(409): revoke it first to change its note or its expiry.

A write: it needs write mode ('rail0 admin write-mode on').`,
	Example: `  rail0 admin operators grant 019e748b-… --note "on call this week" --expires 7d
  rail0 admin operators grant 019e748b-… --note "platform team"
  rail0 admin operators grant 019e748b-… --expires 2026-12-31T23:59:59Z --yes`,
	Args: cobra.ExactArgs(1),
	RunE: operatorOnly(func(_ *cobra.Command, args []string) error {
		expires, err := parseGrantExpiry(operatorGrantExpires, time.Now())
		if err != nil {
			return err
		}
		if err := requireWriteMode(); err != nil {
			return err
		}
		ctx := context.Background()
		account, err := newReadClient().Admin.Account(ctx, args[0])
		if err != nil {
			return err
		}
		if err := confirmDestructive(grantOperatorPrompt(account, expires)); err != nil {
			return err
		}
		return ui.RunAPIWithMessage("Granting…", func() (any, error) {
			return newClient().Admin.GrantOperator(ctx, account.ID, rail0.GrantOperatorRequest{
				Note: strings.TrimSpace(operatorGrantNote), ExpiresAt: expires,
			})
		})
	}),
}

var adminOperatorsRevokeCmd = &cobra.Command{
	Use:   "revoke <account>",
	Short: "Remove an account's operator role (write mode, audited)",
	Long: `Remove the operator role from an account: from its next request it is a merchant
account like any other, and a write-mode token it holds stops working with it. The
argument is the ACCOUNT's id, as 'rail0 admin operators' lists it. The grant is deleted;
the audit log keeps who granted it and who revoked it.

The gateway refuses two cases. Your own grant: stepping down is another operator's act,
so the role cannot be dropped by a slip, nor by a stolen session covering its tracks. And
the last active operator: someone must be able to grant the role again.

The command reads the account and asks, naming it.

A write: it needs write mode ('rail0 admin write-mode on').`,
	Example: `  rail0 admin operators revoke 019e748b-…
  rail0 admin operators revoke 019e748b-… --yes`,
	Args: cobra.ExactArgs(1),
	RunE: operatorOnly(func(_ *cobra.Command, args []string) error {
		if err := requireWriteMode(); err != nil {
			return err
		}
		ctx := context.Background()
		account, err := newReadClient().Admin.Account(ctx, args[0])
		if err != nil {
			return err
		}
		prompt := fmt.Sprintf("Revoke the operator role of account %s?\n"+
			"  - it stops reading and writing across accounts at its next request\n"+
			"  - it keeps its own merchant account; the role can be granted again", accountLabel(account))
		if err := confirmDestructive(prompt); err != nil {
			return err
		}
		return ui.RunAPIWithMessage("Revoking…", func() (any, error) {
			if err := newClient().Admin.RevokeOperator(ctx, account.ID); err != nil {
				return nil, err
			}
			return map[string]any{"account_id": account.ID, "revoked": true}, nil
		})
	}),
}

// operatorGrantConflictHint is what a 409 on 'operators grant' means: the unique index on
// the grant answered, and its body cannot say so.
const operatorGrantConflictHint = "the account already holds an operator grant (an expired one counts): " +
	"'rail0 admin operators revoke' it first to change its note or its expiry"

// parseGrantExpiry reads --expires: empty (no expiry), a duration from now in days or
// hours ('7d', '12h', '1h30m'), or an ISO-8601 time. It returns the instant in UTC, and
// refuses one that is not ahead of now: the gateway would, and a grant born expired is a
// typo, not a decision.
func parseGrantExpiry(s string, now time.Time) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	var at time.Time
	if days, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil || n <= 0 {
			return "", usagef("--expires %q is not a whole number of days above zero", s)
		}
		at = now.Add(time.Duration(n) * 24 * time.Hour)
	} else if d, err := time.ParseDuration(s); err == nil {
		at = now.Add(d)
	} else if t, err := time.Parse(time.RFC3339, s); err == nil {
		at = t
	} else {
		return "", usagef("--expires %q is neither a duration (7d, 12h) nor an ISO-8601 time (2026-12-31T23:59:59Z)", s)
	}
	if !at.After(now) {
		return "", usagef("--expires %q is not in the future", s)
	}
	return at.UTC().Format(time.RFC3339), nil
}

// grantOperatorPrompt is the question asked before the role is given: which account, what
// the role opens, and until when.
func grantOperatorPrompt(a *rail0.AccountFull, expires string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Grant the operator role to account %s?\n", accountLabel(a))
	if a.Email != "" {
		fmt.Fprintf(&b, "  - contact: %s\n", a.Email)
	}
	if !a.Active {
		b.WriteString("  - the account is DEACTIVATED: the grant opens nothing until it is activated\n")
	}
	b.WriteString("  - it will read every account, payment and webhook on this gateway\n" +
		"  - with write mode it will switch accounts and wallets off, invite merchants and grant this role\n")
	if expires == "" {
		b.WriteString("  - no expiry: it lasts until 'rail0 admin operators revoke'")
	} else {
		fmt.Fprintf(&b, "  - until %s, then it opens nothing", ui.ShortTime(expires))
	}
	return b.String()
}

func init() {
	adminOperatorsGrantCmd.Flags().StringVar(&operatorGrantNote, "note", "", "Why the grant exists (kept with the grant and in the audit log)")
	adminOperatorsGrantCmd.Flags().StringVar(&operatorGrantExpires, "expires", "",
		"Time-box the grant: a duration from now (7d, 12h) or an ISO-8601 time; default: no expiry")
	addYesFlag(adminOperatorsGrantCmd)
	addYesFlag(adminOperatorsRevokeCmd)
}
