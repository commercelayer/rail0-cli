## rail0 admin onboardings cancel

Delete a request that is not completed: its link stops working (write mode, audited)

### Synopsis

Delete an onboarding request that is not completed: a mistaken invite, a sign-up that
should not go on. Its link stops working at once. It cannot be undone, but the merchant
can be invited again.

A completed request is history and is refused (422 onboarding_completed): to stop that
merchant, 'rail0 admin accounts deactivate' the account, whose id is the request's.

A write: it needs write mode ('rail0 admin write-mode on').

```
rail0 admin onboardings cancel <request> [flags]
```

### Examples

```
  rail0 admin onboardings cancel 019f0c11-…
  rail0 admin onboardings cancel 019f0c11-… --yes
```

### Options

```
  -h, --help   help for cancel
  -y, --yes    Skip the confirmation prompt (required to run non-interactively)
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

