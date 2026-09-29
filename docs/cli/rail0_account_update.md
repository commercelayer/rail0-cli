## rail0 account update

Change the account's name and/or email

### Synopsis

Change the account's display name and/or contact email. At least one of
--name / --email is required.

Both are unique across accounts: a value another account already holds is refused
with 409. A deactivated account, or a session signed in with a deactivated wallet,
cannot change them (403) — a revoked key must not be able to redirect the contact
email, which is where the merchant's operational notifications go.

No confirmation is asked: the change is not destructive and 'account update'
reverses it.

```
rail0 account update [flags]
```

### Examples

```
  rail0 account update --name "Acme Store"
  rail0 account update --name "Acme Store" --email ops@acme.example
```

### Options

```
      --email string   New contact email (unique across accounts)
  -h, --help           help for update
      --name string    New display name (unique across accounts)
```

### Options inherited from parent commands

```
      --account string    Account id (default: the logged-in session account; another account's needs an admin session)
      --base-url string   RAIL0 gateway base URL (env: RAIL0_BASE_URL)
      --debug             Print HTTP request/response details to stderr
      --json              Force plain (uncolored) JSON output (useful in scripts)
      --pretty            Use the key-value/table view instead of JSON (terminal only)
  -q, --quiet             Print only the result's id/hash, one per line (useful in scripts)
```

### SEE ALSO

* [rail0 account](rail0_account.md)	 - Show or update the signed-in account's profile (shows when run bare)

