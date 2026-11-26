## rail0 admin onboardings resend

Send a request's link again: the invite, or a reminder for a sign-up (write mode, audited)

### Synopsis

Send the link of a request again, with the same values: the invite again, or a reminder
for a self-service sign-up. The previous link stops working and an expired request is
renewed. It is also how a failed send is retried.

Refused with 409 when an account has taken the request's name, email or address since it
was created: correct the request first ('rail0 admin onboardings correct').

A write: it needs write mode ('rail0 admin write-mode on').

```
rail0 admin onboardings resend <request> [flags]
```

### Examples

```
  rail0 admin onboardings resend 019f0c11-…
```

### Options

```
  -h, --help   help for resend
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

