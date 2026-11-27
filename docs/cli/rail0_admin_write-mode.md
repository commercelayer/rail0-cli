## rail0 admin write-mode

Operator write mode: see it, switch it on or off (shows it when run bare)

### Synopsis

An operator session reads. A write is refused 403 'write_mode_required' until write
mode is on: a second token for the same session, valid one hour by default and never
longer than the session. Switching it on asks for no signature; both switches are
audited on the gateway.

While it is on, every command this CLI runs against this gateway carries it, so the
operator's writes outside 'rail0 admin' work too ('account update --account …',
'payments redrive', a wallet switched off). It ends when the hour is up, with
'rail0 admin write-mode off', with 'rail0 auth logout' and with a new 'rail0 auth login'.

The token is stored beside the session's and is never printed.

```
rail0 admin write-mode [flags]
```

### Examples

```
  rail0 admin write-mode        # is it on, and until when
  rail0 admin write-mode on
  rail0 admin write-mode off
```

### Options

```
  -h, --help   help for write-mode
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

* [rail0 admin](rail0_admin.md)	 - Operator commands: diagnostics, transactions, sync errors, webhooks, disputes, accounts, wallets, onboarding, payouts, operators, audit, configuration, catalogue, write mode
* [rail0 admin write-mode off](rail0_admin_write-mode_off.md)	 - Switch write mode off: revoke its token and remove it
* [rail0 admin write-mode on](rail0_admin_write-mode_on.md)	 - Switch write mode on for an hour (audited)
* [rail0 admin write-mode status](rail0_admin_write-mode_status.md)	 - Whether write mode is on, and until when

