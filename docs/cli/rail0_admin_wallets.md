## rail0 admin wallets

Any account's wallets: list them, see what names one, switch it off or on (lists when run bare)

### Synopsis

List the wallets of an account (--account), each with the tokens it accepts and, when
it is off, who switched it off (deactivated_by: owner or operator).

The account is always named with --account: 'rail0 admin accounts --search 0x…' finds
the account a wallet address belongs to.

```
rail0 admin wallets [flags]
```

### Examples

```
  rail0 admin wallets --account 019e748b-…
  rail0 admin wallets --account 019e748b-… --active false --pretty
  rail0 admin wallets usage 0x2D2B… --account 019e748b-…
  rail0 admin wallets deactivate 0x2D2B… --account 019e748b-…
```

### Options

```
      --account string   The account the wallets belong to (required)
      --active string    Only wallets that are on ('true') or off ('false')
  -h, --help             help for wallets
      --page int         Page number (1-based)
      --per-page int     Wallets per page
      --sort string      Sort wallets (e.g. -created_at)
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
* [rail0 admin wallets activate](rail0_admin_wallets_activate.md)	 - Switch a wallet back on, on any account (write mode, audited)
* [rail0 admin wallets deactivate](rail0_admin_wallets_deactivate.md)	 - Switch a wallet off, on any account (write mode, audited)
* [rail0 admin wallets usage](rail0_admin_wallets_usage.md)	 - What still names a wallet's address: payments as payee and payer, transactions sent

