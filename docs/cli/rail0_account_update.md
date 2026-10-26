## rail0 account update

Change the account's name, email and/or priority tip

### Synopsis

Change the account's display name, contact email and/or priority-tip
setting. At least one of --name / --email / --priority-tip is required.

Name and email are unique across accounts: a value another account already holds
is refused with 409. A deactivated account, or a session signed in with a
deactivated wallet, cannot change them (403) — a revoked key must not be able to
redirect the contact email, which is where the merchant's operational
notifications go. The email is the owner's to change: an admin updating another
account with --account may change its name and priority tip, but --email answers
403 forbidden (before the account exists, it is corrected on its onboarding
request in the console).

--priority-tip is the fee setting of the account's OWN transactions (authorize,
capture, void, release, refund). On, the default, they tip the network's suggested
priority fee; '--priority-tip=false' uses the chain's minimum tip instead (0 on
most chains, 25 gwei on Polygon PoS/Amoy) — cheaper, slower to confirm under load.
The buyer's dispute and close-dispute never read it.

No confirmation is asked: the change is not destructive and 'account update'
reverses it.

```
rail0 account update [flags]
```

### Examples

```
  rail0 account update --name "Acme Store"
  rail0 account update --name "Acme Store" --email ops@acme.example
  rail0 account update --priority-tip=false   # minimum tip on the account's transactions
  rail0 account update --priority-tip         # back to the suggested priority fee
```

### Options

```
      --email string   New contact email (unique across accounts)
  -h, --help           help for update
      --name string    New display name (unique across accounts)
      --priority-tip   Tip the network's suggested priority fee on the account's own transactions; --priority-tip=false uses the chain's minimum tip (default true)
```

### Options inherited from parent commands

```
      --account string    Account id (default: the logged-in session account; another account's needs an admin session)
      --agent             Agent mode: one JSON envelope on stdout, JSON events on stderr, no prompts (env: RAIL0_AGENT=1)
      --base-url string   RAIL0 gateway base URL (env: RAIL0_BASE_URL)
      --debug             Print HTTP request/response details to stderr
      --json              Force plain (uncolored) JSON output (useful in scripts)
      --pretty            Use the key-value/table view instead of JSON (terminal only)
  -q, --quiet             Print only the result's id/hash, one per line (useful in scripts)
```

### SEE ALSO

* [rail0 account](rail0_account.md)	 - Show or update the signed-in account's profile (shows when run bare)

