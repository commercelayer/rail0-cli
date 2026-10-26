## rail0 wallets update

Update label or active status on the session's wallet (by id, 0x address or @alias)

### Synopsis

Change a wallet's label, or turn it on and off with --active.

--active false is the merchant's REVOCATION switch, and it does more than hide a
row: a deactivated wallet stops being offered by 'payment-methods', so no new
payment can name it, and webhooks stop being dispatched for payments to it. That
is deliberate — deactivation says "stop trusting this address", and an outbound
push to a revoked wallet's payments would contradict it.

Existing payments are unaffected: they carry their own payee address and settle
whatever the flag says now. Reversible: switching it back on (--active true) is
the one write a deactivated wallet's own session may make, when you switched it
off yourself. A wallet an operator switched off is the operator's to switch back
on (403 wallet_deactivated_by_operator); 'wallets get' shows deactivated_by.
Another wallet's session cannot change it (403 not_your_wallet).

Because the key's own session can switch an owner's switch-off back on, a
definitive revocation (a leaked key) must come from an operator.

The account's ONLY active wallet cannot be switched off (422 last_active_wallet):
login resolves the account from its wallet, so an account with none could neither
act nor add one back. Another wallet must be active first.

Only the wallet you signed in with: another wallet of the account is refused
(403 not_your_wallet). 'rail0 auth login' with that wallet's key to change it.

'wallets delete' is the same operation under the API's DELETE verb — a soft
delete, not a removal — so the two are interchangeable.

```
rail0 wallets update <wallet> [flags]
```

### Examples

```
  rail0 wallets update 0xABCD… --label "New label" --active true
```

### Options

```
      --active string   Set active flag ('true' or 'false')
  -h, --help            help for update
  -l, --label string    New wallet label
  -y, --yes             Skip the confirmation prompt (required to run non-interactively)
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

* [rail0 wallets](rail0_wallets.md)	 - Manage the logged-in account's wallets (lists when run bare)

