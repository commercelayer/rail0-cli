## rail0 account

Show or update the signed-in account's profile (shows when run bare)

### Synopsis

Read and update the profile of the signed-in account: its id, name, email,
priority-tip setting and timestamps.

Requires a login ('rail0 auth login') by a wallet that belongs to an account — a
wallet-only session has no account to show. The account is the session's own
('rail0 auth status' prints its id); --account reads or updates another one, which
the gateway allows only for an admin (anyone else gets 403 not_your_account).

Run bare, or as 'account get', to show it; 'account update' changes the name,
email and/or priority tip. The account's active flag is operator-only and is not
settable here.

```
rail0 account [flags]
```

### Examples

```
  rail0 account                          # the signed-in account
  rail0 account get --account 019e748b-… # another account (admin only)
  rail0 account update --email ops@example.com
  rail0 account update --priority-tip=false
```

### Options

```
      --account string   Account id (default: the logged-in session account; another account's needs an admin session)
  -h, --help             help for account
```

### Options inherited from parent commands

```
      --base-url string   RAIL0 gateway base URL (env: RAIL0_BASE_URL)
      --debug             Print HTTP request/response details to stderr
      --json              Force plain (uncolored) JSON output (useful in scripts)
      --pretty            Use the key-value/table view instead of JSON (terminal only)
  -q, --quiet             Print only the result's id/hash, one per line (useful in scripts)
```

### SEE ALSO

* [rail0](rail0.md)	 - CLI for the RAIL0 stablecoin payment protocol
* [rail0 account get](rail0_account_get.md)	 - Show the account's profile (the same as 'rail0 account')
* [rail0 account update](rail0_account_update.md)	 - Change the account's name, email and/or priority tip

