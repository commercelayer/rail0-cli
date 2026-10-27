## rail0 account update

Change the account's name, email, priority tip and/or payment windows

### Synopsis

Change the account's display name, contact email, priority-tip setting
and/or payment windows. At least one of --name / --email / --priority-tip /
--auth-ttl / --refund-ttl is required.

Name and email are unique across accounts: a value another account already holds
is refused with 409. A deactivated account, or a session signed in with a
deactivated wallet, cannot change them (403) — a revoked key must not be able to
redirect the contact email, which is where the merchant's operational
notifications go. An admin may update another account with --account; the
gateway audits an operator's change with the values it replaced.

--priority-tip is the fee setting of the account's OWN transactions (authorize,
capture, void, release, refund). On, the default, they tip the network's suggested
priority fee; '--priority-tip=false' uses the chain's minimum tip instead (0 on
most chains, 25 gwei on Polygon PoS/Amoy) — cheaper, slower to confirm under load.
The buyer's dispute and close-dispute never read it.

--auth-ttl and --refund-ttl set the deadlines the account's NEW
payments are created with: how long an authorization can be captured, and how
long a payment can be refunded or disputed. A duration in days or hours ('2d',
'36h'), or 'default' to go back to the gateway default. The bounds are on the
profile ('rail0 account', payment_windows); outside them, or with the refund
window less than a day past the authorization, the gateway answers 422
invalid_payment_windows. Existing payments keep their signed deadlines.

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
  rail0 account update --auth-ttl 2d --refund-ttl 60d
  rail0 account update --refund-ttl default # back to the gateway default
```

### Options

```
      --auth-ttl string     Capture window of new payments: days or hours (2d, 36h), or 'default'
      --email string        New contact email (unique across accounts)
  -h, --help                help for update
      --name string         New display name (unique across accounts)
      --priority-tip        Tip the network's suggested priority fee on the account's own transactions; --priority-tip=false uses the chain's minimum tip (default true)
      --refund-ttl string   Refund and dispute window of new payments: days or hours (60d), or 'default'
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

