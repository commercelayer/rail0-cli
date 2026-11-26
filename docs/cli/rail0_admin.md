## rail0 admin

Operator commands: diagnostics, transactions, sync errors, webhooks, disputes, accounts, wallets, onboarding, payouts, operators, audit, configuration, catalogue, write mode

### Synopsis

Commands for a gateway operator. They read and repair across every account, so they
need a session whose account holds the operator grant: 'rail0 auth login' with an
operator's key. Any other session is answered 403.

An operator session reads. To write (clear sync errors, switch an account or a wallet
off or on, invite a merchant, reset a webhook circuit, grant or revoke the operator role,
reload the chain catalogue, and the operator's writes elsewhere in the CLI, such as 'account update --account …')
switch write mode on with
'rail0 admin write-mode on': it lasts an hour, both switches are audited on the gateway,
and 'rail0 admin write-mode off' ends it early. 'rail0 admin audit' lists every operator
write.

Not available in agent mode: these commands are for a person at a terminal.

### Examples

```
  rail0 admin health                       # one verdict, then a check per thing that can be wrong
  rail0 admin health --rpc                 # every chain's RPC endpoints, probed from the gateway
  rail0 admin transactions --stuck any     # what is stuck, across every account
  rail0 admin trace 0xrail0Id…             # one payment: transactions, sync errors, deliveries, disputes
  rail0 admin sync-errors --reason not_found
  rail0 admin accounts --search 0x2D2B…    # whose wallet is this
  rail0 admin onboardings                  # invites and sign-ups still waiting
  rail0 admin webhooks --circuit open      # subscriptions that are receiving nothing
  rail0 admin audit --target 019e748b-…    # everything operators did to one account
```

### Options

```
  -h, --help   help for admin
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

* [rail0](rail0.md)	 - CLI for the RAIL0 stablecoin payment protocol
* [rail0 admin accounts](rail0_admin_accounts.md)	 - Every account: find one, read it, switch it off or on (lists when run bare)
* [rail0 admin audit](rail0_admin_audit.md)	 - The operator audit log: every operator write, who did it and to whom
* [rail0 admin catalog](rail0_admin_catalog.md)	 - The chain catalogue: what a reload would change, the reload, chains, tokens, contracts (shows the diff when run bare)
* [rail0 admin config](rail0_admin_config.md)	 - The configuration the gateway is running with: every window, limit and feature flag
* [rail0 admin deliveries](rail0_admin_deliveries.md)	 - Webhook deliveries across accounts: what a merchant was told, and whether it landed
* [rail0 admin disputes](rail0_admin_disputes.md)	 - Every dispute across accounts, open and closed
* [rail0 admin health](rail0_admin_health.md)	 - Operator diagnostics: one verdict, then a check per thing that can be wrong
* [rail0 admin onboardings](rail0_admin_onboardings.md)	 - Onboarding requests: list, invite, correct, resend, cancel (lists when run bare)
* [rail0 admin operators](rail0_admin_operators.md)	 - Who holds the operator role; grant it, revoke it (lists when run bare)
* [rail0 admin payouts](rail0_admin_payouts.md)	 - Every account's payouts, read-only (lists when run bare)
* [rail0 admin sync-errors](rail0_admin_sync-errors.md)	 - Why an indexer callback could not be applied (lists when run bare)
* [rail0 admin trace](rail0_admin_trace.md)	 - Everything about one payment: transactions, sync errors, deliveries, disputes, operator actions
* [rail0 admin transactions](rail0_admin_transactions.md)	 - Transactions across every account, or only the stuck ones
* [rail0 admin wallets](rail0_admin_wallets.md)	 - Any account's wallets: list them, see what names one, switch it off or on (lists when run bare)
* [rail0 admin webhooks](rail0_admin_webhooks.md)	 - Every account's webhook subscriptions, with circuit and last attempt (lists when run bare)
* [rail0 admin write-mode](rail0_admin_write-mode.md)	 - Operator write mode: see it, switch it on or off (shows it when run bare)

