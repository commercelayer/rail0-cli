## rail0 admin operators grant

Give an account the operator role (write mode, audited)

### Synopsis

Give an account the operator role. From its next request that account reads every
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

A write: it needs write mode ('rail0 admin write-mode on').

```
rail0 admin operators grant <account> [flags]
```

### Examples

```
  rail0 admin operators grant 019e748b-… --note "on call this week" --expires 7d
  rail0 admin operators grant 019e748b-… --note "platform team"
  rail0 admin operators grant 019e748b-… --expires 2026-12-31T23:59:59Z --yes
```

### Options

```
      --expires string   Time-box the grant: a duration from now (7d, 12h) or an ISO-8601 time; default: no expiry
  -h, --help             help for grant
      --note string      Why the grant exists (kept with the grant and in the audit log)
  -y, --yes              Skip the confirmation prompt (required to run non-interactively)
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

