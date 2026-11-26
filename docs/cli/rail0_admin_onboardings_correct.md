## rail0 admin onboardings correct

Correct a request's name, email or address and send a new link (write mode, audited)

### Synopsis

Replace the name, the email and/or the wallet address of a request that is not
completed, and send a new link to its email, saying what changed. The previous link stops
working, nothing goes to the address that was replaced, and an expired request is
renewed. At least one of --name / --email / --address, with a value that differs.

The same checks as an invite run on the result. A completed request is history (422
onboarding_completed): its account is corrected with 'rail0 account update --account'.

A write: it needs write mode ('rail0 admin write-mode on').

```
rail0 admin onboardings correct <request> [flags]
```

### Examples

```
  rail0 admin onboardings correct 019f0c11-… --email right@acme.example
  rail0 admin onboardings correct 019f0c11-… --address 0x9F3A… --name "Acme Store Ltd"
```

### Options

```
      --address string   The corrected wallet address (0x…40 hex, or @alias)
      --email string     The corrected contact email
  -h, --help             help for correct
      --name string      The corrected account name
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

