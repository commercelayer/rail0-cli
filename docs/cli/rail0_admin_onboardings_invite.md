## rail0 admin onboardings invite

Invite a merchant: create the request and email the link (write mode, audited)

### Synopsis

Create an onboarding request for a merchant and email them the link. The three values
are the account's name, the contact email the link goes to, and the merchant's own wallet
address: the claim must be signed by it, so it is the merchant's to give, never one made
for them.

Repeating the invite for an open request with the same email and address sends a new link
for that request (the previous one stops working) instead of creating a second.

A name, email or address an account already holds is refused (409 account_name_taken,
account_email_taken, account_wallet_taken), and so is an email the environment may not
write to (422 email_domain_not_allowed). The answer says which mail was queued and to
whom; 'rail0 admin onboardings' shows later whether it went out (last_send_error).

A write: it needs write mode ('rail0 admin write-mode on').

```
rail0 admin onboardings invite [flags]
```

### Examples

```
  rail0 admin onboardings invite --name "Acme Store" --email ops@acme.example --address 0x2D2B…
```

### Options

```
      --address string   The merchant's wallet address (0x…40 hex, or @alias): the claim must be signed by it
      --email string     Contact email: the link is sent here
  -h, --help             help for invite
      --name string      The merchant account's name (unique among accounts)
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

* [rail0 admin onboardings](rail0_admin_onboardings.md)	 - Onboarding requests: list, invite, correct, resend, cancel (lists when run bare)

