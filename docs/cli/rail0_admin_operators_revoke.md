## rail0 admin operators revoke

Remove an account's operator role (write mode, audited)

### Synopsis

Remove the operator role from an account: from its next request it is a merchant
account like any other, and a write-mode token it holds stops working with it. The
argument is the ACCOUNT's id, as 'rail0 admin operators' lists it. The grant is deleted;
the audit log keeps who granted it and who revoked it.

The gateway refuses two cases. Your own grant: stepping down is another operator's act,
so the role cannot be dropped by a slip, nor by a stolen session covering its tracks. And
the last active operator: someone must be able to grant the role again.

The command reads the account and asks, naming it.

A write: it needs write mode ('rail0 admin write-mode on').

```
rail0 admin operators revoke <account> [flags]
```

### Examples

```
  rail0 admin operators revoke 019e748b-…
  rail0 admin operators revoke 019e748b-… --yes
```

### Options

```
  -h, --help   help for revoke
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

* [rail0 admin operators](rail0_admin_operators.md)	 - Who holds the operator role; grant it, revoke it (lists when run bare)

