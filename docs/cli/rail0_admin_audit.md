## rail0 admin audit

The operator audit log: every operator write, who did it and to whom

### Synopsis

List the operator audit log, newest first: one row per write an operator made, with
the operator (actor), the account it landed on (target), the resource, and the detail the
gateway recorded with it (the fields changed and the values they replaced, the exposure an
operator acknowledged, the filters of a clear, …).

What is recorded: accounts (update, activate, deactivate), onboarding requests (create,
update, send, delete), wallets (update, deactivate, reactivate), the operator role
(grant, revoke), a webhook circuit reset, a transaction redrive, a catalogue reload, a
sync-error clear, and write mode switched on and off. A merchant's own writes are not
operator actions and are not here.

--actor and --target take account ids ('rail0 admin accounts --search …' finds them).
--payment takes the payment's UUID, not its rail0_id: 'rail0 admin trace 0x…' shows a
payment's operator actions from either.

```
rail0 admin audit [flags]
```

### Examples

```
  rail0 admin audit                                   # the latest operator writes
  rail0 admin audit --target 019e748b-…               # everything done to one account
  rail0 admin audit --actor 019f8a3d-… --from 2026-11-26T00:00:00Z
  rail0 admin audit --action account.deactivate --pretty
  rail0 admin audit --resource-type wallet
```

### Options

```
      --action string          Only this kind of action, e.g. account.deactivate, wallet.deactivate, sync_errors.clear
      --actor string           Only actions by this operator (account id)
      --from string            Only actions at/after this ISO-8601 time
  -h, --help                   help for audit
      --page int               Page number (1-based)
      --payment string         Only actions whose detail names this payment (its UUID): a redrive
      --per-page int           Results per page
      --resource-type string   Only actions on this kind of resource, e.g. account, wallet, onboarding, webhook
      --sort string            Sort by created_at or action (prefix - for descending; default: newest first)
      --target string          Only actions that landed on this account (account id)
      --to string              Only actions at/before this ISO-8601 time
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

