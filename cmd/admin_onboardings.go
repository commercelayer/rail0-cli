package cmd

import (
	"context"
	"fmt"
	"slices"
	"strings"

	rail0 "github.com/commercelayer/rail0-go"
	"github.com/rail0/cli/internal/ui"
	"github.com/spf13/cobra"
)

// Onboarding requests: how a merchant's account comes to exist. The operator invites (or
// the merchant signs up), the gateway emails a link, and the merchant claims the request
// by signing with the wallet it names: only then is there an account. No answer of these
// commands carries the link, so nothing here can hand an operator a credential for
// someone else's account.

var (
	onbStatuses   []string
	onbOrigin     string
	onbSendFailed string
	onbSort       string
	onbPage       int
	onbPerPage    int
	onbName       string
	onbEmail      string
	onbAddress    string
)

var (
	onboardingStatuses = []string{"open", "expired", "completed"}
	onboardingOrigins  = []string{"operator", "signup"}
)

var adminOnboardingsCmd = &cobra.Command{
	Use:   "onboardings",
	Short: "Onboarding requests: list, invite, correct, resend, cancel (lists when run bare)",
	Long: `List the onboarding requests: the invites an operator sent and the sign-ups merchants
made themselves, each with its state, how its last email went, and its clashes with
accounts or with other requests (conflicts).

Open and expired requests are listed by default: the ones still waiting for something.
--status completed shows the history; a completed request's id is its account's id.

A request becomes an account only when the merchant claims it, by signing with the
wallet it names, from the link in the email. The link is never shown here.`,
	Example: `  rail0 admin onboardings
  rail0 admin onboardings --send-failed true        # whose email did not go out
  rail0 admin onboardings --status completed --origin signup --pretty
  rail0 admin onboardings invite --name "Acme Store" --email ops@acme.example --address 0x2D2B…`,
	Args: cobra.NoArgs,
	RunE: operatorOnly(func(_ *cobra.Command, _ []string) error {
		statuses, err := statusFilterValues("--status", onbStatuses, onboardingStatuses)
		if err != nil {
			return err
		}
		if onbOrigin != "" && !slices.Contains(onboardingOrigins, onbOrigin) {
			return usagef("--origin must be operator or signup (got %q)", onbOrigin)
		}
		sendFailed, err := trueOrFalse("--send-failed", onbSendFailed)
		if err != nil {
			return err
		}
		return ui.RunAPI(func() (any, error) {
			resp, err := newReadClient().Admin.Onboardings(context.Background(), rail0.ListOnboardingsParams{
				Statuses: statuses, Origin: onbOrigin, SendFailed: sendFailed,
				Sort: onbSort, Page: onbPage, PerPage: onbPerPage,
			})
			if err != nil {
				return nil, err
			}
			return paged(table(resp.Data, onboardingColumns), resp.Meta), nil
		})
	}),
}

// onboardingValues reads --name, --email and --address: trimmed, and refused when a flag
// was passed blank or the address is not one. A flag left out stays empty.
func onboardingValues(cmd *cobra.Command) (name, email, address string, err error) {
	for _, f := range []struct {
		flag string
		val  string
		dst  *string
	}{{"name", onbName, &name}, {"email", onbEmail, &email}, {"address", onbAddress, &address}} {
		if !cmd.Flags().Changed(f.flag) {
			continue
		}
		v := strings.TrimSpace(f.val)
		if v == "" {
			return "", "", "", usagef("--%s cannot be empty", f.flag)
		}
		*f.dst = v
	}
	if address != "" {
		if address, err = resolveAlias(address); err != nil {
			return "", "", "", err
		}
		if !evmAddressPattern.MatchString(address) {
			return "", "", "", usagef("--address %q is not a wallet address (0x followed by 40 hex characters)", address)
		}
	}
	return name, email, address, nil
}

var adminOnboardingsInviteCmd = &cobra.Command{
	Use:   "invite",
	Short: "Invite a merchant: create the request and email the link (write mode, audited)",
	Long: `Create an onboarding request for a merchant and email them the link. The three values
are the account's name, the contact email the link goes to, and the merchant's own wallet
address: the claim must be signed by it, so it is the merchant's to give, never one made
for them.

Repeating the invite for an open request with the same email and address sends a new link
for that request (the previous one stops working) instead of creating a second.

A name, email or address an account already holds is refused (409 account_name_taken,
account_email_taken, account_wallet_taken), and so is an email the environment may not
write to (422 email_domain_not_allowed). The answer says which mail was queued and to
whom; 'rail0 admin onboardings' shows later whether it went out (last_send_error).

A write: it needs write mode ('rail0 admin write-mode on').`,
	Example: `  rail0 admin onboardings invite --name "Acme Store" --email ops@acme.example --address 0x2D2B…`,
	Args:    cobra.NoArgs,
	RunE: operatorOnly(func(cmd *cobra.Command, _ []string) error {
		name, email, address, err := onboardingValues(cmd)
		if err != nil {
			return err
		}
		if name == "" || email == "" || address == "" {
			return usagef("--name, --email and --address are all required")
		}
		if err := requireWriteMode(); err != nil {
			return err
		}
		return ui.RunAPIWithMessage("Inviting…", func() (any, error) {
			return newClient().Admin.Invite(context.Background(), rail0.InviteRequest{Name: name, Email: email, Address: address})
		})
	}),
}

var adminOnboardingsCorrectCmd = &cobra.Command{
	Use:   "correct <request>",
	Short: "Correct a request's name, email or address and send a new link (write mode, audited)",
	Long: `Replace the name, the email and/or the wallet address of a request that is not
completed, and send a new link to its email, saying what changed. The previous link stops
working, nothing goes to the address that was replaced, and an expired request is
renewed. At least one of --name / --email / --address, with a value that differs.

The same checks as an invite run on the result. A completed request is history (422
onboarding_completed): its account is corrected with 'rail0 account update --account'.

A write: it needs write mode ('rail0 admin write-mode on').`,
	Example: `  rail0 admin onboardings correct 019f0c11-… --email right@acme.example
  rail0 admin onboardings correct 019f0c11-… --address 0x9F3A… --name "Acme Store Ltd"`,
	Args: cobra.ExactArgs(1),
	RunE: operatorOnly(func(cmd *cobra.Command, args []string) error {
		name, email, address, err := onboardingValues(cmd)
		if err != nil {
			return err
		}
		if name == "" && email == "" && address == "" {
			return usagef("nothing to correct: pass --name, --email and/or --address")
		}
		if err := requireWriteMode(); err != nil {
			return err
		}
		return ui.RunAPIWithMessage("Correcting…", func() (any, error) {
			return newClient().Admin.CorrectOnboarding(context.Background(), args[0],
				rail0.CorrectOnboardingRequest{Name: name, Email: email, Address: address})
		})
	}),
}

var adminOnboardingsResendCmd = &cobra.Command{
	Use:   "resend <request>",
	Short: "Send a request's link again: the invite, or a reminder for a sign-up (write mode, audited)",
	Long: `Send the link of a request again, with the same values: the invite again, or a reminder
for a self-service sign-up. The previous link stops working and an expired request is
renewed. It is also how a failed send is retried.

Refused with 409 when an account has taken the request's name, email or address since it
was created: correct the request first ('rail0 admin onboardings correct').

A write: it needs write mode ('rail0 admin write-mode on').`,
	Example: `  rail0 admin onboardings resend 019f0c11-…`,
	Args:    cobra.ExactArgs(1),
	RunE: operatorOnly(func(_ *cobra.Command, args []string) error {
		if err := requireWriteMode(); err != nil {
			return err
		}
		return ui.RunAPIWithMessage("Sending…", func() (any, error) {
			return newClient().Admin.ResendOnboarding(context.Background(), args[0])
		})
	}),
}

var adminOnboardingsCancelCmd = &cobra.Command{
	Use:   "cancel <request>",
	Short: "Delete a request that is not completed: its link stops working (write mode, audited)",
	Long: `Delete an onboarding request that is not completed: a mistaken invite, a sign-up that
should not go on. Its link stops working at once. It cannot be undone, but the merchant
can be invited again.

A completed request is history and is refused (422 onboarding_completed): to stop that
merchant, 'rail0 admin accounts deactivate' the account, whose id is the request's.

A write: it needs write mode ('rail0 admin write-mode on').`,
	Example: `  rail0 admin onboardings cancel 019f0c11-…
  rail0 admin onboardings cancel 019f0c11-… --yes`,
	Args: cobra.ExactArgs(1),
	RunE: operatorOnly(func(_ *cobra.Command, args []string) error {
		if err := requireWriteMode(); err != nil {
			return err
		}
		prompt := fmt.Sprintf("Cancel the onboarding request %s? Its link stops working. This cannot be undone.", args[0])
		if err := confirmDestructive(prompt); err != nil {
			return err
		}
		return ui.RunAPIWithMessage("Cancelling…", func() (any, error) {
			if err := newClient().Admin.CancelOnboarding(context.Background(), args[0]); err != nil {
				return nil, err
			}
			return map[string]any{"id": args[0], "cancelled": true}, nil
		})
	}),
}

// addOnboardingValueFlags registers the three values an invite takes and a correction replaces.
func addOnboardingValueFlags(cmd *cobra.Command, name, email, address string) {
	f := cmd.Flags()
	f.StringVar(&onbName, "name", "", name)
	f.StringVar(&onbEmail, "email", "", email)
	f.StringVar(&onbAddress, "address", "", address)
}

func init() {
	f := adminOnboardingsCmd.Flags()
	f.StringSliceVar(&onbStatuses, "status", nil, "State(s), comma-separated or repeated: open, expired, completed (default: open,expired)")
	f.StringVar(&onbOrigin, "origin", "", "Only requests from this entrance: operator (an invite) or signup (self-service)")
	f.StringVar(&onbSendFailed, "send-failed", "", "Only requests whose last email failed ('true') or did not ('false')")
	f.StringVar(&onbSort, "sort", "", "Sort by created_at, expires_at, last_sent_at, name or email (prefix - for descending; default: newest first)")
	f.IntVar(&onbPage, "page", 0, "Page number (1-based)")
	f.IntVar(&onbPerPage, "per-page", 0, "Results per page")
	_ = adminOnboardingsCmd.RegisterFlagCompletionFunc("status", fixedListCompletions(onboardingStatuses...))
	_ = adminOnboardingsCmd.RegisterFlagCompletionFunc("origin", fixedCompletions(onboardingOrigins...))
	_ = adminOnboardingsCmd.RegisterFlagCompletionFunc("send-failed", fixedCompletions("true", "false"))

	addOnboardingValueFlags(adminOnboardingsInviteCmd,
		"The merchant account's name (unique among accounts)",
		"Contact email: the link is sent here",
		"The merchant's wallet address (0x…40 hex, or @alias): the claim must be signed by it")
	addOnboardingValueFlags(adminOnboardingsCorrectCmd,
		"The corrected account name", "The corrected contact email", "The corrected wallet address (0x…40 hex, or @alias)")
	addYesFlag(adminOnboardingsCancelCmd)
}
