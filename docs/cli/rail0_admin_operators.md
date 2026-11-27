## rail0 admin operators

Who holds the operator role; grant it, revoke it (lists when run bare)

### Synopsis

List the operator grants, newest first: the account that holds each, who granted it
(empty for a grant made out of band, by the seeds or the gateway's own maintenance task),
the note it was given with, and when it expires.

Expired grants are listed too, as history. A grant opens the operator surface while its
expiry is empty or still ahead and its account is active.

```
rail0 admin operators [flags]
```

### Examples

```
  rail0 admin operators
  rail0 admin operators --pretty
  rail0 admin operators grant 019e748b-… --note "on call this week" --expires 7d
  rail0 admin operators revoke 019e748b-…
```

### Options

```
  -h, --help   help for operators
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

* [rail0 admin](rail0_admin.md)	 - Operator commands: diagnostics, transactions, sync errors, webhooks, disputes, accounts, wallets, onboarding, operators, audit, write mode
* [rail0 admin operators grant](rail0_admin_operators_grant.md)	 - Give an account the operator role (write mode, audited)
* [rail0 admin operators revoke](rail0_admin_operators_revoke.md)	 - Remove an account's operator role (write mode, audited)

