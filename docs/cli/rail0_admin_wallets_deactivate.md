## rail0 admin wallets deactivate

Switch a wallet off, on any account (write mode, audited)

### Synopsis

Switch a wallet off. It stops being offered to buyers, so no new payment can name it,
and webhooks stop being dispatched for its payments. Existing payments settle whatever
the switch says: they carry the address themselves.

This is the definitive revocation, for a leaked or lost key: a wallet an operator
switched off can be switched back on only by an operator ('rail0 admin wallets
activate'). Its own session cannot, where it could after switching itself off.

The command reads what still names the address first ('rail0 admin wallets usage') and
asks. The account's only active wallet is refused (422 last_active_wallet): the account
could no longer sign in.

A write: it needs write mode ('rail0 admin write-mode on').

```
rail0 admin wallets deactivate <wallet> [flags]
```

### Examples

```
  rail0 admin wallets deactivate 0x2D2B… --account 019e748b-…
  rail0 admin wallets deactivate 019f02aa-… --account 019e748b-… --yes
```

### Options

```
  -h, --help   help for deactivate
  -y, --yes    Skip the confirmation prompt (required to run non-interactively)
```

### Options inherited from parent commands

```
      --account string    The account the wallets belong to (required)
      --agent             Agent mode: one JSON envelope on stdout, JSON events on stderr, no prompts (env: RAIL0_AGENT=1)
      --base-url string   RAIL0 gateway base URL (env: RAIL0_BASE_URL)
      --debug             Print HTTP request/response details to stderr
      --json              Force plain (uncolored) JSON output (useful in scripts)
      --pretty            Use the key-value/table view instead of JSON (terminal only)
  -q, --quiet             Print only the result's id/hash, one per line (useful in scripts)
```

### SEE ALSO

* [rail0 admin wallets](rail0_admin_wallets.md)	 - Any account's wallets: list them, see what names one, switch it off or on (lists when run bare)

