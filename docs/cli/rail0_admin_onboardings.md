## rail0 admin onboardings

Onboarding requests: list, invite, correct, resend, cancel (lists when run bare)

### Synopsis

List the onboarding requests: the invites an operator sent and the sign-ups merchants
made themselves, each with its state, how its last email went, and its clashes with
accounts or with other requests (conflicts).

Open and expired requests are listed by default: the ones still waiting for something.
--status completed shows the history; a completed request's id is its account's id.

A request becomes an account only when the merchant claims it, by signing with the
wallet it names, from the link in the email. The link is never shown here.

```
rail0 admin onboardings [flags]
```

### Examples

```
  rail0 admin onboardings
  rail0 admin onboardings --send-failed true        # whose email did not go out
  rail0 admin onboardings --status completed --origin signup --pretty
  rail0 admin onboardings invite --name "Acme Store" --email ops@acme.example --address 0x2D2B…
```

### Options

```
  -h, --help                 help for onboardings
      --origin string        Only requests from this entrance: operator (an invite) or signup (self-service)
      --page int             Page number (1-based)
      --per-page int         Results per page
      --send-failed string   Only requests whose last email failed ('true') or did not ('false')
      --sort string          Sort by created_at, expires_at, last_sent_at, name or email (prefix - for descending; default: newest first)
      --status strings       State(s), comma-separated or repeated: open, expired, completed (default: open,expired)
```

### Options inherited from parent commands

```
      --agent             Agent mode: one JSON envelope on stdout, JSON events on stderr, no prompts (env: RAIL0_AGENT=1)
      --base-url string   RAIL0 gateway base URL (env: RAIL0_BASE_URL)
      --debug             Print HTTP request/response details to stderr
      --json              Force plain (uncolored) JSON output (useful in scripts)
      --pretty            Use the key-value/table view instead of JSON (terminal only)
  -q, --quiet             Print only the result's id/hash, one per line (useful in scripts)
```

### SEE ALSO

* [rail0 admin](rail0_admin.md)	 - Operator commands: diagnostics, stuck transactions, sync errors, accounts, wallets, onboarding, write mode
* [rail0 admin onboardings cancel](rail0_admin_onboardings_cancel.md)	 - Delete a request that is not completed: its link stops working (write mode, audited)
* [rail0 admin onboardings correct](rail0_admin_onboardings_correct.md)	 - Correct a request's name, email or address and send a new link (write mode, audited)
* [rail0 admin onboardings invite](rail0_admin_onboardings_invite.md)	 - Invite a merchant: create the request and email the link (write mode, audited)
* [rail0 admin onboardings resend](rail0_admin_onboardings_resend.md)	 - Send a request's link again: the invite, or a reminder for a sign-up (write mode, audited)

