## rail0 admin accounts

Every account: find one, read it, switch it off or on (lists when run bare)

### Synopsis

List every account, as the operator reads them: the whole record, with the operator's
switch (active, status) and whether the account holds the operator grant (admin).

--search finds an account from whatever is at hand: a part of its name or of its email,
a wallet address or its beginning (0x…), or its id.

```
rail0 admin accounts [flags]
```

### Examples

```
  rail0 admin accounts
  rail0 admin accounts --search acme --pretty
  rail0 admin accounts --search 0x2D2B…            # whose wallet is this
  rail0 admin accounts --status deactivated
  rail0 admin accounts --operator true             # who holds the operator grant
```

### Options

```
  -h, --help              help for accounts
      --operator string   Only accounts that hold the operator grant ('true') or that do not ('false')
      --page int          Page number (1-based)
      --per-page int      Results per page
      --search string     A part of the name or email, a wallet address or its beginning (0x…), or the account's id
      --sort string       Sort by name, email or created_at (prefix - for descending; default: name)
      --status string     Only accounts with this status: active or deactivated
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

* [rail0 admin](rail0_admin.md)	 - Operator commands: diagnostics, transactions, sync errors, webhooks, accounts, wallets, onboarding, audit, write mode
* [rail0 admin accounts activate](rail0_admin_accounts_activate.md)	 - Switch a deactivated account back on (write mode, audited)
* [rail0 admin accounts deactivate](rail0_admin_accounts_deactivate.md)	 - Switch an account off (write mode, audited)
* [rail0 admin accounts exposure](rail0_admin_accounts_exposure.md)	 - What an account still has in motion: what a deactivation would leave half-way
* [rail0 admin accounts get](rail0_admin_accounts_get.md)	 - One account's whole record: profile, status, operator grant

